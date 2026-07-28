package reader

import (
	"testing"

	"gotest.tools/v3/assert"
)

// Lines are created from slices of a larger read buffer. Growing one Line must
// never write into the bytes following it in that buffer, since those belong to
// other lines.
func TestCreatedLineCannotGrowIntoCallersBuffer(t *testing.T) {
	buffer := []byte("first\nsecond\n")

	pool := linePool{}
	line := pool.create(buffer[0:5]) // "first"

	line.raw = append(line.raw, []byte("XXXXX")...)

	assert.Equal(t, string(buffer), "first\nsecond\n")
	assert.Equal(t, string(line.raw), "firstXXXXX")
}
