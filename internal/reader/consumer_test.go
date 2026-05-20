package reader

import (
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moor/v2/internal/linemetadata"
	"gotest.tools/v3/assert"
)

func testCompressedFile(t *testing.T, filename string) {
	filenameWithPath := path.Join(samplesDir, filename)
	reader, e := NewFromFilename(filenameWithPath, formatters.TTY16m, ReaderOptions{Style: styles.Get("native")})
	if e != nil {
		t.Errorf("Error opening file <%s>: %s", filenameWithPath, e.Error())
		panic(e)
	}
	assert.NilError(t, reader.Wait())

	lines := reader.GetLines(linemetadata.Index{}, 5)
	assert.Equal(t, lines.Lines[0].Plain(), "This is a compressed file", "%s", filename)
}

func TestCompressedFiles(t *testing.T) {
	testCompressedFile(t, "compressed.txt.gz")
	testCompressedFile(t, "compressed.txt.bz2")
	testCompressedFile(t, "compressed.txt.xz")
	testCompressedFile(t, "compressed.txt.zst")
	testCompressedFile(t, "compressed.txt.zstd")
}

func TestReadFileDoneNoHighlighting(t *testing.T) {
	testMe, err := NewFromFilename(samplesDir+"/empty",
		formatters.TTY, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())
}

func TestReadFileDoneYesHighlighting(t *testing.T) {
	testMe, err := NewFromFilename("reader_test.go",
		formatters.TTY, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())
}

func TestReadStreamDoneNoHighlighting(t *testing.T) {
	testMe, err := NewFromStream("", strings.NewReader("Johan"), nil, ReaderOptions{Style: &chroma.Style{}})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())
}

func TestReadStreamDoneYesHighlighting(t *testing.T) {
	testMe, err := NewFromStream("",
		strings.NewReader("Johan"),
		formatters.TTY, ReaderOptions{Lexer: lexers.EmacsLisp, Style: styles.Get("native")})
	assert.NilError(t, err)

	assert.NilError(t, testMe.Wait())
}

func TestReadFromDevFd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/fd is not available on Windows")

		return
	}

	readPipe, writePipe, err := os.Pipe()
	assert.NilError(t, err)
	defer readPipe.Close() //nolint:errcheck

	_, err = writePipe.WriteString("test\n")
	assert.NilError(t, err)
	assert.NilError(t, writePipe.Close())

	fileName := "/dev/fd/" + strconv.Itoa(int(readPipe.Fd()))
	testMe, err := NewFromFilename(fileName, formatters.TTY, ReaderOptions{Style: styles.Get("native")})
	assert.NilError(t, err)
	assert.NilError(t, testMe.Wait())

	lines := testMe.GetLines(linemetadata.Index{}, 10)
	assert.Equal(t, len(lines.Lines), 1)
	assert.Equal(t, lines.Lines[0].Plain(), "test")
}
