package internal

import (
	"unicode"

	"github.com/walles/moor/v2/internal/textstyles"
)

// From: https://www.compart.com/en/unicode/U+00A0
//
//revive:disable-next-line:var-naming
const NO_BREAK_SPACE = '\xa0'

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

// Wrap one line of text to a maximum width.
//
// The return value will not contain any trailers, but the ContainsSearchHit
// field will be correctly set for sub-lines with search hits.
func wrapLine(width int, line textstyles.CellWithMetadataSlice) []textstyles.StyledRunesWithTrailer {
	// Trailing space risks showing up by itself on a line, which would just
	// look weird.
	line = line.WithoutSpaceRight()

	screenCellCount := getScreenCellCount(line)
	if screenCellCount == 0 {
		return []textstyles.StyledRunesWithTrailer{{}}
	}

	wrapped := make([]textstyles.StyledRunesWithTrailer, 0, len(line)/width)
	for screenCellCount > width {
		wrapWidth := getWrapCount(line, width)
		firstPart := line[:wrapWidth]
		isOnFirstLine := len(wrapped) == 0
		if !isOnFirstLine {
			// Leading whitespace on wrapped lines would just look like
			// indentation, which would be weird for wrapped text.
			firstPart = firstPart.WithoutSpaceLeft()
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
		line = line.WithoutSpaceLeft()
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
