package reader

import (
	"regexp"

	"github.com/rivo/uniseg"
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
)

type NumberedLine struct {
	Index  linemetadata.Index
	Number linemetadata.Number
	Line   *Line
}

func (nl *NumberedLine) Plain() string {
	return nl.Line.Plain()
}

func (nl *NumberedLine) HighlightedTokens(plainTextStyle twin.Style, searchHitStyle twin.Style, search *regexp.Regexp) textstyles.StyledRunesWithTrailer {
	return nl.Line.HighlightedTokens(plainTextStyle, searchHitStyle, search, &nl.Index)
}

func (nl *NumberedLine) DisplayWidth() int {
	width := 0
	for _, r := range nl.Plain() {
		width += uniseg.StringWidth(string(r))
	}
	return width
}
