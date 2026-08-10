package internal

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/twin"
	"gotest.tools/v3/assert"
)

// How long to wait for the pager to get somewhere. Long enough that exceeding it
// means the pager is stuck rather than slow.
const pagerTimeout = time.Second

// An event the pager has no handling for.
//
// Sending one on an unbuffered event channel completes only once the main loop
// receives it, and the loop reads events after deciding whether to paint. So a
// completed send means the decision has been made.
type nudge struct{}

// A screen that can run the pager main loop, and that counts paints.
//
// twin.FakeScreen.Events() returns nil, and receiving from a nil channel blocks
// forever, so the main loop cannot be run with a plain FakeScreen.
//
// Show() is what takes over the user's terminal, alternate screen included, so
// counting Show() calls is counting how much the user gets to see.
type countingScreen struct {
	*twin.FakeScreen
	events chan twin.Event
	shows  atomic.Int64
}

func newCountingScreen(width int, height int) *countingScreen {
	return &countingScreen{
		FakeScreen: twin.NewFakeScreen(width, height),

		// Unbuffered, so that sending on it is a handshake with the main loop
		events: make(chan twin.Event),
	}
}

func (screen *countingScreen) Events() chan twin.Event {
	return screen.events
}

func (screen *countingScreen) Show() {
	screen.shows.Add(1)
	screen.FakeScreen.Show()
}

// Start paging in the background, and stop paging again when the test ends.
//
// Returns a channel that is closed once paging is done.
//
// Stopping matters even for tests that expect the pager to quit on its own: a
// pager left running keeps painting on its spinner ticks for as long as the test
// binary lives, and it shares package level style state with the next test.
func startPagingInBackground(t *testing.T, pager *Pager, screen *countingScreen) chan struct{} {
	t.Helper()

	pagingDone := make(chan struct{})
	go func() {
		pager.StartPaging(screen, nil, nil)
		close(pagingDone)
	}()

	t.Cleanup(func() {
		select {
		case screen.events <- twin.EventExit{}:
		case <-pagingDone:
		}

		<-pagingDone
	})

	return pagingDone
}

// Wait for the pager to decide whether to paint, and report whether it painted.
func paintedBeforeReadingEvents(t *testing.T, screen *countingScreen, pagingDone chan struct{}) bool {
	t.Helper()

	select {
	case screen.events <- nudge{}:
		return screen.shows.Load() > 0

	case <-pagingDone:
		t.Fatal("Pager quit instead of deciding whether to paint")

	case <-time.After(pagerTimeout):
		t.Fatal("Pager never got as far as reading events")
	}

	return false // Unreachable, t.Fatal() above ends the test
}

// With --quit-if-one-screen, contents that fit must never be painted, not even
// while we are still waiting to find out whether they fit.
//
// Highlighting can change the answer, so until it is done we don't know. Ask the
// user's terminal for nothing in the meantime: painting first and quitting right
// after is the blink from https://github.com/walles/moor/issues/425.
func TestQuitIfOneScreenPaintsNothingWhileHighlighting(t *testing.T) {
	testReader := reader.NewFromTextForTesting("", "hello\nworld")

	// NewFromTextForTesting() leaves this nil, and the pager needs it to hear
	// about highlighting finishing. Assign before paging starts, the pager reads
	// it from a goroutine of its own.
	testReader.MaybeDone = make(chan bool, 2)

	// Highlighting pending, like for a source file argument or piped JSON
	testReader.HighlightingDone.Store(false)

	screen := newCountingScreen(20, 10)

	pager := NewPager(testReader)
	pager.QuitIfOneScreen = true

	pagingDone := startPagingInBackground(t, pager, screen)

	painted := paintedBeforeReadingEvents(t, screen, pagingDone)
	assert.Assert(t, !painted,
		"Nothing should be painted while we don't know whether the contents fit")

	// Now we know, and two lines do fit on ten, so the pager should quit
	testReader.HighlightingDone.Store(true)
	testReader.MaybeDone <- true

	select {
	case <-pagingDone:
	case <-time.After(pagerTimeout):
		t.Fatal("Pager should have quit, two lines fit on ten")
	}
}

// Contents that do not fit must be painted right away, without waiting for
// highlighting.
//
// Highlighting only ever gets us more lines to show, so contents that are
// already too tall are staying too tall. We know we are staying, and waiting
// would be latency in front of the first paint for nothing.
func TestQuitIfOneScreenPaintsContentsThatDoNotFit(t *testing.T) {
	testReader := reader.NewFromTextForTesting("", strings.Repeat("hello\n", 20))
	testReader.MaybeDone = make(chan bool, 2)
	testReader.HighlightingDone.Store(false)

	screen := newCountingScreen(20, 10)

	pager := NewPager(testReader)
	pager.QuitIfOneScreen = true

	pagingDone := startPagingInBackground(t, pager, screen)

	painted := paintedBeforeReadingEvents(t, screen, pagingDone)
	assert.Assert(t, painted,
		"Contents that don't fit should be painted without waiting for highlighting")
}
