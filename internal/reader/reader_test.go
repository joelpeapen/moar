package reader

import (
	"math"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	log "github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	"github.com/walles/moor/v2/internal/linemetadata"
)

const samplesDir = "../../sample-files"

func init() {
	// Info logs clutter at least benchmark output
	log.SetLevel(log.WarnLevel)
}

func testGetLineCount(t *testing.T, reader *ReaderImpl) {
	if strings.Contains(*reader.DisplayName, "compressed") {
		// We are no good at counting lines of compressed files, never mind
		return
	}

	cmd := exec.Command("wc", "-l", *reader.FileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Error("Error calling wc -l to count lines of", *reader.FileName, err)
	}

	wcNumberString := strings.Split(strings.TrimSpace(string(output)), " ")[0]
	wcLineCount, err := strconv.Atoi(wcNumberString)
	if err != nil {
		t.Error("Error counting lines of", *reader.FileName, err)
	}

	// wc -l under-counts by 1 if the file doesn't end in a newline
	rawBytes, err := os.ReadFile(*reader.FileName)
	if err == nil && len(rawBytes) > 0 && rawBytes[len(rawBytes)-1] != '\n' {
		wcLineCount++
	}

	if reader.GetLineCount() != wcLineCount {
		t.Errorf("Got %d lines from the reader but %d lines from wc -l: <%s>",
			reader.GetLineCount(), wcLineCount, *reader.FileName)
	}

	countLinesCount, err := countLines(*reader.FileName)
	assert.NilError(t, err)
	if countLinesCount != uint64(wcLineCount) {
		t.Errorf("Got %d lines from wc -l, but %d lines from our countLines() function", wcLineCount, countLinesCount)
	}
}

func firstLine(inputLines InputLines) linemetadata.Index {
	return inputLines.Lines[0].Index
}

func testGetLines(t *testing.T, reader *ReaderImpl) {
	lines := reader.GetLines(linemetadata.Index{}, 10)
	if len(lines.Lines) > 10 {
		t.Errorf("Asked for 10 lines, got too many: %d", len(lines.Lines))
	}

	if len(lines.Lines) < 10 {
		// No good plan for how to test short files, more than just
		// querying them, which we just did
		return
	}

	// Test clipping at the end
	lines = reader.GetLines(linemetadata.IndexMax(), 10)
	if len(lines.Lines) != 10 {
		t.Errorf("Asked for 10 lines but got %d", len(lines.Lines))
		return
	}

	startOfLastSection := firstLine(lines)
	lines = reader.GetLines(startOfLastSection, 10)
	if firstLine(lines) != startOfLastSection {
		t.Errorf("Expected start line %d when asking for the last 10 lines, got %s",
			startOfLastSection, firstLine(lines).Format())
		return
	}
	if len(lines.Lines) != 10 {
		t.Errorf("Expected 10 lines when asking for the last 10 lines, got %d",
			len(lines.Lines))
		return
	}

	lines = reader.GetLines(startOfLastSection.NonWrappingAdd(1), 10)
	if firstLine(lines) != startOfLastSection {
		t.Errorf("Expected start line %d when asking for the last+1 10 lines, got %s",
			startOfLastSection, firstLine(lines).Format())
		return
	}
	if len(lines.Lines) != 10 {
		t.Errorf("Expected 10 lines when asking for the last+1 10 lines, got %d",
			len(lines.Lines))
		return
	}

	lines = reader.GetLines(startOfLastSection.NonWrappingAdd(-1), 10)
	if firstLine(lines) != startOfLastSection.NonWrappingAdd(-1) {
		t.Errorf("Expected start line %d when asking for the last-1 10 lines, got %s",
			startOfLastSection, firstLine(lines).Format())
		return
	}
	if len(lines.Lines) != 10 {
		t.Errorf("Expected 10 lines when asking for the last-1 10 lines, got %d",
			len(lines.Lines))
		return
	}
}

func getTestFiles(t *testing.T) []string {
	files, err := os.ReadDir(samplesDir)
	assert.NilError(t, err)

	var filenames []string
	for _, file := range files {
		filenames = append(filenames, path.Join(samplesDir, file.Name()))
	}

	return filenames
}

