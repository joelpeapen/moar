package textstyles

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestLazyRunes_empty(t *testing.T) {
	testMe := lazyRunes{str: ""}
	assert.Equal(t, true, testMe.getRelative(0) == nil)
}

func TestLazyRunes_unicode(t *testing.T) {
	testMe := lazyRunes{str: "åäö"}

	// What's up first?
	assert.Equal(t, 'å', *testMe.getRelative(0))
	assert.Equal(t, 'ä', *testMe.getRelative(1))
	// Intentionally don't get the third rune yet

	// Move to 'ä'
	testMe.next()
	assert.Equal(t, 'ä', *testMe.getRelative(0))

	// Move to 'ö'
	testMe.next()
	assert.Equal(t, 'ö', *testMe.getRelative(0))

	// No more runes
	assert.Equal(t, true, testMe.getRelative(1) == nil)

	// Move past the end
	testMe.next()
	assert.Equal(t, true, testMe.getRelative(0) == nil)
}
