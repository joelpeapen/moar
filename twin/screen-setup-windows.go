//go:build windows

package twin

import (
	"fmt"
	"os"
	"runtime/debug"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// Console mode flags we need for paging to work. Applied in setupTtyInTtyOut()
// and re-applied by reassertTtyInMode() / reassertTtyOutMode().
//
// Set and clear flags rather than one absolute mode, because flags not listed
// here must be left alone. Some are user preferences (ENABLE_QUICK_EDIT_MODE),
// and the console flips ENABLE_MOUSE_INPUT by itself when we enable mouse
// tracking, so writing an absolute mode would turn mouse reporting back off.
const (
	ttyInSetFlags = windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	// These match what term.MakeRaw() clears on Windows
	ttyInClearFlags = windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT

	ttyOutSetFlags = windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
)

var peekNamedPipe = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekNamedPipe")

func waitForPipeReadReady(handle windows.Handle) (ready bool, err error) {
	var bytesAvailable uint32
	result, _, callErr := peekNamedPipe.Call(
		uintptr(handle),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&bytesAvailable)),
		0,
	)
	if result != 0 {
		return bytesAvailable > 0, nil
	}

	if callErr == windows.ERROR_BROKEN_PIPE {
		// Writer closed: let a real Read() return EOF.
		return true, nil
	}

	if callErr == windows.ERROR_NO_DATA {
		// Pipe has no data right now.
		return false, nil
	}

	if callErr == windows.ERROR_HANDLE_EOF {
		return true, nil
	}

	return false, fmt.Errorf("PeekNamedPipe failed: %w", callErr)
}

// cmd.exe resets the console modes behind our back, both while running batch
// files and when the process piping into moor terminates. We cannot detect
// when that happens, so instead we re-apply the flags we need before every
// tty read and write, leaving all other flags untouched.
//
// less does the same thing.
//
// Ref: https://github.com/walles/moor/issues/394
func reassertConsoleMode(handle windows.Handle, setFlags uint32, clearFlags uint32) {
	var currentMode uint32
	err := windows.GetConsoleMode(handle, &currentMode)
	if err != nil {
		// Not a console, nothing to re-assert
		return
	}

	wantedMode := (currentMode | setFlags) &^ clearFlags
	if wantedMode == currentMode {
		return
	}

	err = windows.SetConsoleMode(handle, wantedMode)
	if err != nil {
		log.Debug(fmt.Sprintf("Failed to change console mode from %#x to %#x: %v", currentMode, wantedMode, err))
		return
	}

	log.Debug(fmt.Sprintf("Console mode changed behind our back, set it back from %#x to %#x", currentMode, wantedMode))
}

// Re-apply the ttyIn console mode set by setupTtyInTtyOut(), in case cmd.exe
// has reset it.
func reassertTtyInMode(ttyIn *os.File) {
	reassertConsoleMode(windows.Handle(ttyIn.Fd()), ttyInSetFlags, ttyInClearFlags)
}

// Re-apply the ttyOut console mode set by setupTtyInTtyOut(), in case cmd.exe
// has reset it.
func reassertTtyOutMode(ttyOut *os.File) {
	reassertConsoleMode(windows.Handle(ttyOut.Fd()), ttyOutSetFlags, 0)
}

func (r *interruptableReader) waitForReadReady(timeout time.Duration) (ready bool, err error) {
	fileType, err := windows.GetFileType(windows.Handle(r.base.Fd()))
	if err != nil {
		return false, err
	}

	if fileType == windows.FILE_TYPE_PIPE {
		ready, err = waitForPipeReadReady(windows.Handle(r.base.Fd()))
		if ready || err != nil {
			return
		}

		time.Sleep(timeout / 2)
		ready, err = waitForPipeReadReady(windows.Handle(r.base.Fd()))
		if ready || err != nil {
			return
		}

		time.Sleep(timeout / 2)
		return
	}

	// We're reading from the console
	reassertTtyInMode(r.base)

	timeoutMillis := uint32(timeout.Milliseconds())
	if timeoutMillis == 0 {
		timeoutMillis = 1
	}

	waitResult, err := windows.WaitForSingleObject(windows.Handle(r.base.Fd()), timeoutMillis)
	if err != nil {
		return false, err
	}

	if waitResult == uint32(windows.WAIT_OBJECT_0) {
		return true, nil
	}

	if waitResult == uint32(windows.WAIT_TIMEOUT) {
		return false, nil
	}

	return false, fmt.Errorf("unexpected WaitForSingleObject result: %d", waitResult)
}

// Poll for terminal size changes. No SIGWINCH on Windows, this is apparently
// the way.
func (screen *UnixScreen) setupSigwinchNotification() {
	screen.sigwinch = make(chan int, 1)
	screen.sigwinch <- 0 // Trigger initial screen size query

	go func() {
		defer func() {
			panicHandler("setupSigwinchNotification()", recover(), debug.Stack())
		}()

		var lastWidth, lastHeight int
		for {
			time.Sleep(100 * time.Millisecond)

			width, height, err := term.GetSize(int(screen.ttyOut.Fd()))
			if err != nil {
				log.Debug(fmt.Sprint("Failed to get terminal size: ", err))
				continue
			}

			if width == lastWidth && height == lastHeight {
				// No change, skip notification
				continue
			}

			lastWidth, lastHeight = width, height

			screen.onWindowResized()
		}
	}()
}

func (screen *UnixScreen) setupTtyInTtyOut() error {
	in, err := syscall.Open("CONIN$", syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open CONIN$: %w", err)
	}

	screen.ttyIn = os.NewFile(uintptr(in), "/dev/tty")

	// Set input stream to raw mode
	stdin := windows.Handle(screen.ttyIn.Fd())
	err = windows.GetConsoleMode(stdin, &screen.oldTtyInMode)
	if err != nil {
		return fmt.Errorf("failed to get stdin console mode: %w", err)
	}
	err = windows.SetConsoleMode(stdin, (screen.oldTtyInMode|ttyInSetFlags)&^ttyInClearFlags)
	if err != nil {
		return fmt.Errorf("failed to set stdin console mode: %w", err)
	}

	screen.ttyOut = os.Stdout

	// Enable console colors, from: https://stackoverflow.com/a/52579002
	stdout := windows.Handle(screen.ttyOut.Fd())
	err = windows.GetConsoleMode(stdout, &screen.oldTtyOutMode)
	if err != nil {
		screen.restoreTtyInTtyOut() // Error intentionally ignored, report the first one only
		return fmt.Errorf("failed to get stdout console mode: %w", err)
	}
	err = windows.SetConsoleMode(stdout, screen.oldTtyOutMode|ttyOutSetFlags)
	if err != nil {
		screen.restoreTtyInTtyOut() // Error intentionally ignored, report the first one only
		return fmt.Errorf("failed to set stdout console mode: %w", err)
	}

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
	errors := []error{}

	stdin := windows.Handle(screen.ttyIn.Fd())
	err := windows.SetConsoleMode(stdin, screen.oldTtyInMode)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to restore stdin console mode: %w", err))
	}

	stdout := windows.Handle(screen.ttyOut.Fd())
	err = windows.SetConsoleMode(stdout, screen.oldTtyOutMode)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to restore stdout console mode: %w", err))
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("failed to restore terminal state: %v", errors)
}
