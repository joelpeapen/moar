package internal

import (
	"testing"

	"github.com/walles/moor/v2/internal/textstyles"
)

// isolateStyles gives this test the default styles, and restores them when the
// test is done.
//
// Styling lives in package level variables, so without this a test that styles
// the UI changes what every later test in the package sees. Call it from any
// test that does styling, either directly or by starting a pager.
func isolateStyles(t *testing.T) {
	reset := func() {
		theme = defaultUiStyles()
		textstyles.ResetManPageStyles()
	}

	reset()
	t.Cleanup(reset)
}
