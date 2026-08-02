package search

import (
	"testing"

	"gotest.tools/v3/assert"
)

// This is really a constant, don't change it!
var _TestString = "mamma"

func TestGetMatchRanges(t *testing.T) {
	matchRanges := For("m+").GetMatchRanges(_TestString, 0)
	assert.Equal(t, len(matchRanges.Matches), 2) // Two matches

	assert.DeepEqual(t, matchRanges.Matches[0][0], 0) // First match starts at 0
	assert.DeepEqual(t, matchRanges.Matches[0][1], 1) // And ends on 1 exclusive

	assert.DeepEqual(t, matchRanges.Matches[1][0], 2) // Second match starts at 2
	assert.DeepEqual(t, matchRanges.Matches[1][1], 4) // And ends on 4 exclusive
}

func TestGetMatchRangesNilPattern(t *testing.T) {
	matchRanges := For("").GetMatchRanges(_TestString, 0)
	assert.Assert(t, matchRanges == nil)
	assert.Assert(t, !matchRanges.InRange(0))
}

func TestInRange(t *testing.T) {
	// Should match the one in TestGetMatchRanges()
	matchRanges := For("m+").GetMatchRanges(_TestString, 0)

	assert.Assert(t, !matchRanges.InRange(-1)) // Before start
	assert.Assert(t, matchRanges.InRange(0))   // m
	assert.Assert(t, !matchRanges.InRange(1))  // a
	assert.Assert(t, matchRanges.InRange(2))   // m
	assert.Assert(t, matchRanges.InRange(3))   // m
	assert.Assert(t, !matchRanges.InRange(4))  // a
	assert.Assert(t, !matchRanges.InRange(5))  // After end
}

func TestUtf8(t *testing.T) {
	// This test verifies that the match ranges are by rune rather than by byte
	unicodes := "-ä-ä-"
	matchRanges := For("ä").GetMatchRanges(unicodes, 0)

	assert.Assert(t, !matchRanges.InRange(0)) // -
	assert.Assert(t, matchRanges.InRange(1))  // ä
	assert.Assert(t, !matchRanges.InRange(2)) // -
	assert.Assert(t, matchRanges.InRange(3))  // ä
	assert.Assert(t, !matchRanges.InRange(4)) // -
}

func TestNoMatch(t *testing.T) {
	// This test verifies that the match ranges are by rune rather than by byte
	unicodes := "gris"
	matchRanges := For("apa").GetMatchRanges(unicodes, 0)

	assert.Assert(t, !matchRanges.InRange(0))
	assert.Assert(t, !matchRanges.InRange(1))
	assert.Assert(t, !matchRanges.InRange(2))
	assert.Assert(t, !matchRanges.InRange(3))
	assert.Assert(t, !matchRanges.InRange(4))
}

func TestEndMatch(t *testing.T) {
	// This test verifies that the match ranges are by rune rather than by byte
	unicodes := "-ä"
	matchRanges := For("ä").GetMatchRanges(unicodes, 0)

	assert.Assert(t, !matchRanges.InRange(0)) // -
	assert.Assert(t, matchRanges.InRange(1))  // ä
	assert.Assert(t, !matchRanges.InRange(2)) // Past the end
}

func TestRealWorldBug(t *testing.T) {
	// Verify a real world bug found in v1.9.8

	testString := "anna"
	matchRanges := For("n").GetMatchRanges(testString, 0)
	assert.Equal(t, len(matchRanges.Matches), 2) // Two matches

	assert.DeepEqual(t, matchRanges.Matches[0][0], 1) // First match starts at 1
	assert.DeepEqual(t, matchRanges.Matches[0][1], 2) // And ends on 2 exclusive

	assert.DeepEqual(t, matchRanges.Matches[1][0], 2) // Second match starts at 2
	assert.DeepEqual(t, matchRanges.Matches[1][1], 3) // And ends on 3 exclusive
}

func TestMatchRanges_CaseSensitiveRegex(t *testing.T) {
	matchRanges := For("G.*S").GetMatchRanges("GRIIIS", 0)
	assert.Assert(t, len(matchRanges.Matches) > 0)

	matchRangesLower := For("G.*S").GetMatchRanges("griiis", 0)
	assert.Assert(t, matchRangesLower == nil || len(matchRangesLower.Matches) == 0)
}

func TestMatchRanges_CaseInsensitiveRegex(t *testing.T) {
	testString := "GRIIIS"
	matchRanges := For("g.*s").GetMatchRanges(testString, 0)
	assert.Assert(t, len(matchRanges.Matches) > 0)
}

func TestMatchRanges_CaseSensitiveSubstring(t *testing.T) {
	matchRanges := For(")G").GetMatchRanges(")G", 0)
	assert.Assert(t, len(matchRanges.Matches) == 1)

	matchRangesLower := For(")G").GetMatchRanges(")g", 0)
	assert.Assert(t, matchRangesLower == nil || len(matchRangesLower.Matches) == 0)
}

func TestMatchRanges_CaseInsensitiveSubstring(t *testing.T) {
	testString := ")G"
	matchRanges := For(")g").GetMatchRanges(testString, 0)
	assert.Assert(t, len(matchRanges.Matches) == 1)
}

func TestMatchRanges_EmptyPattern(t *testing.T) {
	testString := "anything"
	matchRanges := For("").GetMatchRanges(testString, 0)
	assert.Assert(t, matchRanges == nil)
}

func TestMatchRanges_MaxRuneCount(t *testing.T) {
	// The second "a" is on rune 4, well past the two runes asked for
	matchRanges := For("a").GetMatchRanges("a---a", 2)

	assert.DeepEqual(t, matchRanges.Matches, [][2]int{{0, 1}})
}

func TestMatchRanges_MaxRuneCountKeepsStraddlingMatch(t *testing.T) {
	// "bc" starts on rune 1, which is within the two runes asked for, so it is
	// reported even though it reaches past them
	matchRanges := For("bc").GetMatchRanges("abc", 2)

	assert.DeepEqual(t, matchRanges.Matches, [][2]int{{1, 3}})
}

func TestMatchRanges_MaxRuneCountCountsRunesNotBytes(t *testing.T) {
	// Two runes but four bytes before the "a", which is still within the three
	// runes asked for
	matchRanges := For("a").GetMatchRanges("ääa", 3)

	assert.DeepEqual(t, matchRanges.Matches, [][2]int{{2, 3}})
}

func TestMatchRanges_MaxRuneCountIgnoredByRegexp(t *testing.T) {
	// Regexp searches report every match regardless of the limit
	matchRanges := For("a+").GetMatchRanges("a---a", 2)

	assert.DeepEqual(t, matchRanges.Matches, [][2]int{{0, 1}, {4, 5}})
}

func TestMatchRanges_MaxRuneCountDoesNotMoveTheEndAnchor(t *testing.T) {
	// "$" is the end of the string, never the end of the first maxRuneCount
	// runes. There is no "c" at the end of "abcdef", so there is no match.
	matchRanges := For("c$").GetMatchRanges("abcdef", 1)

	assert.Assert(t, matchRanges.Empty())
}
