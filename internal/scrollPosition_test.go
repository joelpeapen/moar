package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/twin"
	"gotest.tools/v3/assert"
)

const screenHeight = 60

// Repro for: https://github.com/walles/moor/issues/166
func testCanonicalize1000(t *testing.T, withStatusBar bool, currentStartLine linemetadata.Index, lastVisibleLine linemetadata.Index) {
	pager := Pager{}
	pager.screen = twin.NewFakeScreen(100, screenHeight)
	pager.readers = []*reader.ReaderImpl{reader.NewFromTextForTesting("test", strings.Repeat("a\n", 2000))}
	pager.filteringReader = FilteringReader{
		BackingReader: pager.readers[pager.currentReader],
		FilterPattern: &pager.filterPattern,
	}
	pager.ShowLineNumbers = true
	pager.showLineNumbers = true
	pager.ShowStatusBar = withStatusBar
	pager.scrollPosition = scrollPosition{
		internalDontTouch: scrollPositionInternal{
			lineIndex:        &currentStartLine,
			deltaScreenLines: 0,
			name:             "findFirstHit",
			canonicalizing:   false,
		},
	}

	lastVisiblePosition := scrollPosition{
		internalDontTouch: scrollPositionInternal{
			lineIndex:        &lastVisibleLine,
			deltaScreenLines: 0,
			name:             "Last Visible Position",
		},
	}

	assert.Equal(t, *lastVisiblePosition.lineIndex(&pager), lastVisibleLine)
}

func TestCanonicalize1000WithStatusBar(t *testing.T) {
	for startLine := 0; startLine < 1500; startLine++ {
		t.Run(fmt.Sprint("startLine=", startLine), func(t *testing.T) {
			testCanonicalize1000(t, true,
				linemetadata.IndexFromZeroBased(startLine),
				linemetadata.IndexFromZeroBased(startLine+screenHeight-2),
			)
		})
	}
}

func TestCanonicalize1000WithoutStatusBar(t *testing.T) {
	for startLine := 0; startLine < 1500; startLine++ {
		t.Run(fmt.Sprint("startLine=", startLine), func(t *testing.T) {
			testCanonicalize1000(t, true,
				linemetadata.IndexFromZeroBased(startLine),
				linemetadata.IndexFromZeroBased(startLine+screenHeight-1),
			)
		})
	}
}

// Try scrolling between two points, on a 80 x screenHeight screen with 1492
// lines of input.
func tryScrollAmount(t *testing.T, scrollFrom linemetadata.Index, scrollDistance int) {
	// Create 1492 lines of single-char content
	pager := Pager{}
	pager.screen = twin.NewFakeScreen(80, screenHeight)
	pager.readers = []*reader.ReaderImpl{reader.NewFromTextForTesting("test", strings.Repeat("x\n", 1492))}
	pager.filteringReader = FilteringReader{
		BackingReader: pager.readers[pager.currentReader],
		FilterPattern: &pager.filterPattern,
	}
	pager.ShowLineNumbers = true
	pager.showLineNumbers = true

	pager.scrollPosition = scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:             "tryScrollAmount",
			lineIndex:        &scrollFrom,
			deltaScreenLines: scrollDistance,
		},
	}

	// Trigger rendering (and canonicalization). If the prefix is miscomputed
	// this would previously panic inside createLinePrefix().
	rendered := pager.renderLines()

	// Sanity check the result
	assert.Assert(t, rendered.lines != nil)
	assert.Equal(t, len(rendered.lines), pager.visibleHeight())
	assert.Equal(t, rendered.lines[0].inputLineIndex, scrollFrom.NonWrappingAdd(scrollDistance))
}

// Repro for https://github.com/walles/moor/issues/313: Rapid scroll
// (deltaScreenLines > 0) crossing from 3 to 4 digits must not panic due to
// too-short number prefix length.
func TestFastScrollAcross1000DoesNotPanic(t *testing.T) {
	tryScrollAmount(t, linemetadata.IndexFromZeroBased(900), 200)
}

// Repro for https://github.com/walles/moor/issues/338
func TestIssue338(t *testing.T) {
	tryScrollAmount(t, linemetadata.IndexFromZeroBased(1000), -60)
}

func TestMultipleScrollStartsAcross1000DoNotPanic(t *testing.T) {
	for scrollFrom := 1000 - screenHeight - 10; scrollFrom <= 1000; scrollFrom++ {
		tryScrollAmount(t, linemetadata.IndexFromZeroBased(scrollFrom), screenHeight)
	}
}

func TestMultipleScrollDistancesAcross1000DoNotPanic(t *testing.T) {
	scrollFrom := 1000 - screenHeight - 10
	for scrollDistance := 0; scrollDistance <= 3*screenHeight; scrollDistance++ {
		tryScrollAmount(t, linemetadata.IndexFromZeroBased(scrollFrom), scrollDistance)
	}
}

func TestMultipleBackwardsScrollStartsAcross1000DoNotPanic(t *testing.T) {
	for scrollFrom := 1000 + screenHeight + 10; scrollFrom >= 1000; scrollFrom-- {
		tryScrollAmount(t, linemetadata.IndexFromZeroBased(scrollFrom), -screenHeight)
	}
}

func TestMultipleBackwardsScrollDistancesAcross1000DoNotPanic(t *testing.T) {
	scrollFrom := 1000 + screenHeight + 10
	for scrollDistance := 0; scrollDistance <= 3*screenHeight; scrollDistance++ {
		tryScrollAmount(t, linemetadata.IndexFromZeroBased(scrollFrom), -scrollDistance)
	}
}
