//go:build !windows

package main

import (
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
	assert.Assert(t, strings.Contains(captured, altScreenEnter),
		"Never entered the alternate screen:\n%s", humanizeEscapes(captured))
	assert.Assert(t, strings.Contains(captured, altScreenLeave),
		"Never left the alternate screen:\n%s", humanizeEscapes(captured))
}
