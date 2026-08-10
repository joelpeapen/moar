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
	"syscall"
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

// The size of the pseudo terminal tests run moor on. Tests that care how wide
// the screen is should say ptyCols rather than spelling out the number.
const (
	ptyRows = 24
	ptyCols = 80
)

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

// How to start moor. See startMoor().
type moorOptions struct {
	// Answer moor's terminal background color query the way a real terminal
	// would. Without this moor has to wait out its answer timeout, just like on
	// terminals not supporting the query. Both are real world cases, and moor's
	// startup timing differs a lot between them.
	answerBackgroundQuery bool

	// "NAME=value" entries added to moor's otherwise minimal environment.
	extraEnv []string

	// Piped into moor's stdin, which is then closed. Nil hands moor the pty as
	// its stdin instead, which is what makes stdinIsRedirected false.
	stdin *string

	// Moor's command line arguments.
	args []string
}

// Starts moor on an 80x24 pseudo terminal, and starts capturing everything it
// writes. See the moorOptions fields for the knobs.
//
// The process is killed when the test ends. To have moor exit by itself, either
// send it a quit key or give it a reason to quit on its own, then wait().
func startMoor(t *testing.T, options moorOptions) *ptySession {
	t.Helper()

	binary, err := moorBinary()
	assert.NilError(t, err)

	// Sizing the terminal before starting moor means moor can never observe any
	// other size than this one.
	master, slave, err := pty.Open()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = master.Close() })
	assert.NilError(t, pty.Setsize(master, &pty.Winsize{Rows: ptyRows, Cols: ptyCols}))

	// Moor gets its own copy when it starts, and the capture() goroutine below
	// needs moor to be the only one holding the slave open. Otherwise reads from
	// the master would block forever rather than reporting moor gone.
	defer func() { _ = slave.Close() }()

	cmd := exec.Command(binary, options.args...)

	// Explicit and minimal, so that neither the MOOR, MOAR, TERM and COLORTERM
	// settings nor the search history of whoever runs the tests can change what
	// moor does here.
	cmd.Env = append([]string{
		"TERM=xterm-256color",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
	}, options.extraEnv...)

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	var stdinWriter *os.File
	if options.stdin != nil {
		stdinReader, writer, err := os.Pipe()
		assert.NilError(t, err)
		defer func() { _ = stdinReader.Close() }()

		cmd.Stdin = stdinReader
		stdinWriter = writer
	}

	// Ctty is an fd number in the child, where stdout is always 1. Moor's
	// keypresses don't come from here, twin dups stdout for those, but SIGWINCH
	// does, and $EDITOR expects a controlling terminal to do job control on.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 1}

	assert.NilError(t, cmd.Start())

	session := &ptySession{
		master:                master,
		cmd:                   cmd,
		exited:                make(chan struct{}),
		answerBackgroundQuery: options.answerBackgroundQuery,
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
	})

	go session.capture()

	// After capture() is running, so that a payload larger than the pty buffer
	// can't wedge moor between a blocked write to us and a blocked read from us.
	if stdinWriter != nil {
		_, err = stdinWriter.WriteString(*options.stdin)
		assert.NilError(t, err)
		assert.NilError(t, stdinWriter.Close())
	}

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

	s.waitForCount(t, needle, 1)
}

// Blocks until moor has written needle at least count times, and fails the test
// if moor exits or times out before that.
//
// See waitFor() for what makes a good needle.
func (s *ptySession) waitForCount(t *testing.T, needle string, count int) {
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

		if strings.Count(s.captured(), needle) >= count {
			return
		}

		if moorIsGone {
			t.Fatalf("Moor exited after writing %s only %d of %d times, it wrote:\n%s",
				humanizeEscapes(needle), strings.Count(s.captured(), needle), count,
				humanizeEscapes(s.captured()))
		}

		if time.Now().After(deadline) {
			t.Fatalf("Timed out waiting for %s to show up %d times, moor wrote:\n%s",
				humanizeEscapes(needle), count, humanizeEscapes(s.captured()))
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

// The requested number of lines of text, for piping into moor.
func textLines(lineCount int) string {
	contents := strings.Builder{}
	for i := 1; i <= lineCount; i++ {
		fmt.Fprintf(&contents, "hello world %d\n", i)
	}

	return contents.String()
}

// Creates a file with the requested number of lines, and returns its path. The
// contents are the same as textLines() would give you, so that a test can page
// the same text from a file and from a pipe.
func createTextFile(t *testing.T, lineCount int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), fmt.Sprintf("hello-%d.txt", lineCount))
	assert.NilError(t, os.WriteFile(path, []byte(textLines(lineCount)), 0o600))

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
