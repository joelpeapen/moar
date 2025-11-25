// Public API for embedding a pager in your application
package moor

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	log "github.com/sirupsen/logrus"
	"github.com/walles/moor/v2/internal"
	internalReader "github.com/walles/moor/v2/internal/reader"
	"github.com/walles/moor/v2/twin"
	"golang.org/x/term"
)

const logLevel = log.WarnLevel

// If you feel some option is missing, make PRs at
// https://github.com/walles/moor/pulls open an issue.
type Options struct {
	// Name displayed in the bottom left corner of the pager.
	//
	// Defaults to the file name when paging files, otherwise nothing. Leave
	// blank for default.
	Title string

	// The default is to auto format JSON input. Set this to true to disable
	// auto formatting.
	NoAutoFormat bool

	// The default is to truncate long lines, and let the user press right-arrow
	// to see more of them. Set this to true to wrap long lines instead. Users
	// can toggle wrapping on / off using the 'w' key while paging.
	WrapLongLines bool

	// The default is to show line numbers. Set this to true to disable line
	// numbers. The user can toggle line numbers on by pressing the left-arrow
	// key while paging.
	NoLineNumbers bool

	// The default is to always start the pager. If this is set to true, short
	// input will just be printed, and no paging will happen.
	QuitIfOneScreen bool
}

// If stdout is not a terminal, the stream contents will just be printed to
// stdout.
func PageFromStream(reader io.Reader, options Options) error {
	logs := startLogCollection()
	defer collectLogs(logs)

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return dumpToStdoutAndClose(reader)
	}

	pagerReader, err := internalReader.NewFromStream(
		options.Title,
		reader,
		getColorFormatter(),
		internalReader.ReaderOptions{
			ShouldFormat: !options.NoAutoFormat,
		})
	if err != nil {
		return err
	}

	return pageFromReader(pagerReader, options)
}

// If stdout is not a terminal, the file contents will just be printed to
// stdout.
func PageFromFile(name string, options Options) error {
	logs := startLogCollection()
	defer collectLogs(logs)

	if !term.IsTerminal(int(os.Stdout.Fd())) {
		stream, err := os.Open(name)
		if err != nil {
			return err
		}
		return dumpToStdoutAndClose(stream)
	}

	pagerReader, err := internalReader.NewFromFilename(
		name,
		getColorFormatter(),
		internalReader.ReaderOptions{
			ShouldFormat: !options.NoAutoFormat,
		})
	if err != nil {
		return err
	}

	if options.Title != "" {
		pagerReader.DisplayName = &options.Title
	}

	return pageFromReader(pagerReader, options)
}

// If stdout is not a terminal, the string contents will just be printed to
// stdout.
func PageFromString(text string, options Options) error {
	// NOTE: Pager froze when I tried to use internalReader.NewFromText() here.
	// If you want to try that again, make sure to test it using some external
	// test program!
	return PageFromStream(strings.NewReader(text), options)
}

func startLogCollection() *internal.LogWriter {
	log.SetLevel(logLevel)

	var logLines internal.LogWriter
	log.SetOutput(&logLines)
	return &logLines
}

func collectLogs(logs *internal.LogWriter) {
	if len(logs.String()) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, logs.String())
}

func dumpToStdoutAndClose(reader io.Reader) error {
	_, err := io.Copy(os.Stdout, reader)
	if err != nil {
		return err
	}

	// Close the reader if we can
	if closer, ok := reader.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func getColorFormatter() chroma.Formatter {
	if os.Getenv("COLORTERM") != "truecolor" && strings.Contains(os.Getenv("TERM"), "256") {
		// Covers "xterm-256color" as used by the macOS Terminal
		return formatters.TTY256
	}
	return formatters.TTY16m
}

func pageFromReader(reader *internalReader.ReaderImpl, options Options) error {
	pager := internal.NewPager(reader)
	pager.WrapLongLines = options.WrapLongLines
	pager.ShowLineNumbers = !options.NoLineNumbers
	pager.QuitIfOneScreen = options.QuitIfOneScreen

	screen, e := twin.NewScreen()
	if e != nil {
		// Screen setup failed
		return e
	}

	style := internal.GetStyleForScreen(screen)
	reader.SetStyleForHighlighting(style)

	formatter := getColorFormatter()

	pager.StartPaging(screen, &style, &formatter)
	screen.Close()

	if !pager.DeInit {
		pager.ReprintAfterExit()
	}

	return nil
}
