//go:build !windows

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// What moor leaves on the user's own screen on the way out must not change the
// style of whatever the shell prints next.
func TestReprintEndsWithStyleReset(t *testing.T) {
	// A red line exactly as wide as the screen. Moor resets the style as part of
	// clearing to end of line, which a line this wide doesn't get, so this is
	// what a missing reset shows up on. The input deliberately ends without a
	// reset of its own, so that finding one in the output means moor wrote it.
	wideLine := strings.Repeat("x", ptyCols)
	contents := "\x1b[41m" + wideLine + "\n"

	session := startMoor(t, moorOptions{
		answerBackgroundQuery: true,
		stdin:                 &contents,
		args:                  []string{"--quit-if-one-screen"},
	})
	session.wait(t)

	captured := session.captured()
	assert.Assert(t, strings.Contains(captured, wideLine),
		"Never printed the contents:\n%s", humanizeEscapes(captured))

	assert.Assert(t, strings.HasSuffix(strings.TrimRight(captured, "\r\n"), "\x1b[m"),
		"Terminal style left modified:\n%s", humanizeEscapes(captured))
}
