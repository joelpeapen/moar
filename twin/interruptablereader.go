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
}

// Basically how long we wait between interrupt checks
const interruptableReaderMaxWait = 100 * time.Millisecond

func newInterruptableReader(base *os.File) interruptableReader {
	return interruptableReader{
		base: base,

		// Ensures we can either read or be paused, but not both at the same time
		pauseOrRead: *semaphore.NewWeighted(1),
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