func TestGetLines(t *testing.T) {
	for _, file := range getTestFiles(t) {
		t.Run(file, func(t *testing.T) {
			reader, err := NewFromFilename(file, formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
			if err != nil {
				t.Errorf("Error opening file <%s>: %s", file, err.Error())
				return
			}
			if err := reader.Wait(); err != nil {
				t.Errorf("Error reading file <%s>: %s", file, err.Error())
				return
			}

			t.Run(file, func(t *testing.T) {
				testGetLines(t, reader)
				testGetLineCount(t, reader)
				testHighlightingLineCount(t, file)
			})
		})
	}
}

func testHighlightingLineCount(t *testing.T, filenameWithPath string) {
	// This won't work on compressed files
	if strings.HasSuffix(filenameWithPath, ".xz") {
		return
	}
	if strings.HasSuffix(filenameWithPath, ".bz2") {
		return
	}
	if strings.HasSuffix(filenameWithPath, ".gz") {
		return
	}
	if strings.HasSuffix(filenameWithPath, ".zst") {
		return
	}
	if strings.HasSuffix(filenameWithPath, ".zstd") {
		return
	}

	// Load the unformatted file
	rawBytes, err := os.ReadFile(filenameWithPath)
	assert.NilError(t, err)
	rawContents := string(rawBytes)

	// Count its lines
	rawLinefeedsCount := strings.Count(rawContents, "\n")
	rawRunes := []rune(rawContents)
	rawFileEndsWithNewline := true // Special case empty files
	if len(rawRunes) > 0 {
		rawFileEndsWithNewline = rawRunes[len(rawRunes)-1] == '\n'
	}
	rawLinesCount := rawLinefeedsCount
	if !rawFileEndsWithNewline {
		rawLinesCount++
	}

	// Then load the same file using one of our Readers
	reader, err := NewFromFilename(filenameWithPath, formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)
	err = reader.Wait()
	assert.NilError(t, err)

	highlightedLinesCount := reader.GetLineCount()
	assert.Equal(t, rawLinesCount, highlightedLinesCount)
}

func TestGetLongLine(t *testing.T) {
	file := samplesDir + "/very-long-line.txt"
	reader, err := NewFromFilename(file, formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)
	assert.NilError(t, reader.Wait())

	lines := reader.GetLines(linemetadata.Index{}, 5)
	assert.Equal(t, firstLine(lines), linemetadata.Index{})
	assert.Equal(t, len(lines.Lines), 1)

	line := lines.Lines[0]
	assert.Assert(t, strings.HasPrefix(line.Plain(), "1 2 3 4"), "<%s>", line.Plain())
	assert.Assert(t, strings.HasSuffix(line.Plain(), "0123456789"), line.Plain())

	assert.Equal(t, len(line.Plain()), 100021)
}

func getReaderWithLineCount(totalLines int) *ReaderImpl {
	return NewFromTextForTesting("", strings.Repeat("x\n", totalLines))
}

func testStatusText(t *testing.T, fromLine linemetadata.Index, toLine linemetadata.Index, totalLines int, expected string) {
	testMe := getReaderWithLineCount(totalLines)
	linesRequested := fromLine.CountLinesTo(toLine)
	lines := testMe.GetLines(fromLine, linesRequested)
	statusText := lines.StatusText
	assert.Equal(t, statusText, expected)
}

func TestStatusText(t *testing.T) {
	testStatusText(t, linemetadata.Index{}, linemetadata.IndexFromOneBased(10), 20, "20 lines  50%")
	testStatusText(t, linemetadata.Index{}, linemetadata.IndexFromOneBased(5), 5, "5 lines  100%")
	testStatusText(t,
		linemetadata.IndexFromOneBased(998),
		linemetadata.IndexFromOneBased(999),
		1000,
		"1000 lines  99%")

	testStatusText(t, linemetadata.Index{}, linemetadata.Index{}, 0, "<empty>")
	testStatusText(t, linemetadata.Index{}, linemetadata.Index{}, 1, "1 line  100%")

	// Test with filename
	testMe, err := NewFromFilename(samplesDir+"/empty", formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)
	assert.NilError(t, testMe.Wait())

	line := testMe.GetLines(linemetadata.Index{}, 0)
	if line.Lines != nil {
		t.Error("line.lines is should have been nil when reading from an empty stream")
	}
	assert.Equal(t, line.FilenameText, "empty")
	assert.Equal(t, line.StatusText, ": <empty>")
}

func TestClipRangeToLength(t *testing.T) {
	// Within bounds
	i0, i1 := clipRangeToLength(linemetadata.Index{}, 1, 20)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 0)

	// Touching the end, still within bounds
	i0, i1 = clipRangeToLength(linemetadata.Index{}, 1, 0)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 0)

	// Overflow, push down to indices 6, 7, 8
	i0, i1 = clipRangeToLength(linemetadata.IndexFromOneBased(100), 3, 8)
	assert.Equal(t, i0, 6)
	assert.Equal(t, i1, 8)

	// Overflow, push down and clip to indices 0, 1
	i0, i1 = clipRangeToLength(linemetadata.IndexFromOneBased(100), 3, 1)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 1)

	// Maxed out start
	i0, i1 = clipRangeToLength(linemetadata.IndexMax(), 1, 0)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 0)

	// Maxed out count
	i0, i1 = clipRangeToLength(linemetadata.Index{}, math.MaxInt, 0)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 0)

	// Maxed out start and count
	i0, i1 = clipRangeToLength(linemetadata.IndexMax(), math.MaxInt, 3)
	assert.Equal(t, i0, 0)
	assert.Equal(t, i1, 3)
}

