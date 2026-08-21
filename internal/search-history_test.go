package internal

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// Existing history entries must stay in place, in order, with a newly
// committed search appended after them.
func TestAddEntryAppendsAfterExistingHistory(t *testing.T) {
	// Otherwise BootSearchHistory() will import the real user's less history
	t.Setenv("LESSHISTFILE", "/dev/null")

	historyFile := filepath.Join(t.TempDir(), "search_history")
	err := os.WriteFile(historyFile, []byte("before\n"), 0o600)
	assert.NilError(t, err)

	instance := BootSearchHistory(historyFile)
	instance.addEntry("after")

	afterAdd, err := loadMoorSearchHistory(historyFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, afterAdd, []string{"before", "after"})
}

// Committing a search that's already in the history, but isn't the most
// recent entry, moves it to the end as the newest entry rather than
// leaving a duplicate behind at its old position.
func TestAddEntryMovesExistingEntryToEnd(t *testing.T) {
	// Otherwise BootSearchHistory() will import the real user's less history
	t.Setenv("LESSHISTFILE", "/dev/null")

	historyFile := filepath.Join(t.TempDir(), "search_history")
	err := os.WriteFile(historyFile, []byte("one\ntwo\n"), 0o600)
	assert.NilError(t, err)

	instance := BootSearchHistory(historyFile)
	instance.addEntry("one")

	afterAdd, err := loadMoorSearchHistory(historyFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, afterAdd, []string{"two", "one"})
}

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

// A history file that already has entries when two moor instances boot
// should end up with all four entries afterwards: the pre-existing ones,
// plus each instance's newly committed search.
//
// Ref: https://github.com/walles/moor/issues/454
func TestAddEntrySurvivesConcurrentInstanceWithExistingHistory(t *testing.T) {
	// Otherwise BootSearchHistory() will import the real user's less history
	t.Setenv("LESSHISTFILE", "/dev/null")

	historyFile := filepath.Join(t.TempDir(), "search_history")
	err := os.WriteFile(historyFile, []byte("old1\nold2\n"), 0o600)
	assert.NilError(t, err)

	instanceA := BootSearchHistory(historyFile)
	instanceB := BootSearchHistory(historyFile)

	instanceA.addEntry("from-a")
	instanceB.addEntry("from-b")

	afterBoth, err := loadMoorSearchHistory(historyFile)
	assert.NilError(t, err)
	assert.DeepEqual(t, afterBoth, []string{"old1", "old2", "from-a", "from-b"})
}

// less stores both search patterns and shell commands in .lesshst, each
// under its own section header, with entries in both sections sharing the
// same leading-quote line format. Importing less history should only pick
// up the .search section, not shell commands from the .shell section.
func TestLoadLessSearchHistoryIgnoresShellSection(t *testing.T) {
	lessHistFile := filepath.Join(t.TempDir(), ".lesshst")
	err := os.WriteFile(lessHistFile, []byte(
		".less-history-file:\n"+
			".search\n"+
			"\"search-term\n"+
			".shell\n"+
			"\"shell-command\n",
	), 0o600)
	assert.NilError(t, err)
	t.Setenv("LESSHISTFILE", lessHistFile)

	history, err := loadLessSearchHistory()
	assert.NilError(t, err)
	assert.DeepEqual(t, history, []string{"search-term"})
}
