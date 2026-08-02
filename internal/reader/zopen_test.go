package reader

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"gotest.tools/v3/assert"
)

// Test that ZReader works with an empty stream
func TestZReaderEmpty(t *testing.T) {
	bytesReader := bytes.NewReader([]byte{})

	zReader, err := ZReader(bytesReader)
	assert.NilError(t, err)

	all, err := io.ReadAll(zReader)
	assert.NilError(t, err)

	assert.Equal(t, 0, len(all))
}

// Test that ZReader works with a one-byte stream
func TestZReaderOneByte(t *testing.T) {
	bytesReader := bytes.NewReader([]byte{42})

	zReader, err := ZReader(bytesReader)
	assert.NilError(t, err)

	all, err := io.ReadAll(zReader)
	assert.NilError(t, err)

	assert.Equal(t, 1, len(all))
	assert.Equal(t, byte(42), all[0])
}

// How many zstd decompressors are running right now?
//
// The zstd decompressor does its work in goroutines, so this is how we tell
// whether one is still alive.
func zstdDecompressorCount() int {
	bufferSize := 64 * 1024
	for {
		stacks := make([]byte, bufferSize)
		length := runtime.Stack(stacks, true)
		if length == bufferSize {
			// Truncated, try again with more room
			bufferSize *= 2
			continue
		}

		return bytes.Count(stacks[:length], []byte("zstd.(*Decoder).startStreamDecoder"))
	}
}

// A zstd stream too large for its decompressor to finish in one go, so that
// closing it early has something left to release.
//
// Measured, the decompressor keeps running from somewhere between 100kB and
// 400kB of contents, more on machines with more cores. This is far above that,
// and if it ever stops being enough then
// TestZstdDecompressorReleasedOnClose() says so rather than quietly passing.
func largeZstdStream(t *testing.T) []byte {
	const uncompressedSize = 12_000_000
	chunk := []byte(strings.Repeat("Some filler text to compress.\n", 1000))

	compressed := bytes.Buffer{}
	writer, err := zstd.NewWriter(&compressed)
	assert.NilError(t, err)

	written := 0
	for written < uncompressedSize {
		count, err := writer.Write(chunk)
		assert.NilError(t, err)
		written += count
	}
	assert.NilError(t, writer.Close())

	return compressed.Bytes()
}

// ZReader hands out a decompressor with goroutines of its own, and only the
// decompressor itself can stop those. Closing the stream has to get that done,
// otherwise every zstd stream we stop reading early leaks them for the lifetime
// of the process.
func TestZstdDecompressorReleasedOnClose(t *testing.T) {
	if runtime.GOMAXPROCS(0) == 1 {
		t.Skip("Single core zstd decompresses inline, leaving no goroutines to release")

		return
	}

	// io.MultiReader hides that this is a bytes.Reader, which zstd would
	// otherwise decompress synchronously without any goroutines. ZReader wraps
	// its input in an io.MultiReader in production as well.
	input := io.MultiReader(bytes.NewReader(largeZstdStream(t)))

	zReader, err := ZReader(input)
	assert.NilError(t, err)

	// Read a little and walk away, the way the pager does when the user quits
	// early. Reading all the way to the end would let the decompressor finish
	// on its own.
	_, err = io.CopyN(io.Discard, zReader, 100)
	assert.NilError(t, err)
	assert.Assert(t, zstdDecompressorCount() > 0,
		"Test setup problem, try raising uncompressedSize in largeZstdStream()")

	closer, isCloser := zReader.(io.Closer)
	assert.Assert(t, isCloser, "Decompressed zstd streams should be closable")
	assert.NilError(t, closer.Close())

	// Closing tells the decompressor to stop, stopping takes it a moment
	for range 100 {
		if zstdDecompressorCount() == 0 {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("The zstd decompressor kept running after its stream was closed")
}

// Decompressing a file takes ownership of it, so closing the decompressed
// reader closes the file too. Otherwise every compressed file we page leaks a
// file descriptor.
//
// One decompressor per format, so ownership has to survive each of those.
func TestCompressedFileClosedOnClose(t *testing.T) {
	for _, sampleName := range []string{
		"compressed.txt.gz",
		"compressed.txt.bz2",
		"compressed.txt.xz",
		"compressed.txt.zst",
	} {
		t.Run(sampleName, func(t *testing.T) {
			sampleFile := path.Join(samplesDir, sampleName)
			file, err := os.Open(sampleFile)
			assert.NilError(t, err)

			stream, _, err := zOpenFile(file, sampleFile)
			assert.NilError(t, err)

			_, err = io.Copy(io.Discard, stream)
			assert.NilError(t, err)
			assert.NilError(t, stream.Close())

			// Reading a closed file reports it closed rather than returning data
			_, err = file.Read(make([]byte, 1))
			assert.Assert(t, errors.Is(err, os.ErrClosed),
				"Want the file closed, got %v when reading it", err)
		})
	}
}
