package internal

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// Two moor instances sharing a history file should not silently lose each
// other's committed searches.
//
// Ref: https://github.com/walles/moor/issues/454
func TestAddEntrySurvivesConcurrentInstance(t *testing.T) {
	// Otherwise BootSearchHistory() will import the real user's less history
	t.Setenv("LESSHISTFILE", "/dev/null")

	historyFile := filepath.Join(t.TempDir(), "search_history")

	instanceA := BootSearchHistory(historyFile)
	instanceB := BootSearchHistory(historyFile)

	instanceA.addEntry("from-a")
	instanceB.addEntry("from-b")

	afterBoth, err := loadMoorSearchHistory(historyFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, afterBoth, []string{"from-a", "from-b"})
}
