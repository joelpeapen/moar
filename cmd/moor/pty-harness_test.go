//go:build !windows

// Test harness for running moor on a pseudo terminal. Moor refuses to page
// unless stdout is a terminal, so this is the only way of getting at the escape
// sequences it writes.

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"gotest.tools/v3/assert"
)

// Switching to and from the alternate screen.
const (
	altScreenEnter = "\x1b[?1049h"
	altScreenLeave = "\x1b[?1049l"
)

// Moor asks the terminal for its background color on startup. A real terminal
// answers; the color below is black, but which color it is doesn't matter here.
const (
	backgroundQuery  = "\x1b]11;?"
	backgroundAnswer = "\x1b]11;rgb:0000/0000/0000\x07"
)

// How long tests wait for moor to write something, or to exit.
const ptyTimeout = 5 * time.Second

// Where moorBinary() puts the binary it builds. Removed by TestMain().
var moorBinaryDir string

func TestMain(m *testing.M) {
	var err error
	moorBinaryDir, err = os.MkdirTemp("", "moor-pty-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	exitCode := m.Run()

	_ = os.RemoveAll(moorBinaryDir)
	os.Exit(exitCode)
}

// Path to the moor binary under test, building it on first call. Tests that
// never touch a pty don't pay for the build.
//
// Deliberately not built with -race: a race report from moor would land in the
// pty stream in the middle of the escape sequences tests assert on. The rest of
// the test suite covers moor under the race detector.
var moorBinary = sync.OnceValues(func() (string, error) {
	binary := filepath.Join(moorBinaryDir, "moor")

	// -trimpath is what build.sh uses, and it changes the build ID of every
	// package, so without it we'd compile all of moor's dependencies a second
	// time. test.sh runs build.sh before the tests, so with it this is just a
	// link. Skipping build.sh's -ldflags is fine, those only affect the link.
	//
	// Relies on go test running us with the package directory as our working
	// directory.
	build := exec.Command("go", "build", "-trimpath", "-o", binary, ".")
	problems := strings.Builder{}
	build.Stderr = &problems
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("building moor: %w: %s", err, problems.String())
	}

	return binary, nil
})

// A moor process running on a pseudo terminal, with everything it writes
// captured, escape sequences included.
type ptySession struct {
	master *os.File
	cmd    *exec.Cmd

	// Closed once moor is gone and all of its output has been captured
	exited chan struct{}

	// Protects output, which the capture() goroutine writes to
	lock   sync.Mutex
	output bytes.Buffer

	// After startMoor() returns, only the capture() goroutine touches this
	answerBackgroundQuery bool
}

// Starts moor on an 80x24 pseudo terminal with the given command line
// arguments, and starts capturing everything it writes.
//
// With answerBackgroundQuery set, the harness answers moor's terminal
// background color query the way a real terminal would. Without it moor has to
// wait out its answer timeout, just like on terminals not supporting the query.
// Both are real world cases, and moor's startup timing differs a lot between
// them.
//
// The process is killed when the test ends. To have moor exit by itself, either
// send it a quit key or give it a reason to quit on its own, then wait().
func startMoor(t *testing.T, answerBackgroundQuery bool, args ...string) *ptySession {
	t.Helper()

	binary, err := moorBinary()
	assert.NilError(t, err)

	cmd := exec.Command(binary, args...)

	// Explicit and minimal, so that neither the MOOR, MOAR, TERM and COLORTERM
	// settings nor the search history of whoever runs the tests can change what
	// moor does here.
	cmd.Env = []string{
		"TERM=xterm-256color",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
	}

	// Sizing the terminal before starting moor means moor can never observe any
	// other size than this one.
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	assert.NilError(t, err)

	session := &ptySession{
		master:                master,
		cmd:                   cmd,
		exited:                make(chan struct{}),
		answerBackgroundQuery: answerBackgroundQuery,
	}

	t.Cleanup(func() {
		// Kill first: on macOS the pty master is a non-pollable file
		// descriptor, so closing it does not interrupt the capture() goroutine's
		// pending read. Moor going away does.
		_ = cmd.Process.Kill()
		select {
		case <-session.exited:
		case <-time.After(ptyTimeout):
		}

		// Reap moor, in case this test never called wait(). Harmless if it did.
		_ = cmd.Wait()

		_ = master.Close()
	})

	go session.capture()

	return session
}

