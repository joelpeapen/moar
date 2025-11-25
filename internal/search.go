package internal

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/internal/linemetadata"
)

// Scroll to the next search hit, while the user is typing the search string.
func (p *Pager) scrollToSearchHits() {
	if p.searchPattern == nil {
		// This is not a search
		return
	}

	if p.searchHitIsVisible() {
		// Already on-screen
		return
	}

	if p.scrollRightToSearchHits() {
		// Found it to the right, done!
		return
	}

	lineIndex := p.scrollPosition.lineIndex(p)
	if lineIndex == nil {
		// No lines to search
		return
	}

	firstHitIndex := FindFirstHit(p.Reader(), *p.searchPattern, *lineIndex, nil, SearchDirectionForward)
	if firstHitIndex == nil {
		alreadyAtTheTop := (*lineIndex == linemetadata.Index{})
		if alreadyAtTheTop {
			// No match, can't wrap, give up
			return
		}

		// Try again from the top
		firstHitIndex = FindFirstHit(p.Reader(), *p.searchPattern, linemetadata.Index{}, lineIndex, SearchDirectionForward)
	}
	if firstHitIndex == nil {
		// No match, give up
		return
	}

	// Found a match on some line
	p.scrollPosition = NewScrollPositionFromIndex(*firstHitIndex, "scrollToSearchHits")

	p.leftColumnZeroBased = 0
	p.showLineNumbers = p.ShowLineNumbers
	if !p.searchHitIsVisible() {
		p.scrollRightToSearchHits()
	}
	p.centerSearchHitsVertically()
}

// Scroll to the next search hit, when the user presses 'n'.
func (p *Pager) scrollToNextSearchHit() {
	if p.searchPattern == nil {
		// Nothing to search for, never mind
		return
	}

	if p.Reader().GetLineCount() == 0 {
		// Nothing to search in, never mind
		return
	}

	if p.scrollRightToSearchHits() {
		// Found it to the right, done!
		return
	}

	if p.isViewing() && p.isScrolledToEnd() {
		p.mode = PagerModeNotFound{pager: p}
		return
	}

	var firstSearchIndex linemetadata.Index

	switch {
	case p.isViewing():
		// Start searching on the first line below the bottom of the screen
		position := p.getLastVisiblePosition().NextLine(1)
		firstSearchIndex = *position.lineIndex(p)

	case p.isNotFound():
		// Restart searching from the top
		p.mode = PagerModeViewing{pager: p}
		firstSearchIndex = linemetadata.Index{}

	default:
		panic(fmt.Sprint("Unknown search mode when finding next: ", p.mode))
	}

	firstHitIndex := FindFirstHit(p.Reader(), *p.searchPattern, firstSearchIndex, nil, SearchDirectionForward)
	if firstHitIndex == nil {
		p.mode = PagerModeNotFound{pager: p}
		return
	}
	p.scrollPosition = NewScrollPositionFromIndex(*firstHitIndex, "scrollToNextSearchHit")

	// Don't let any search hit scroll out of sight
	p.setTargetLine(nil)

	p.leftColumnZeroBased = 0
	p.showLineNumbers = p.ShowLineNumbers
	if !p.searchHitIsVisible() {
		p.scrollRightToSearchHits()
	}
	p.centerSearchHitsVertically()
}

