package internal

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/twin"
	"gotest.tools/v3/assert"

	log "github.com/sirupsen/logrus"
)

func TestFindFirstHitSimple(t *testing.T) {
	reader := reader.NewFromTextForTesting("TestFindFirstHitSimple", "AB")
	assert.NilError(t, reader.Wait())

	hit := FindFirstHit(reader, *toPattern("AB"), linemetadata.Index{}, nil, SearchDirectionForward)
	assert.Assert(t, hit.IsZero())
}

func TestFindFirstHitAnsi(t *testing.T) {
	reader := reader.NewFromTextForTesting("", "A\x1b[30mB")
	assert.NilError(t, reader.Wait())

	hit := FindFirstHit(reader, *toPattern("AB"), linemetadata.Index{}, nil, SearchDirectionForward)
	assert.Assert(t, hit.IsZero())
}

func TestFindFirstHitNoMatch(t *testing.T) {
	reader := reader.NewFromTextForTesting("TestFindFirstHitSimple", "AB")
	assert.NilError(t, reader.Wait())

	hit := FindFirstHit(reader, *toPattern("this pattern should not be found"), linemetadata.Index{}, nil, SearchDirectionForward)
	assert.Assert(t, hit == nil)
}

func TestFindFirstHitNoMatchBackwards(t *testing.T) {
	reader := reader.NewFromTextForTesting("TestFindFirstHitSimple", "AB")
	assert.NilError(t, reader.Wait())

	theEnd := *linemetadata.IndexFromLength(reader.GetLineCount())

	hit := FindFirstHit(reader, *toPattern("this pattern should not be found"), theEnd, nil, SearchDirectionBackward)
	assert.Assert(t, hit == nil)
}

// Converts a cell row to a plain string and removes trailing whitespace.
func rowToString(row []twin.StyledRune) string {
	rowString := ""
	for _, cell := range row {
		rowString += string(cell.Rune)
	}

	return strings.TrimRight(rowString, " ")
}

func benchmarkSearch(b *testing.B, highlighted bool) {
	log.SetLevel(log.WarnLevel) // Stop info logs from polluting benchmark output

	// Pick a go file so we get something with highlighting
	_, sourceFilename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Getting current filename failed")
	}

	sourceBytes, err := os.ReadFile(sourceFilename)
	assert.NilError(b, err)
	fileContents := string(sourceBytes)

	// Repeat input enough times to get to some target size, before highlighting
	// to get the same amount of text in either case
	replications := 5_000_000 / len(fileContents)

	// Read one copy of the example input
	if highlighted {
		highlightedSourceCode, err := reader.Highlight(fileContents, *styles.Get("native"), formatters.TTY16m, lexers.Get("go"))
		assert.NilError(b, err)
		if highlightedSourceCode == nil {
			panic("Highlighting didn't want to, returned nil")
		}
		fileContents = *highlightedSourceCode
	}

	// Create some input to search. Use a Builder to avoid quadratic string concatenation time.
	var builder strings.Builder
	builder.Grow(len(fileContents) * replications)
	for range replications {
		builder.WriteString(fileContents)
	}
	testString := builder.String()

	reader := reader.NewFromTextForTesting("hello", testString)
	assert.NilError(b, reader.Wait())

	// The [] around the 't' is there to make sure it doesn't match, remember
	// we're searching through this very file.
	pattern := regexp.MustCompile("This won'[t] match anything")

	// I hope forcing a GC here will make numbers more predictable
	runtime.GC()

	b.ResetTimer()

	for range b.N {
		// This test will search through all the N copies we made of our file
		hit := FindFirstHit(reader, *pattern, linemetadata.Index{}, nil, SearchDirectionForward)

		if hit != nil {
			panic(fmt.Errorf("This test is meant to scan the whole file without finding anything"))
		}
	}
}

// How long does it take to search a highlighted file for some regex?
//
// Run with: go test -run='^$' -bench=. . ./...
func BenchmarkHighlightedSearch(b *testing.B) {
	benchmarkSearch(b, true)
}

// How long does it take to search a plain text file for some regex?
//
// Search performance was a problem for me when I had a 600MB file to search in.
//
// Run with: go test -run='^$' -bench=. . ./...
func BenchmarkPlainTextSearch(b *testing.B) {
	benchmarkSearch(b, false)
}
