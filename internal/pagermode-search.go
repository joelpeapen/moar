package internal

import (
	"regexp"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/twin"
)

type SearchDirection bool

const (
	SearchDirectionForward  SearchDirection = false
	SearchDirectionBackward SearchDirection = true
)

type PagerModeSearch struct {
	pager                 *Pager
	initialScrollPosition scrollPosition // Pager position before search started
	direction             SearchDirection
	inputBox              *InputBox
	searchHistoryIndex    int
	userEditedText        string
}

func NewPagerModeSearch(p *Pager, direction SearchDirection, initialScrollPosition scrollPosition) *PagerModeSearch {
	m := &PagerModeSearch{
		pager:                 p,
		initialScrollPosition: initialScrollPosition,
		direction:             direction,
		searchHistoryIndex:    len(p.searchHistory.entries), // Past the end
	}
	m.inputBox = &InputBox{
		accept: INPUTBOX_ACCEPT_ALL,
		onTextChanged: func(text string) {
			m.updateSearchPattern(text)
		},
	}
	return m
}

func (m PagerModeSearch) drawFooter(_ string, _ string) {
	prompt := "Search: "
	if m.direction == SearchDirectionBackward {
		prompt = "Search backwards: "
	}
	m.inputBox.draw(m.pager.screen, "Type to search, 'ENTER' submits, 'ESC' cancels, '↑↓' navigate history", prompt)
}

func (m *PagerModeSearch) updateSearchPattern(text string) {
	m.pager.searchString = text
	m.pager.searchPattern = toPattern(text)

	switch m.direction {
	case SearchDirectionBackward:
		m.pager.scrollToSearchHitsBackwards()
	case SearchDirectionForward:
		m.pager.scrollToSearchHits()
	}
}

// toPattern compiles a search string into a pattern.
//
// If the string contains only lower-case letter the pattern will be case insensitive.
//
// If the string is empty the pattern will be nil.
//
// If the string does not compile into a regexp the pattern will match the string verbatim
func toPattern(compileMe string) *regexp.Regexp {
	if len(compileMe) == 0 {
		return nil
	}

	hasUppercase := false
	for _, char := range compileMe {
		if unicode.IsUpper(char) {
			hasUppercase = true
		}
	}

	// Smart case; be case insensitive unless there are upper case chars
	// in the search string
	prefix := "(?i)"
	if hasUppercase {
		prefix = ""
	}

	pattern, err := regexp.Compile(prefix + compileMe)
	if err == nil {
		// Search string is a regexp
		return pattern
	}

	pattern, err = regexp.Compile(prefix + regexp.QuoteMeta(compileMe))
	if err == nil {
		// Pattern matching the string exactly
		return pattern
	}

	// Unable to create a match-string-verbatim pattern
	panic(err)
}

func (m *PagerModeSearch) moveSearchHistoryIndex(delta int) {
	if len(m.pager.searchHistory.entries) == 0 {
		return
	}

	m.searchHistoryIndex += delta
	if m.searchHistoryIndex < 0 {
		m.searchHistoryIndex = 0
	}
	if m.searchHistoryIndex > len(m.pager.searchHistory.entries) {
		m.searchHistoryIndex = len(m.pager.searchHistory.entries) // Beyond the end of the history
	}

	if m.searchHistoryIndex == len(m.pager.searchHistory.entries) {
		// Reset to whatever the user typed last
		m.inputBox.setText(m.userEditedText)
	} else {
		// Get the history entry
		m.inputBox.setText(m.pager.searchHistory.entries[m.searchHistoryIndex])
	}
}

func (m *PagerModeSearch) onKey(key twin.KeyCode) {
	if m.inputBox.handleKey(key) {
		m.searchHistoryIndex = len(m.pager.searchHistory.entries) // Reset history index when user types
		m.userEditedText = m.inputBox.text
		return
	}

	switch key {
	case twin.KeyEnter:
		m.pager.searchHistory.addEntry(m.inputBox.text)
		m.pager.mode = PagerModeViewing{pager: m.pager}

	case twin.KeyEscape:
		m.pager.searchHistory.addEntry(m.inputBox.text)
		m.pager.mode = PagerModeViewing{pager: m.pager}
		m.pager.scrollPosition = m.initialScrollPosition

	case twin.KeyPgUp, twin.KeyPgDown:
		m.pager.searchHistory.addEntry(m.inputBox.text)
		m.pager.mode = PagerModeViewing{pager: m.pager}
		m.pager.mode.onKey(key)

	case twin.KeyUp:
		m.moveSearchHistoryIndex(-1)

	case twin.KeyDown:
		m.moveSearchHistoryIndex(1)

	default:
		log.Debugf("Unhandled search key event %v", key)
	}
}

func (m *PagerModeSearch) onRune(char rune) {
	m.searchHistoryIndex = len(m.pager.searchHistory.entries) // Reset history index when user types
	m.inputBox.handleRune(char)
	m.userEditedText = m.inputBox.text
}
