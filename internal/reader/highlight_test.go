package reader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moor/v2/internal/linemetadata"
	"gotest.tools/v3/assert"
)

func TestFormatJson(t *testing.T) {
	// Note the space after "key" to verify formatting actually happens
	jsonStream := strings.NewReader(`{"key" :"value"}`)
	testMe, err := NewFromStream(
		"JSON test",
		jsonStream,
		formatters.TTY,
		ReaderOptions{
			Style:        styles.Get("native"),
			ShouldFormat: true,
		})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())

	lines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, lines.Lines[0].Plain(), "{")
	assert.Equal(t, lines.Lines[1].Plain(), `  "key": "value"`)
	assert.Equal(t, lines.Lines[2].Plain(), "}")
	assert.Equal(t, len(lines.Lines), 3)
}

func TestFormatJsonArray(t *testing.T) {
	// Note the space after "key" to verify formatting actually happens
	jsonStream := strings.NewReader(`[{"key" :"value"}]`)
	testMe, err := NewFromStream(
		"JSON test",
		jsonStream,
		formatters.TTY,
		ReaderOptions{
			Style:        styles.Get("native"),
			ShouldFormat: true,
		})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())

	lines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, lines.Lines[0].Plain(), "[")
	assert.Equal(t, lines.Lines[1].Plain(), "  {")
	assert.Equal(t, lines.Lines[2].Plain(), `    "key": "value"`)
	assert.Equal(t, lines.Lines[3].Plain(), "  }")
	assert.Equal(t, lines.Lines[4].Plain(), "]")
	assert.Equal(t, len(lines.Lines), 5)
}

// Highlighting that gives every visible character the same styling conveys
// nothing, so Highlight() declines to do it.
func TestHighlightUniformIsNoop(t *testing.T) {
	for _, text := range []string{
		"1\n",
		" 1 \n",
		"\t1\n",
		`"text"` + "\n",
	} {
		t.Run(text, func(t *testing.T) {
			highlighted, err := Highlight(text, *styles.Get("native"), formatters.TTY, lexers.Get("json"))
			assert.NilError(t, err)
			if highlighted != nil {
				t.Errorf("Expected no highlighting, got %q", *highlighted)
			}
		})
	}
}

// A JSON object gets punctuation, key names and values in different colors, so
// highlighting it is worthwhile.
func TestHighlightJsonObject(t *testing.T) {
	highlighted, err := Highlight(`{"key": "value"}`, *styles.Get("native"), formatters.TTY, lexers.Get("json"))
	assert.NilError(t, err)
	assert.Assert(t, highlighted != nil)
}

// Whitespace has no visible foreground, so coloring it differently from the
// visible text doesn't make highlighting worthwhile.
//
// The json lexer reports whitespace as Text and the Go one as TextWhitespace,
// so whitespace must be recognized by its contents rather than its token type.
func TestHighlightWhitespaceForegroundIsNoop(t *testing.T) {
	style := whiteOnBlackWithWhitespace("#ff0000 bg:#000000")

	for _, lexerName := range []string{"json", "Go"} {
		t.Run(lexerName, func(t *testing.T) {
			highlighted, err := Highlight(" 1 \n", *style, formatters.TTY, lexers.Get(lexerName))
			assert.NilError(t, err)
			if highlighted != nil {
				t.Errorf("Expected no highlighting, got %q", *highlighted)
			}
		})
	}
}

// A whitespace background color is visible, so it counts.
func TestHighlightWhitespaceBackgroundIsNotNoop(t *testing.T) {
	style := whiteOnBlackWithWhitespace("#ffffff bg:#ff0000")

	highlighted, err := Highlight(" 1 \n", *style, formatters.TTY, lexers.Get("json"))
	assert.NilError(t, err)
	assert.Assert(t, highlighted != nil)
}

// A style rendering all visible tokens as white on black, with whitespace
// styled as requested.
func whiteOnBlackWithWhitespace(whitespace string) *chroma.Style {
	return chroma.MustNewStyle("test", chroma.StyleEntries{
		chroma.Background:           "#ffffff bg:#000000",
		chroma.LiteralNumberInteger: "#ffffff bg:#000000",
		chroma.Text:                 whitespace,
	})
}

func TestIsJsonOrJsonl(t *testing.T) {
	// Standard JSON
	assert.Assert(t, isJsonOrJsonl(`{"hello": "world"}`))

	// JSONL sample file
	jsonlPath := filepath.Join("..", "..", "sample-files", "jsonl.jsonl")
	jsonlBytes, err := os.ReadFile(jsonlPath)
	assert.NilError(t, err)

	assert.Assert(t, isJsonOrJsonl(string(jsonlBytes)))
}
