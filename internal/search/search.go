package search

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charlievieth/strcase"
)

type Search struct {
	findMe string

	// If this is false it means the input has to be interpreted as a regexp.
	isSubstringSearch bool

	hasUppercase bool

	pattern *regexp.Regexp
}

func (search Search) Equals(other Search) bool {
	return search.findMe == other.findMe
}

func (search Search) String() string {
	return search.findMe
}

func For(s string) Search {
	search := Search{}
	search.For(s)
	return search
}

func (search *Search) For(s string) *Search {
	search.findMe = s
	if s == "" {
		// No search
		search.pattern = nil
		return search
	}

	var err error
	hasSpecialChars := regexp.QuoteMeta(s) != s
	search.pattern, err = regexp.Compile(s)
	isValidRegexp := err == nil
	regexpMatchingRequired := hasSpecialChars && isValidRegexp
	search.isSubstringSearch = !regexpMatchingRequired

	search.hasUppercase = false
	for _, char := range s {
		if unicode.IsUpper(char) {
			search.hasUppercase = true
			break
		}
	}

	if search.isSubstringSearch {
		// No need to compile a regexp pattern since GetMatchRanges and Matches
		// use fast paths for substring searches.
		search.pattern = nil
		return search
	}

	// At this point we know it's a valid regexp, and that it does include
	// regexp specific characters. We also know the pattern has been
	// successfully compiled.

	return search
}

func (search *Search) Clear() {
	search.findMe = ""
	search.pattern = nil
}

func (search Search) Active() bool {
	return search.findMe != ""
}

func (search Search) Inactive() bool {
	return search.findMe == ""
}

func (search Search) Matches(line string) bool {
	if search.findMe == "" {
		return false
	}

	if search.isSubstringSearch && search.hasUppercase {
		// Case sensitive substring search
		return strings.Contains(line, search.findMe)
	}

	if search.isSubstringSearch && !search.hasUppercase {
		// Case insensitive substring search
		return strcase.Contains(line, search.findMe)
	}

	// Regexp search

	if !search.hasUppercase {
		// Regexp is already lowercase, do the same to the line to make the
		// search case insensitive
		line = strings.ToLower(line)
	}

	return search.pattern.MatchString(line)
}

// getMatchRanges locates one or more regexp matches in a string
func (search Search) GetMatchRanges(String string) *MatchRanges {
	if search.Inactive() {
		return nil
	}

	if !search.hasUppercase {
		// Case insensitive search, lowercase the string. The pattern is already
		// lowercase whenever hasUppercase is false.
		String = strings.ToLower(String)
	}

	regexpSearch := !search.isSubstringSearch
	if regexpSearch {
		return &MatchRanges{
			Matches: toRunePositions(search.pattern.FindAllStringIndex(String, -1), String),
		}
	}

	// Faster code for non-regexp search follows

	offset := 0
	currentByteIndex := 0
	currentRuneIndex := 0

	var matches [][2]int

	for {
		idx := strings.Index(String[offset:], search.findMe)
		if idx == -1 {
			break
		}

		matchStart := offset + idx
		matchEnd := matchStart + len(search.findMe)

		// Advance to the start of the match
		currentRuneIndex += utf8.RuneCountInString(String[currentByteIndex:matchStart])
		fromRuneIndex := currentRuneIndex

		// Advance to the end of the match
		currentRuneIndex += utf8.RuneCountInString(String[matchStart:matchEnd])
		toRuneIndex := currentRuneIndex

		currentByteIndex = matchEnd

		matches = append(matches, [2]int{fromRuneIndex, toRuneIndex})
		offset = matchEnd
	}
	return &MatchRanges{
		Matches: matches,
	}
}

// Convert byte indices to rune indices
func toRunePositions(byteIndices [][]int, matchedString string) [][2]int {
	var returnMe [][2]int
	if len(byteIndices) == 0 {
		// Nothing to see here, move along
		return returnMe
	}

	currentByteIndex := 0
	currentRuneIndex := 0

	for _, bytePair := range byteIndices {
		fromByteIndex := bytePair[0]
		toByteIndex := bytePair[1]

		// Advance to the start of the match
		currentRuneIndex += utf8.RuneCountInString(matchedString[currentByteIndex:fromByteIndex])
		fromRuneIndex := currentRuneIndex

		// Advance to the end of the match
		currentRuneIndex += utf8.RuneCountInString(matchedString[fromByteIndex:toByteIndex])
		toRuneIndex := currentRuneIndex

		currentByteIndex = toByteIndex

		returnMe = append(returnMe, [2]int{fromRuneIndex, toRuneIndex})
	}

	return returnMe
}
