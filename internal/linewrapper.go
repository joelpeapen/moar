package internal

import (
	"unicode"

	"github.com/walles/moor/v2/internal/textstyles"
)

// From: https://www.compart.com/en/unicode/U+00A0
//
//revive:disable-next-line:var-naming
const NO_BREAK_SPACE = '\xa0'

var minWrapWidth = 10

// Given some text and a maximum width in screen cells, find the best point at
// which to wrap the text. Return value is in number of runes.
func getWrapCount(line []textstyles.CellWithMetadata, maxScreenCellsCount int) int {
	screenCells := 0
	bestCutPoint := maxScreenCellsCount
	inLeadingWhitespace := true
	for cutBeforeThisIndex := 0; cutBeforeThisIndex <= maxScreenCellsCount; cutBeforeThisIndex++ {
		canBreakHere := false

		char := line[cutBeforeThisIndex].Rune
		onBreakableSpace := unicode.IsSpace(char) && char != NO_BREAK_SPACE
		if onBreakableSpace && !inLeadingWhitespace {
			// Break-OK whitespace, cut before this one!
			canBreakHere = true
		}

		if !onBreakableSpace {
			inLeadingWhitespace = false
		}

		// Accept cutting inside "]("" in Markdown links: [home](http://127.0.0.1)
		if cutBeforeThisIndex > 0 {
			previousChar := line[cutBeforeThisIndex-1].Rune
			if previousChar == ']' && char == '(' {
				canBreakHere = true
			}
		}

		// Break after single slashes, this is to enable breaking inside URLs / paths
		if cutBeforeThisIndex > 1 {
			beforeSlash := line[cutBeforeThisIndex-2].Rune
			slash := line[cutBeforeThisIndex-1].Rune
			afterSlash := char
			if beforeSlash != '/' && slash == '/' && afterSlash != '/' {
				canBreakHere = true
			}
		}

		if cutBeforeThisIndex > 0 {
			// Break after a hyphen / dash. That's something people do.
			previousChar := line[cutBeforeThisIndex-1].Rune
			if previousChar == '-' && char != '-' {
				canBreakHere = true
			}
		}

		if canBreakHere {
			bestCutPoint = cutBeforeThisIndex
		}

		screenCells += line[cutBeforeThisIndex].Width()
		if screenCells > maxScreenCellsCount {
			// We went too far
			if bestCutPoint > cutBeforeThisIndex {
				// We have to cut here
				bestCutPoint = cutBeforeThisIndex
			}
			break
		}
	}

	return bestCutPoint
}

// How many screen cells wide will this line be?
func getScreenCellCount(runes []textstyles.CellWithMetadata) int {
	cellCount := 0
	for _, rune := range runes {
		cellCount += rune.Width()
	}

	return cellCount
}

// matchListMarker checks if the given left-trimmed line starts with a list
// marker like "*", "1.", "-", or a command-line flag like "--foo". It returns
// the length of the marker in runes, or 0 if no marker is found.
func matchListMarker(line textstyles.CellWithMetadataSlice) int {
	if len(line) == 0 {
		return 0
	}

	r := line[0].Rune

	if r == '*' {
		return 1
	}

	if unicode.IsDigit(r) {
		i := 1
		for i < len(line) && unicode.IsDigit(line[i].Rune) {
			i++
		}
		if i < len(line) && line[i].Rune == '.' {
			return i + 1
		}
		return 0
	}

	if r == '-' {
		if len(line) > 1 && unicode.IsSpace(line[1].Rune) {
			return 1 // Matches a single "-"
		}

		i := 1
		if i < len(line) && line[i].Rune == '-' {
			i++ // Matches optional second "-"
		}

		if i < len(line) && !unicode.IsSpace(line[i].Rune) {
			// Matches 1+ non-whitespace characters
			for i < len(line) && !unicode.IsSpace(line[i].Rune) {
				i++
			}
			return i
		}
	}

	return 0
}