// Scroll backwards to the previous search hit, while the user is typing the
// search string.
func (p *Pager) scrollToSearchHitsBackwards() {
	if p.searchPattern == nil {
		// This is not a search
		return
	}

	if p.searchHitIsVisible() {
		// Already on-screen
		return
	}

	if p.scrollLeftToSearchHits() {
		// Found it to the left, done!
		return
	}

	// Start at the top visible line
	lineIndex := p.scrollPosition.lineIndex(p)

	firstHitIndex := FindFirstHit(p.Reader(), *p.searchPattern, *lineIndex, nil, SearchDirectionBackward)
	if firstHitIndex == nil {
		lastReaderLineIndex := linemetadata.IndexFromLength(p.Reader().GetLineCount())
		if lastReaderLineIndex == nil {
			// In the first part of the search we had some lines to search.
			// Lines should never go away, so this should never happen.
			log.Error("Wrapped backwards search had no lines to search")
			return
		}

		lastVisibleLineIndex := p.getLastVisiblePosition().lineIndex(p)
		canWrap := (*lineIndex != *lastVisibleLineIndex)
		if !canWrap {
			// No match, can't wrap, give up
			return
		}

		// Try again from the bottom
		firstHitIndex = FindFirstHit(p.Reader(), *p.searchPattern, *lastReaderLineIndex, lineIndex, SearchDirectionBackward)
	}
	if firstHitIndex == nil {
		// No match, give up
		return
	}

	hitPosition := NewScrollPositionFromIndex(*firstHitIndex, "scrollToSearchHitsBackwards")

	// Scroll so that the first hit is at the bottom of the screen. If the
	// visible height is 1, we should scroll 0 steps.
	p.scrollPosition = hitPosition.PreviousLine(p.visibleHeight() - 1)

	p.scrollMaxRight()
	if !p.searchHitIsVisible() {
		p.scrollLeftToSearchHits()
	}
	p.centerSearchHitsVertically()
}

// Scroll backwards to the previous search hit, when the user presses 'N'.
func (p *Pager) scrollToPreviousSearchHit() {
	if p.searchPattern == nil {
		// Nothing to search for, never mind
		return
	}

	if p.Reader().GetLineCount() == 0 {
		// Nothing to search in, never mind
		return
	}

	if p.scrollLeftToSearchHits() {
		// Found it to the left, done!
		return
	}

	var firstSearchIndex linemetadata.Index

	switch {
	case p.isViewing():
		if p.scrollPosition.lineIndex(p).Index() == 0 {
			// Already at the top, can't go further up
			p.mode = PagerModeNotFound{pager: p}
			return
		}

		// Start searching on the first line above the top of the screen
		position := p.scrollPosition.PreviousLine(1)
		firstSearchIndex = *position.lineIndex(p)

	case p.isNotFound():
		// Restart searching from the bottom
		p.mode = PagerModeViewing{pager: p}
		firstSearchIndex = *linemetadata.IndexFromLength(p.Reader().GetLineCount())

	default:
		panic(fmt.Sprint("Unknown search mode when finding previous: ", p.mode))
	}

	hitIndex := FindFirstHit(p.Reader(), *p.searchPattern, firstSearchIndex, nil, SearchDirectionBackward)
	if hitIndex == nil {
		p.mode = PagerModeNotFound{pager: p}
		return
	}
	p.scrollPosition = *scrollPositionFromIndex("scrollToPreviousSearchHit", *hitIndex)

	// Don't let any search hit scroll out of sight
	p.setTargetLine(nil)

	// Prefer hits to the right
	p.scrollMaxRight()
	if !p.searchHitIsVisible() {
		p.scrollLeftToSearchHits()
	}
	p.centerSearchHitsVertically()
}

// Return true if any search hit is currently visible on screen.
//
// A search hit is considered visible if the first character of the hit is
// visible. This means that if the hit is longer than one character, the rest of
// it may be off-screen to the right. If that happens, the user can scroll right
// manually to see the rest of the hit.
func (p *Pager) searchHitIsVisible() bool {
	for _, row := range p.renderLines().lines {
		for _, cell := range row.cells {
			if cell.StartsSearchHit {
				// Found a search hit on screen!
				return true
			}
		}
	}

	// No search hits found
	return false
}

func (p *Pager) centerSearchHitsVertically() {
	if p.WrapLongLines {
		// FIXME: Centering is not supported when wrapping, future improvement!
		return
	}

	for {
		rendered := p.renderLines()
		firstHitRow := -1
		lastHitRow := -1
		for rowIndex, row := range rendered.inputLines {
			if !p.searchPattern.MatchString(row.Plain()) {
				continue
			}

			if firstHitRow == -1 {
				firstHitRow = rowIndex
			}
			lastHitRow = rowIndex
		}

		if firstHitRow == -1 || lastHitRow == -1 {
			log.Warn("No hits found while centering, how did we get here?")
			return
		}

		// If the visible height is 1, the center screen row is 0.
		centerScreenRowDoubled := p.visibleHeight() - 1

		centerHitRowDoubled := firstHitRow + lastHitRow

		// Divide by 2 here to get the amount of rows we need to scroll. We
		// postponed the division by 2 until now to avoid rounding errors.
		//
		// If the center screen row is 1 (3 lines visible), and the center hit
		// row is 2 (last screen line), we need to arrow down once.
		deltaRows := (centerHitRowDoubled - centerScreenRowDoubled) / 2

		newScrollPosition := p.scrollPosition.NextLine(deltaRows)
		if p.ScrollPositionsEqual(p.scrollPosition, newScrollPosition) {
			// No change, done!
			return
		}

		p.scrollPosition = newScrollPosition
	}
}