// How long does it take to read a file?
//
// This can be slow due to highlighting.
//
// Run with: go test -run='^$' -bench=. . ./...
func BenchmarkReaderDone(b *testing.B) {
	filename := "reader.go" // This is our longest .go file
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// This is our longest .go file
		readMe, err := NewFromFilename(filename, formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
		assert.NilError(b, err)

		assert.NilError(b, readMe.Wait())
		assert.NilError(b, readMe.Err)
	}
}

// Try loading a large file
func BenchmarkReadLargeFile(b *testing.B) {
	// Try loading a file this large
	const largeSizeBytes = 35_000_000

	// First, create it from something...
	inputFilename := "reader.go"
	contents, err := os.ReadFile(inputFilename)
	assert.NilError(b, err)

	testdir := b.TempDir()
	largeFileName := testdir + "/large-file"
	largeFile, err := os.Create(largeFileName)
	assert.NilError(b, err)

	totalBytesWritten := 0
	for totalBytesWritten < largeSizeBytes {
		written, err := largeFile.Write(contents)
		assert.NilError(b, err)

		totalBytesWritten += written
	}
	err = largeFile.Close()
	assert.NilError(b, err)

	// Make sure we don't pause during the benchmark
	targetLineCount := largeSizeBytes * 2

	b.SetBytes(int64(totalBytesWritten))

	// Try making the whole run more predictable
	runtime.GC()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		readMe, err := NewFromFilename(
			largeFileName,
			formatters.TTY16m,
			ReaderOptions{
				Style:           styles.Get("native"),
				PauseAfterLines: &targetLineCount,
			})
		assert.NilError(b, err)

		<-readMe.MaybeDone
		assert.NilError(b, readMe.Wait())
		assert.NilError(b, readMe.Err)
	}
}

// Try loading a file with a long line
func BenchmarkReadLongLine(b *testing.B) {
	// Try loading a line this long
	const longLineBytes = 3_000_000

	// First, create it from something...
	lineBytes := []byte(strings.Repeat("x", longLineBytes-1) + "\n")
	testdir := b.TempDir()
	largeFileName := testdir + "/long-line-file.txt"
	largeFile, err := os.Create(largeFileName)
	assert.NilError(b, err)

	totalBytesWritten := 0
	for totalBytesWritten < longLineBytes {
		written, err := largeFile.Write(lineBytes)
		assert.NilError(b, err)

		totalBytesWritten += written
	}
	err = largeFile.Close()
	assert.NilError(b, err)

	// Make sure we don't pause during the benchmark
	targetLineCount := longLineBytes * 2

	b.SetBytes(int64(totalBytesWritten))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		b.StopTimer()
		runtime.GC()
		b.StartTimer()

		readMe, err := NewFromFilename(
			largeFileName,
			formatters.TTY16m,
			ReaderOptions{
				Style:           styles.Get("native"),
				PauseAfterLines: &targetLineCount,
			})
		assert.NilError(b, err)

		<-readMe.MaybeDone
		assert.NilError(b, readMe.Wait())
		assert.NilError(b, readMe.Err)
	}
}

// Count lines in pager.go
func BenchmarkCountLines(b *testing.B) {
	// First, get some sample lines...
	inputFilename := "reader.go"
	contents, err := os.ReadFile(inputFilename)
	assert.NilError(b, err)

	testdir := b.TempDir()
	countFileName := testdir + "/count-file"
	countFile, err := os.Create(countFileName)
	assert.NilError(b, err)

	// Make a large enough test case that a majority of the time is spent
	// counting lines, rather than on any counting startup cost.
	//
	// We used to have 1000 here, but that made the benchmark result fluctuate
	// too much. 10_000 seems to provide stable enough results.
	for range 10_000 {
		_, err := countFile.Write(contents)
		assert.NilError(b, err)
	}
	err = countFile.Close()
	assert.NilError(b, err)

	b.ResetTimer()
	for range b.N {
		_, err = countLines(countFileName)
		assert.NilError(b, err)
	}
}
