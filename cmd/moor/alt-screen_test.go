//go:build !windows

package main

import (
	"fmt"
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

	captured := session.captured()
	enteredAt := strings.Index(captured, altScreenEnter)
	assert.Assert(t, enteredAt >= 0,
		"Never entered the alternate screen:\n%s", humanizeEscapes(captured))

	leftAt := strings.LastIndex(captured, altScreenLeave)
	assert.Assert(t, leftAt > enteredAt,
		"Never left the alternate screen after entering it:\n%s", humanizeEscapes(captured))
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