// If we are alredy too far right when you call this method, it will scroll
// left.
func (p *Pager) scrollMaxRight() {
	if p.WrapLongLines {
		// No horizontal scrolling when wrapping
		return
	}

	// First, render a screen scrolled to the far left so we know how much space
	// line numbers take.
	p.leftColumnZeroBased = 0
	p.showLineNumbers = p.ShowLineNumbers
	rendered := p.renderLines()

	// Find the widest line, in screen cells. Some runes are double-width.
	widestLineWidth := 0
	for _, inputLine := range rendered.inputLines {
		lineLength := inputLine.DisplayWidth()
		if lineLength > widestLineWidth {
			widestLineWidth = lineLength
		}
	}

	screenWidth, _ := p.screen.Size()

	availableWidth := screenWidth - rendered.numberPrefixWidth
	if widestLineWidth <= availableWidth {
		// All lines fit on screen, this means we're now max scrolled right
		return
	}

	p.showLineNumbers = false
	availableWidth += rendered.numberPrefixWidth
	if widestLineWidth <= availableWidth {
		// All lines fit on screen with line numbers off, this means we're now
		// max scrolled right
		return
	}

	// If the line width is 10 and the available width is also 10, we should
	// start at column 0.
	p.leftColumnZeroBased = widestLineWidth - availableWidth
}

// Scroll right looking for search hits. Return true if we found any.
func (p *Pager) scrollRightToSearchHits() bool {
	if p.WrapLongLines {
		// No horizontal scrolling when wrapping
		return false
	}

	restoreShowLineNumbers := p.showLineNumbers
	restoreLeftColumn := p.leftColumnZeroBased

	// Check how far right we can scroll at most. Factors involved:
	// - Screen width
	// - Length of longest visible line
	screenWidth, _ := p.screen.Size()

	widestLineWidth := 0 // In screen cells, some runes are double-width
	rendered := p.renderLines()
	for _, inputLine := range rendered.inputLines {
		lineLength := inputLine.DisplayWidth()
		if lineLength > widestLineWidth {
			widestLineWidth = lineLength
		}
	}

	// With a 10 wide screen and a 15 wide line (max index 14), the leftmost
	// screen column can at most be 5:
	//
	// Screen column: 0123456789
	// Line column:   5678901234
	maxLeftmostColumn := widestLineWidth - screenWidth

	// If we have line numbers and disable them, do any new hits appear?
	if rendered.numberPrefixWidth > 0 {
		// If the number prefix width is 4, and the screen width is 10, then 6 should be
		// the first newly revealed column index (10-4):
		//
		// Screen column: 0123456789
		// New cells:     ______1234
		//
		// But since the rightmost column can be covered by scroll-right we need to subtract
		// one more and get to 5.
		firstJustRevealedColumn := screenWidth - rendered.numberPrefixWidth - 1
		if firstJustRevealedColumn <= 0 {
			log.Info("Screen too narrow ({}) to disable line numbers for search hits, skipping", screenWidth)
			return false
		}

		p.showLineNumbers = false
		for _, row := range p.renderLines().lines {
			for column := firstJustRevealedColumn; column < len(row.cells); column++ {
				if row.cells[column].StartsSearchHit {
					// Found a search hit on screen!
					return true
				}
			}
		}
		p.showLineNumbers = restoreShowLineNumbers
	}

	for p.leftColumnZeroBased < maxLeftmostColumn {
		// FIXME: Rather than scrolling right one screen at a time, we should
		// consider scanning all lines for search hits and scrolling directly to the
		// first one that is off-screen to the right.

		// If the screen width is 1, and we have no line numbers, the answer
		// could be 1. But since the last column could be covered by scroll-right
		// markers, we'll say 0.
		firstNotVisibleColumn := p.leftColumnZeroBased + screenWidth - rendered.numberPrefixWidth - 1
		if firstNotVisibleColumn < 1 {
			log.Info("Screen is narrower than number prefix length, not scrolling right for search hits")
			p.showLineNumbers = restoreShowLineNumbers
			p.leftColumnZeroBased = restoreLeftColumn
			return false
		}

		// Minus one to account for the scroll-left marker that will cover the
		// first column after scrolling.
		scrollToColumn := firstNotVisibleColumn - 1

		p.showLineNumbers = false
		p.leftColumnZeroBased = scrollToColumn

		if p.searchHitIsVisible() {
			// A new hit showed up!
			if p.leftColumnZeroBased > maxLeftmostColumn {
				// Scrolled beyond max, adjust
				p.leftColumnZeroBased = maxLeftmostColumn
			}
			return true
		}
	}

	// Can't scroll right, pretend nothing happened
	p.showLineNumbers = restoreShowLineNumbers
	p.leftColumnZeroBased = restoreLeftColumn
	return false
}

