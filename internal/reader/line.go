package reader

import (
	"regexp"

	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

// A Line represents a line of text that can / will be paged
type Line struct {
	raw   string
	plain *string // This will be populated by the reader's Get methods if not set
}

// NewLine creates a new Line from a (potentially ANSI / man page formatted) string
func NewLine(raw string) Line {
	return Line{
		raw: raw,
	}
}

// Returns a representation of the string split into styled tokens. Any regexp
// matches are highlighted. A nil regexp means no highlighting.
func (line *Line) HighlightedTokens(
	plainTextStyle twin.Style,
	searchHitStyle twin.Style,
	search *regexp.Regexp,
	lineIndex *linemetadata.Index,
) textstyles.StyledRunesWithTrailer {
	matchRanges := getMatchRanges(line.plain, search)

	fromString := textstyles.StyledRunesFromString(plainTextStyle, line.raw, lineIndex)
	returnRunes := make([]textstyles.CellWithMetadata, 0, len(fromString.StyledRunes))
	lastWasSearchHit := false
	for _, token := range fromString.StyledRunes {
		style := token.Style
		searchHit := matchRanges.InRange(len(returnRunes))
		if searchHit {
			// Highlight the search hit
			style = searchHitStyle
		}

		returnRunes = append(returnRunes, textstyles.CellWithMetadata{
			Rune:            token.Rune,
			Style:           style,
			IsSearchHit:     searchHit,
			StartsSearchHit: searchHit && !lastWasSearchHit,
		})
		lastWasSearchHit = searchHit
	}

	return textstyles.StyledRunesWithTrailer{
		StyledRunes:       returnRunes,
		Trailer:           fromString.Trailer,
		ContainsSearchHit: !matchRanges.Empty(),
	}
}

// Plain returns a plain text representation of the initial string
func (line *Line) Plain() string {
	return *line.plain
}

func (line *Line) HasManPageFormatting() bool {
	return textstyles.HasManPageFormatting(line.raw)
}
