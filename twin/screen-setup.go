//go:build !windows

package twin

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Re-apply the raw mode set by setupTtyInTtyOut(), in case some other process
// has reset the terminal mode behind our back.
//
// Node.js does that: it snapshots the terminal mode on startup and restores it
// on exit. In a "node ... | moor" pipeline, node usually snapshots cooked mode
// before we go raw, and then puts the terminal back into cooked mode when it
// exits.
//
// Ref: https://github.com/walles/moor/issues/443
func reassertTtyInMode(ttyIn *os.File) {
	fd := int(ttyIn.Fd())

	before, err := term.GetState(fd)
	if err != nil {
		// Not a terminal, nothing to re-assert
		return
	}

	// MakeRaw() reads the current mode and adjusts only the raw mode related
	// flags, so unrelated settings are preserved and repeated calls are
	// harmless.
	_, err = term.MakeRaw(fd)
	if err != nil {
		log.Debug(fmt.Sprint("Re-asserting raw terminal mode: ", err))
		return
	}

	after, err := term.GetState(fd)
	if err != nil {
		log.Debug(fmt.Sprint("Re-asserting raw terminal mode, reading state back: ", err))
		return
	}

	if !reflect.DeepEqual(before, after) {
		log.Info("Terminal mode was reset behind our back, raw mode re-asserted")
	}
}

func reassertTtyOutMode(_ *os.File) {
	// This block intentionally left blank.
	//
	// Escape sequence interpretation is done by the terminal emulator, not by
	// the kernel's termios layer, so our writes work no matter what state the
	// termios is in.
	//
	// In cooked mode our "\r\n" line endings render as "\r\r\n" (ONLCR), but
	// the extra "\r" is invisible on screen. So there is nothing here worth
	// fixing.
	//
	// Re-asserting the termios state here would even be harmful: unlike the
	// reads, writes are not synchronized with the intentional cooked mode
	// restores done by PauseAndCall() and Close(), so we could race with
	// those and re-raw the terminal right after they restored it.
	//
	// On Windows, input and output console modes are separate, and escape
	// sequence interpretation does depend on the output mode, see the
	// implementation in twin/screen-setup-windows.go.
}

func (r *interruptableReader) waitForReadReady(timeout time.Duration) (ready bool, err error) {
	// "This argument should be set to the highest-numbered file descriptor in
	// any of the three sets, plus 1. The indicated file descriptors in each set
	// are checked, up to this limit"
	//
	// Ref: https://man7.org/linux/man-pages/man2/select.2.html
	nfds := r.base.Fd()
	readFds := unix.FdSet{}
	readFds.Set(int(r.base.Fd()))
	selectTimeout := unix.NsecToTimeval(timeout.Nanoseconds())

	_, err = unix.Select(int(nfds)+1, &readFds, nil, nil, &selectTimeout)
	if err == syscall.EINTR {
		// Not really a problem, we can get this on window resizes for example
		return false, nil
	}
	if err != nil {
		// Select failed
		return
	}

	if readFds.IsSet(int(r.base.Fd())) {
		return true, nil
	}

	// Timeout: nothing to read right now.
	return false, nil
}

// Subscribe to SIGWINCH signals. Compared to polling, this will reduce power
// usage in the absence of window resizes.
func (screen *UnixScreen) setupSigwinchNotification() {
	screen.sigwinch = make(chan int, 1)
	screen.sigwinch <- 0 // Trigger initial screen size query

	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		defer func() {
			panicHandler("setupSigwinchNotification()/SIGWINCH", recover(), debug.Stack())
		}()

		for {
			// Await window resize signal
			<-sigwinch

			screen.onWindowResized()
		}
	}()
}

func (screen *UnixScreen) setupTtyInTtyOut() error {
	// Dup stdout so we can close stdin in Close() without closing stdout.
	// Before this dupping, we crashed on using --quit-if-one-screen.
	//
	// Ref:https://github.com/walles/moor/issues/214
	stdoutDupFd, err := syscall.Dup(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	stdoutDup := os.NewFile(uintptr(stdoutDupFd), "moor-stdout-dup")

	// os.Stdout is a stream that goes to our terminal window.
	//
	// So if we read from there, we'll get input from the terminal window.
	//
	// If we just read from os.Stdin that would fail when getting data piped
	// into ourselves from some other command.
	//
	// Tested on macOS and Linux, works like a charm!
	screen.ttyIn = stdoutDup // <- YES, WE SHOULD ASSIGN STDOUT TO TTYIN

	// Set input stream to raw mode
	screen.oldTerminalState, err = term.MakeRaw(int(screen.ttyIn.Fd()))
	if err != nil {
		return err
	}

	screen.ttyOut = os.Stdout

	ttyInTerminalState, err := term.GetState(int(screen.ttyIn.Fd()))
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("ttyin terminal state: %+v", ttyInTerminalState))

	ttyOutTerminalState, err := term.GetState(int(screen.ttyOut.Fd()))
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("ttyout terminal state: %+v", ttyOutTerminalState))

	return nil
}

func (screen *UnixScreen) restoreTtyInTtyOut() error {
	return term.Restore(int(screen.ttyIn.Fd()), screen.oldTerminalState)
}
