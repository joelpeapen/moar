package textstyles

import "unicode/utf8"

// Lazily iterate a string by runes. Init by setting str only.
//
// Use getRelative(0) to get the current rune, and do next() to advance to the
// next rune.
type lazyRunes struct {
	str string

	// From what position in the string should we pick the next rune?
	nextByteIndex int

	lookaheadBuffer []rune
}

// Get rune at index runeIndex relative to the current rune
func (l *lazyRunes) getRelative(runeIndex int) *rune {
	for runeIndex >= len(l.lookaheadBuffer) {
		// Need to add one more rune to the lookahead buffer

		if l.nextByteIndex >= len(l.str) {
			// No more runes
			return nil
		}

		newRune, runeSize := utf8.DecodeRuneInString(l.str[l.nextByteIndex:])
		if runeSize == 0 {
			panic("We just checked, there should be more runes")
		}

		// Append the new rune to the lookahead buffer
		l.lookaheadBuffer = append(l.lookaheadBuffer, newRune)
		l.nextByteIndex += runeSize
	}

	return &l.lookaheadBuffer[runeIndex]
}

// Move the base rune index forward by one rune.
func (l *lazyRunes) next() {
	if l.getRelative(0) == nil {
		// Already past the end
		return
	}

	// Shift the lookahead buffer down
	if len(l.lookaheadBuffer) > 0 {
		l.lookaheadBuffer = l.lookaheadBuffer[1:]
	}
}