// Scroll left looking for search hits. Return true if we found any.
func (p *Pager) scrollLeftToSearchHits() bool {
	if p.WrapLongLines {
		// No horizontal scrolling when wrapping
		return false
	}

	restoreLeftColumn := p.leftColumnZeroBased
	restoreShowLineNumbers := p.showLineNumbers

	screenWidth, _ := p.screen.Size()

	// If we go max left, which column will be the rightmost visible one?
	var fullLeftRightmostVisibleColumn int
	{
		p.showLineNumbers = p.ShowLineNumbers
		p.leftColumnZeroBased = 0
		rendered := p.renderLines()
		// If the screen width is 2, we have columns 0 and 1. The rightmost column can be covered by
		// scroll-right markers, so the first not-visible column when fully scrolled left is 0, or
		// "2 - 2".
		fullLeftRightmostVisibleColumn = screenWidth - 2 - rendered.numberPrefixWidth

		p.leftColumnZeroBased = restoreLeftColumn
		p.showLineNumbers = restoreShowLineNumbers
	}

	if fullLeftRightmostVisibleColumn < 0 {
		log.Info("Screen too narrow ({}) to scroll left for search hits, skipping", screenWidth)
		return false
	}

	// Keep scrolling left until we either find a search hit, or reach the
	// leftmost column with line numbers shown or not based on the user's
	// preference.
	for p.leftColumnZeroBased > 0 || (p.showLineNumbers != p.ShowLineNumbers) {
		// FIXME: Rather than scrolling left one screen at a time, we should
		// consider scanning all lines for search hits and scrolling directly to the
		// first one that is off-screen to the left.

		// Pretend the current leftmost column is not visible, since it could be
		// covered by scroll-left markers.
		lastNotVisibleColumn := p.leftColumnZeroBased

		// Go left
		if lastNotVisibleColumn <= fullLeftRightmostVisibleColumn {
			// Going max left will show the column we want
			p.showLineNumbers = p.ShowLineNumbers
			p.leftColumnZeroBased = 0
		} else {
			// Scroll left one screen.
			//
			// If the screen width is 3, and we want column 5 to be visible, and
			// there can be both scroll-left and scroll-right markers, we should
			// start at colum 4 (covered by a scroll-left marker), so that
			// column 5 is visible next to it.
			//
			// Set the leftmost column to 4, which is "5 - 3 + 2".
			scrollToColumn := lastNotVisibleColumn - screenWidth + 2
			if scrollToColumn < 0 {
				scrollToColumn = 0
			}

			p.leftColumnZeroBased = scrollToColumn

			// If showing line numbers was possible we should have ended up in
			// the other if branch ^
			p.showLineNumbers = false
		}

		if p.searchHitIsVisible() {
			// Found it!
			return true
		}
	}

	// Scrolling left didn't find anything, pretend nothing happened
	p.showLineNumbers = restoreShowLineNumbers
	p.leftColumnZeroBased = restoreLeftColumn
	return false
}

func (p *Pager) isViewing() bool {
	_, isViewing := p.mode.(PagerModeViewing)
	return isViewing
}

func (p *Pager) isNotFound() bool {
	_, isNotFound := p.mode.(PagerModeNotFound)
	return isNotFound
}
