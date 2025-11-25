package internal

import (
	"regexp"
	"strings"
	"testing"

	"github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/twin"
	"gotest.tools/v3/assert"
)

func TestScrollMaxRight_AllLinesFitWithLineNumbers(t *testing.T) {
	// Case 2: All lines fit with line numbers
	screenWidth := 20
	widestLineWidth := 16 // Just below available width
	line := strings.Repeat("x", widestLineWidth)
	reader := reader.NewFromTextForTesting("test", line)
	screen := twin.NewFakeScreen(screenWidth, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.WrapLongLines = false

	pager.scrollMaxRight()
	assert.Equal(t, 0, pager.leftColumnZeroBased)
	assert.Equal(t, true, pager.showLineNumbers)
}

func TestScrollMaxRight_AllLinesFitWithoutLineNumbers1(t *testing.T) {
	// Case 2: All lines fit with line numbers
	screenWidth := 20
	widestLineWidth := 17 // Just above available width with line numbers
	line := strings.Repeat("x", widestLineWidth)
	reader := reader.NewFromTextForTesting("test", line)
	screen := twin.NewFakeScreen(screenWidth, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.WrapLongLines = false

	pager.scrollMaxRight()
	assert.Equal(t, 0, pager.leftColumnZeroBased)
	assert.Equal(t, false, pager.showLineNumbers)
}

func TestScrollMaxRight_AllLinesFitWithoutLineNumbers2(t *testing.T) {
	// Case 3: All lines fit only if line numbers are hidden, just at the edge
	screenWidth := 20
	widestLineWidth := 20 // Above available with line numbers, just below without
	line := strings.Repeat("x", widestLineWidth)
	reader := reader.NewFromTextForTesting("test", line)
	screen := twin.NewFakeScreen(screenWidth, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.WrapLongLines = false

	pager.scrollMaxRight()
	assert.Equal(t, 0, pager.leftColumnZeroBased)
	assert.Equal(t, false, pager.showLineNumbers)
}

func TestScrollMaxRight_WidestLineExceedsScreenWidth_Edge(t *testing.T) {
	// Case 4: Widest line just exceeds available width even without line numbers
	screenWidth := 20
	widestLineWidth := 21 // Just above available width without line numbers
	line := strings.Repeat("x", widestLineWidth)
	reader := reader.NewFromTextForTesting("test", line)
	screen := twin.NewFakeScreen(screenWidth, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.WrapLongLines = false

	pager.scrollMaxRight()
	assert.Equal(t, 1, pager.leftColumnZeroBased)
	assert.Equal(t, false, pager.showLineNumbers)
}

func modeName(pager *Pager) string {
	switch pager.mode.(type) {
	case PagerModeViewing:
		return "Viewing"
	case PagerModeNotFound:
		return "NotFound"
	case *PagerModeSearch:
		return "Search"
	case *PagerModeGotoLine:
		return "GotoLine"
	default:
		panic("Unknown pager mode")
	}
}

// Create a pager with three screen lines reading from a six lines stream
func createThreeLinesPager(t *testing.T) *Pager {
	reader := reader.NewFromTextForTesting("", "a\nb\nc\nd\ne\nf\n")

	screen := twin.NewFakeScreen(20, 3)
	pager := NewPager(reader)

	pager.screen = screen

	assert.Equal(t, "Viewing", modeName(pager), "Initial pager state")

	return pager
}

func TestScrollToNextSearchHit_StartAtBottom(t *testing.T) {
	// Create a pager scrolled to the last line
	pager := createThreeLinesPager(t)
	pager.scrollToEnd()

	// Set the search to something that doesn't exist in this pager
	pager.searchString = "xxx"
	pager.searchPattern = toPattern(pager.searchString)

	// Scroll to the next search hit
	pager.scrollToNextSearchHit()

	assert.Equal(t, "NotFound", modeName(pager))
}

func TestScrollToNextSearchHit_StartAtTop(t *testing.T) {
	// Create a pager scrolled to the first line
	pager := createThreeLinesPager(t)

	// Set the search to something that doesn't exist in this pager
	pager.searchString = "xxx"
	pager.searchPattern = toPattern(pager.searchString)

	// Scroll to the next search hit
	pager.scrollToNextSearchHit()

	assert.Equal(t, "NotFound", modeName(pager))
}

func TestScrollToNextSearchHit_WrapAfterNotFound(t *testing.T) {
	// Create a pager scrolled to the last line
	pager := createThreeLinesPager(t)
	pager.scrollToEnd()

	// Search for "a", it's on the first line (ref createThreeLinesPager())
	pager.searchString = "a"
	pager.searchPattern = toPattern(pager.searchString)

	// Scroll to the next search hit, this should take us into _NotFound
	pager.scrollToNextSearchHit()
	assert.Equal(t, "NotFound", modeName(pager))

	// Scroll to the next search hit, this should wrap the search and take us to
	// the top
	pager.scrollToNextSearchHit()
	assert.Equal(t, "Viewing", modeName(pager))
	assert.Assert(t, pager.lineIndex().IsZero())
}

func TestScrollToNextSearchHit_WrapAfterFound(t *testing.T) {
	// Create a pager scrolled to the last line
	pager := createThreeLinesPager(t)
	pager.scrollToEnd()

	// Search for "f", it's on the last line (ref createThreeLinesPager())
	pager.searchString = "f"
	pager.searchPattern = toPattern(pager.searchString)

	// Scroll to the next search hit, this should take us into _NotFound
	pager.scrollToNextSearchHit()
	assert.Equal(t, "NotFound", modeName(pager))

	// Scroll to the next search hit, this should wrap the search and take us
	// back to the bottom again
	pager.scrollToNextSearchHit()
	assert.Equal(t, "Viewing", modeName(pager))
	assert.Equal(t, 4, pager.lineIndex().Index())
}

// Ref: https://github.com/walles/moor/issues/152
func Test152(t *testing.T) {
	// Show a pager on a five lines terminal
	reader := reader.NewFromTextForTesting("", "a\nab\nabc\nabcd\nabcde\nabcdef\n")
	screen := twin.NewFakeScreen(20, 5)
	pager := NewPager(reader)
	pager.screen = screen
	assert.Equal(t, "Viewing", modeName(pager), "Initial pager state")

	searchMode := NewPagerModeSearch(pager, SearchDirectionForward, pager.scrollPosition)
	pager.mode = searchMode
	// Search for the first not-visible hit
	searchMode.inputBox.setText("abcde")

	assert.Equal(t, "Search", modeName(pager))
	assert.Equal(t, 2, pager.lineIndex().Index())
}

func TestScrollLeftToSearchHits_NoLineNumbers(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "a234567890")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = false
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 1

	assert.Equal(t, true, pager.scrollLeftToSearchHits())
	assert.Equal(t, 0, pager.leftColumnZeroBased)
	assert.Equal(t, false, pager.showLineNumbers)
}

func TestScrollLeftToSearchHits_WithLineNumbers(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "a234567890")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 1

	assert.Equal(t, true, pager.scrollLeftToSearchHits())
	assert.Equal(t, 0, pager.leftColumnZeroBased)
	assert.Equal(t, true, pager.showLineNumbers)
}

func TestScrollLeftToSearchHits_ScrollOneScreen(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "01234567890a234567890123456789")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 20

	assert.Equal(t, true, pager.scrollLeftToSearchHits())
	assert.Equal(t, 4, pager.leftColumnZeroBased,
		"We started at 20, screen is 10 wide, each scroll moves 8 to compensate for scroll markers, and 20-8-8=4")
	assert.Equal(t, false, pager.showLineNumbers)
}

// If the screen is too narrow for line numbers, there's no point in scrolling.
// This test has provoked some panics.
func TestScrollRightToSearchHits_NarrowScreen(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "abcdefg")
	screen := twin.NewFakeScreen(1, 5)
	pager := NewPager(reader)
	pager.screen = screen

	pager.showLineNumbers = false
	assert.Equal(t, pager.scrollRightToSearchHits(), false)

	pager.showLineNumbers = true
	assert.Equal(t, pager.scrollRightToSearchHits(), false)
}

