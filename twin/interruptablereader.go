package twin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

type interruptableReader struct {
	base *os.File

	interrupted atomic.Bool

	pauseOrRead semaphore.Weighted

	// Re-apply the terminal mode we need. Replaced in tests, which have no
	// terminal to observe; production code always wants reassertTtyInMode.
	reassert func(ttyIn *os.File)
}

// Basically how long we wait between interrupt checks
const interruptableReaderMaxWait = 100 * time.Millisecond

func newInterruptableReader(base *os.File) interruptableReader {
	return interruptableReader{
		base: base,

		// Ensures we can either read or be paused, but not both at the same time
		pauseOrRead: *semaphore.NewWeighted(1),

		reassert: reassertTtyInMode,
	}
}

// Interrupt unblocks the read call, either now or eventually.
func (r *interruptableReader) Interrupt() {
	r.interrupted.Store(true)

	log.Info("Interruptable reader interrupted")
}

func (r *interruptableReader) SetPaused(paused bool) {
	if paused {
		err := r.pauseOrRead.Acquire(context.TODO(), 1)
		if err != nil {
			panic(fmt.Errorf("Failed to acquire interruptable reader pause semaphore for pausing: %w", err))
		}
	} else {
		r.pauseOrRead.Release(1)
	}
}

func (r *interruptableReader) Read(p []byte) (n int, err error) {
	for {
		if r.interrupted.Load() {
			log.Info("Interruptable reader already interrupted, returning fabricated EOF")
			return 0, io.EOF
		}

		// Other processes can (and do) reset the terminal mode behind our back.
		// We cannot detect that happening, so we just keep re-applying the mode
		// we need.
		//
		// This must happen even when there is nothing to read: in cooked mode
		// keystrokes are line buffered by the kernel, so they don't make our fd
		// readable until the user presses enter. Waiting with the re-assert
		// until we have something to read would be waiting for input that
		// cannot arrive.
		//
		// The semaphore makes sure we never re-assert while the terminal mode
		// has intentionally been restored, by PauseAndCall() or by Close().
		//
		// Refs:
		//   - https://github.com/walles/moor/issues/443
		//   - https://github.com/walles/moor/issues/394
		if r.pauseOrRead.TryAcquire(1) {
			r.reassert(r.base)
			r.pauseOrRead.Release(1)
		}

		// A reset while we're waiting here is fine: the wait is bounded, and
		// the next re-assert picks it up. Keystrokes made in the meantime stay
		// buffered by the kernel until then.
		ready, waitErr := r.waitForReadReady(interruptableReaderMaxWait)
		if waitErr != nil {
			return 0, waitErr
		}

		if !ready {
			continue
		}

		if r.interrupted.Load() {
			log.Info("Interruptable reader interrupted while waiting, returning fabricated EOF")
			return 0, io.EOF
		}

		err = r.pauseOrRead.Acquire(context.TODO(), 1)
		if err != nil {
			panic(fmt.Errorf("Failed to acquire interruptable reader pause semaphore for reading: %w", err))
		}

		// The acquire above can have waited out a whole pause, with every
		// re-assert at the top of the loop skipped throughout it. Re-assert
		// here, because the read below blocks while holding the semaphore: if
		// the mode is wrong by then, the top of the loop won't get to fix it
		// either.
		r.reassert(r.base)

		n, err = r.base.Read(p)
		r.pauseOrRead.Release(1)

		if r.interrupted.Load() {
			log.Info("Interruptable reader interrupted while reading, returning fabricated EOF")
			return 0, io.EOF
		}

		if err == io.EOF {
			log.Info("Interruptable reader base returned a genuine EOF")
		}

		return
	}
}