// Reads moor's output until moor is gone, answering the terminal background
// color query on the way if we're supposed to.
func (s *ptySession) capture() {
	defer close(s.exited)

	buffer := make([]byte, 4096)
	for {
		// Moor being gone shows up as a read error: EOF on macOS, EIO on Linux.
		// There can be output to take care of even when that happens.
		count, err := s.master.Read(buffer)

		if count > 0 {
			s.lock.Lock()
			s.output.Write(buffer[:count])
			sawQuery := bytes.Contains(s.output.Bytes(), []byte(backgroundQuery))
			s.lock.Unlock()

			if s.answerBackgroundQuery && sawQuery {
				s.answerBackgroundQuery = false
				_, _ = s.master.Write([]byte(backgroundAnswer))
			}
		}

		if err != nil {
			return
		}
	}
}

// Everything moor has written so far, escape sequences included.
func (s *ptySession) captured() string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.output.String()
}

// Blocks until moor has written needle, and fails the test if moor exits or
// times out without doing so.
//
// Wait for painted contents rather than for a mode setting escape sequence.
// Mode settings can happen before moor has drawn anything, and then keys sent
// on seeing one get handled before there is anything on screen to act on.
func (s *ptySession) waitFor(t *testing.T, needle string) {
	t.Helper()

	deadline := time.Now().Add(ptyTimeout)
	for {
		// Check this before looking at the output, so that the last look
		// happens after the last of that output was captured
		moorIsGone := false
		select {
		case <-s.exited:
			moorIsGone = true
		default:
		}

		if strings.Contains(s.captured(), needle) {
			return
		}

		if moorIsGone {
			t.Fatalf("Moor exited without writing %s, it wrote:\n%s",
				humanizeEscapes(needle), humanizeEscapes(s.captured()))
		}

		if time.Now().After(deadline) {
			t.Fatalf("Timed out waiting for %s, moor wrote:\n%s",
				humanizeEscapes(needle), humanizeEscapes(s.captured()))
		}

		time.Sleep(time.Millisecond)
	}
}

// Sends keypresses to moor.
func (s *ptySession) send(t *testing.T, keys string) {
	t.Helper()

	_, err := s.master.Write([]byte(keys))
	assert.NilError(t, err)
}

// Blocks until moor has exited successfully and all of its output has been
// captured, and fails the test if it doesn't.
func (s *ptySession) wait(t *testing.T) {
	t.Helper()

	select {
	case <-s.exited:
	case <-time.After(ptyTimeout):
		t.Fatalf("Timed out waiting for moor to exit, it wrote:\n%s", humanizeEscapes(s.captured()))
	}

	assert.NilError(t, s.cmd.Wait(), "moor wrote:\n%s", humanizeEscapes(s.captured()))
}

// Creates a file with the requested number of lines, and returns its path.
func createTextFile(t *testing.T, lineCount int) string {
	t.Helper()

	contents := strings.Builder{}
	for i := 1; i <= lineCount; i++ {
		fmt.Fprintf(&contents, "hello world %d\n", i)
	}

	path := filepath.Join(t.TempDir(), fmt.Sprintf("hello-%d.txt", lineCount))
	assert.NilError(t, os.WriteFile(path, []byte(contents.String()), 0o600))

	return path
}

// Makes moor's output readable in test failure messages. Newlines are kept as
// newlines, ESC and other control characters are spelled out.
func humanizeEscapes(s string) string {
	humanized := strings.Builder{}
	for _, char := range s {
		if char == '\x1b' {
			humanized.WriteString("ESC")
			continue
		}

		if char == '\n' || (char >= ' ' && char != 0x7f) {
			humanized.WriteRune(char)
			continue
		}

		fmt.Fprintf(&humanized, "<0x%02x>", char)
	}

	return humanized.String()
}