func TestScrollRightToSearchHits_DisableLineNumbersToSeeHit0(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "12345a")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.showLineNumbers = true
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 0

	assert.Equal(t, true, pager.scrollRightToSearchHits())
	assert.Equal(t, 0, pager.leftColumnZeroBased, "Should scroll right to bring 'a' into view")
	assert.Equal(t, false, pager.showLineNumbers, "Should disable line numbers to fit search hit")
}

func TestScrollRightToSearchHits_DisableLineNumbersToSeeHit(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "123456789a")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = true
	pager.showLineNumbers = true
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 0

	assert.Equal(t, true, pager.scrollRightToSearchHits())
	assert.Equal(t, 0, pager.leftColumnZeroBased, "Should scroll right to bring 'a' into view")
	assert.Equal(t, false, pager.showLineNumbers, "Should disable line numbers to fit search hit")
}

func TestScrollRightToSearchHits_HiddenByScrollMarker(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "123456789a234567890")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = false
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 0

	assert.Equal(t, true, pager.scrollRightToSearchHits())
	assert.Equal(t, 8, pager.leftColumnZeroBased, "Should scroll right to bring 'a' into view from behind scroll marker")
}

// Repro case for https://github.com/walles/moor/issues/337.
func TestScrollRightToSearchHits_Issue337(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "123456a89012345")
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = false
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 0

	assert.Equal(t, false, pager.scrollRightToSearchHits(), "Search hit was already visible, should not have scrolled")
	assert.Equal(t, 0, pager.leftColumnZeroBased, "Should not have scrolled")
}

func TestScrollRightToSearchHits_LastCharHit(t *testing.T) {
	const line = "x0123456789a"
	reader := reader.NewFromTextForTesting("", line)
	screen := twin.NewFakeScreen(10, 5)
	pager := NewPager(reader)
	pager.screen = screen
	pager.ShowLineNumbers = false
	pager.showLineNumbers = false
	pager.searchString = "a"
	pager.searchPattern = toPattern("a")
	pager.leftColumnZeroBased = 0

	assert.Equal(t, true, pager.scrollRightToSearchHits())
	width, _ := screen.Size()
	lastCol := pager.leftColumnZeroBased + width - 1
	assert.Equal(t, strings.Index(line, "a"), lastCol, "Search hit should be in the last screen column")
}

func TestScrollRightToSearchHits_OnlyStartOfHitTriggers(t *testing.T) {
	// Arrange: create a line with a multi-rune search hit
	line := "abcDEFGHIJKLMNOPQRSTUVWXYZ"
	pattern := regexp.MustCompile("DEFGHIJ") // Match starts at index 3
	readerImpl := reader.NewFromTextForTesting("test", line)
	screen := twin.NewFakeScreen(5, 2) // Narrow screen to force scrolling
	pager := NewPager(readerImpl)
	pager.screen = screen
	pager.WrapLongLines = false
	pager.ShowLineNumbers = false
	pager.showLineNumbers = false
	pager.searchPattern = pattern

	assert.Assert(t, !pager.scrollRightToSearchHits(), "No more search hit starts to the right, should not scroll")
}
