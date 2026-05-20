package reader

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestTryOpenDirectory(t *testing.T) {
	tempDir := t.TempDir()

	err := TryOpen(tempDir)
	assert.Assert(t, err != nil, "TryOpen should fail on directories")
}

func TestReadTextDone(t *testing.T) {
	testMe := NewFromTextForTesting("", "Johan")

	assert.NilError(t, testMe.Wait())
}
