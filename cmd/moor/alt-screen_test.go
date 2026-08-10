//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// Paging a file that doesn't fit on one screen happens on the alternate screen,
// so that the user's terminal contents are restored on exit.
func TestLongFileUsesAlternateScreen(t *testing.T) {
	const answerBackgroundQuery = true
	session := startMoor(t, answerBackgroundQuery, createTextFile(t, 100))

	// The status bar, so we know a full screen has been painted
	session.waitFor(t, "100 lines")

	session.send(t, "q")
	session.wait(t)

	assertAltScreenPaired(t, session.captured())
}

// Launching an editor with "v" puts the user's terminal back the way it was for
// the editor, and takes it back again afterwards, without ever getting the
// alternate screen bookkeeping out of sync.
func TestEditorRoundTripKeepsAlternateScreenPaired(t *testing.T) {
	const answerBackgroundQuery = true
	session := startMoorWithEnv(t, answerBackgroundQuery,
		[]string{"EDITOR=" + createEditorStub(t)}, createTextFile(t, 100))

	// The status bar, so we know a full screen has been painted
	session.waitFor(t, "100 lines")

	session.send(t, "v")
	session.waitFor(t, editorStubMarker)

	// Moor puts the terminal back in raw mode before entering the alternate
	// screen again, so by the time we see that second entry our quit key will
	// be delivered as a single keypress rather than getting stuck in a line
	// buffer waiting for a newline.
	session.waitForCount(t, altScreenEnter, 2)

	session.send(t, "q")
	session.wait(t)

	captured := session.captured()
	assert.Equal(t, strings.Count(captured, altScreenEnter), 2,
		"Expected one alternate screen entry at startup and one after the editor:\n%s",
		humanizeEscapes(captured))
	assertAltScreenPaired(t, captured)
}

// Fails the test unless moor entered the alternate screen, left it exactly as
// many times as it entered it, and entered it before leaving it the first time.
//
// An unpaired ESC[?1049l is not harmless: DECRST 1049 also restores a saved
// cursor position, so it can teleport the user's cursor.
func assertAltScreenPaired(t *testing.T, captured string) {
	t.Helper()

	assert.Equal(t, strings.Count(captured, altScreenEnter), strings.Count(captured, altScreenLeave),
		"Unbalanced alternate screen switching:\n%s", humanizeEscapes(captured))

	assert.Assert(t, strings.Contains(captured, altScreenEnter),
		"Never entered the alternate screen:\n%s", humanizeEscapes(captured))

	assert.Assert(t, strings.Index(captured, altScreenEnter) < strings.Index(captured, altScreenLeave),
		"Left the alternate screen before entering it:\n%s", humanizeEscapes(captured))
}

// What createEditorStub()'s editor prints, and moor doesn't.
const editorStubMarker = "editor-stub-was-here"

// Creates a stand-in for $EDITOR that announces itself and exits immediately,
// and returns its path.
func createEditorStub(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "editor-stub")
	script := "#!/bin/sh\necho " + editorStubMarker + "\n"
	assert.NilError(t, os.WriteFile(path, []byte(script), 0o700))

	return path
}

// With --quit-if-one-screen, what moor leaves behind ends up on the user's
// normal screen, so that it looks like cat printed it.
func TestQuitIfOneScreenPrintsOnNormalScreen(t *testing.T) {
	for _, answerBackgroundQuery := range []bool{true, false} {
		t.Run(fmt.Sprintf("answerBackgroundQuery=%t", answerBackgroundQuery), func(t *testing.T) {
			// Two lines fit on our 24 line screen, so moor quits by itself
			session := startMoor(t, answerBackgroundQuery,
				"--quit-if-one-screen", createTextFile(t, 2))
			session.wait(t)

			captured := session.captured()
			printedAt := strings.LastIndex(captured, "hello world 2")
			assert.Assert(t, printedAt >= 0,
				"Never printed the file contents:\n%s", humanizeEscapes(captured))

			// Negative if moor never entered the alternate screen at all, which
			// puts the contents on the normal screen just as well
			leftAltScreenAt := strings.LastIndex(captured, altScreenLeave)
			assert.Assert(t, printedAt > leftAltScreenAt,
				"File contents printed on the alternate screen:\n%s", humanizeEscapes(captured))
		})
	}
}