func getHangingIndentWidth(line textstyles.CellWithMetadataSlice) int {
	trimmed := line.WithoutSpaceLeft()
	leadingSpaces := len(line) - len(trimmed)
	if len(trimmed) == 0 {
		return leadingSpaces
	}

	markerLen := matchListMarker(trimmed)
	if markerLen <= 0 {
		return leadingSpaces
	}

	afterMarker := trimmed[markerLen:]
	withoutSpace := afterMarker.WithoutSpaceLeft()
	trailingSpaces := len(afterMarker) - len(withoutSpace)
	// We only trigger hanging indent if there's trailing space after the marker
	if trailingSpaces > 0 {
		return leadingSpaces + markerLen + trailingSpaces
	}

	return leadingSpaces
}

// Wrap one line of text to a maximum width.
//
// The return value will not contain any trailers, but the ContainsSearchHit
// field will be correctly set for sub-lines with search hits.
func wrapLine(width int, line textstyles.CellWithMetadataSlice) []textstyles.StyledRunesWithTrailer {
	// Trailing space risks showing up by itself on a line, which would just
	// look weird.
	line = line.WithoutSpaceRight()

	if width < minWrapWidth {
		return []textstyles.StyledRunesWithTrailer{{
			StyledRunes:       line,
			ContainsSearchHit: line.ContainsSearchHit(),
		}}
	}

	whitespaceLen := getHangingIndentWidth(line)
	// Don't use hanging indent if it leaves less than minWrapWidth characters for text.
	if width-whitespaceLen < minWrapWidth {
		whitespaceLen = 0
	}

	leadingWhitespace := make(textstyles.CellWithMetadataSlice, whitespaceLen)
	for i := range whitespaceLen {
		leadingWhitespace[i] = textstyles.CellWithMetadata{Rune: ' '}
	}

	screenCellCount := getScreenCellCount(line)
	if screenCellCount == 0 {
		return []textstyles.StyledRunesWithTrailer{{}}
	}

	capacity := len(line) / width
	if capacity == 0 {
		capacity = 1
	}
	wrapped := make([]textstyles.StyledRunesWithTrailer, 0, capacity)

	for {
		availableWidth := width
		isOnFirstLine := len(wrapped) == 0
		if !isOnFirstLine {
			availableWidth = width - whitespaceLen
		}

		if screenCellCount <= availableWidth {
			break
		}

		wrapWidth := getWrapCount(line, availableWidth)
		firstPart := line[:wrapWidth]
		if !isOnFirstLine {
			// Leading whitespace on wrapped lines would just look like
			// indentation, which would be weird for wrapped text.
			withoutLeadingWhitespace := firstPart.WithoutSpaceLeft()
			firstPart = append(append(textstyles.CellWithMetadataSlice{}, leadingWhitespace...), withoutLeadingWhitespace...)
		}

		wrapped = append(wrapped,
			textstyles.StyledRunesWithTrailer{
				StyledRunes:       firstPart.WithoutSpaceRight(),
				ContainsSearchHit: firstPart.ContainsSearchHit(),
			},
		)

		// These runes still need processing
		remaining := line[wrapWidth:].WithoutSpaceLeft()

		// Track how many screen cells are left to handle
		handledCount := len(line) - len(remaining)
		screenCellCount -= getScreenCellCount(line[:handledCount])

		// Prepare for the next iteration
		line = remaining
	}

	isOnFirstLine := len(wrapped) == 0
	if !isOnFirstLine {
		// Leading whitespace on wrapped lines would just look like
		// indentation, which would be weird for wrapped text.
		withoutLeadingWhitespace := line.WithoutSpaceLeft()
		line = append(append(textstyles.CellWithMetadataSlice{}, leadingWhitespace...), withoutLeadingWhitespace...)
	}
	line = line.WithoutSpaceRight()

	if len(line) > 0 {
		wrapped = append(wrapped,
			textstyles.StyledRunesWithTrailer{
				StyledRunes:       line,
				ContainsSearchHit: line.ContainsSearchHit(),
			},
		)
	}

	return wrapped
}
