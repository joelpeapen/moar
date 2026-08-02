package reader

import (
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/alecthomas/chroma/v2"
	"gotest.tools/v3/assert"
)

func TestTryOpenDirectory(t *testing.T) {
	tempDir := t.TempDir()

	err := TryOpen(tempDir)
	assert.Assert(t, err != nil, "TryOpen should fail on directories")
}

func TestReadTextDone(t *testing.T) {
	testMe := NewFromTextForTesting("", "Johan")

	assert.NilError(t, testMe.Wait())
}

// NewFromStream takes ownership of the stream, so once it has consumed the
// initial contents it closes it. Reading from the pipe afterwards should
// therefore report it closed rather than just empty.
//
// Compressed input gets handed to a decompressor rather than read directly, one
// decompressor per format, so ownership has to survive each of those.
func TestStreamClosedAfterReading(t *testing.T) {
	streams := map[string][]byte{
		"uncompressed": []byte("Johan\n"),
		"empty":        nil,
	}
	for _, sampleName := range []string{
		"compressed.txt.gz",
		"compressed.txt.bz2",
		"compressed.txt.xz",
		"compressed.txt.zst",
	} {
		fileContents, err := os.ReadFile(path.Join(samplesDir, sampleName))
		assert.NilError(t, err)
		streams[sampleName] = fileContents
	}

	for name, streamContents := range streams {
		t.Run(name, func(t *testing.T) {
			pipeReader, pipeWriter := io.Pipe()

			go func() {
				if len(streamContents) == 0 {
					// Writing nothing would still show up as a zero length read,
					// and we want an immediate end of stream
					pipeWriter.Close() //nolint:errcheck
					return
				}

				_, err := pipeWriter.Write(streamContents)

				// With nil this is just a Close(), signalling end of stream
				pipeWriter.CloseWithError(err) //nolint:errcheck
			}()

			testMe, err := NewFromStream("", pipeReader, nil, ReaderOptions{Style: &chroma.Style{}})
			assert.NilError(t, err)
			assert.NilError(t, testMe.Wait())

			_, err = pipeReader.Read(make([]byte, 1))
			assert.Equal(t, err, io.ErrClosedPipe)
		})
	}
}

// Closing the reader closes the stream, which is what tells a producer still
// writing into it that nobody is listening any more.
func TestStreamClosedOnReaderClose(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()

	// The pipe is unbuffered and NewFromStream reads from the stream before
	// returning, so this write has to happen concurrently with that call.
	firstWrite := make(chan error, 1)
	go func() {
		_, err := pipeWriter.Write([]byte("Johan\n"))
		firstWrite <- err
	}()

	testMe, err := NewFromStream("", pipeReader, nil, ReaderOptions{Style: &chroma.Style{}})
	assert.NilError(t, err)

	// Once this write has landed the reader is waiting for more, with the
	// stream still open.
	assert.NilError(t, <-firstWrite)

	testMe.Close()

	secondWrite := make(chan error, 1)
	go func() {
		_, err := pipeWriter.Write([]byte("Nobody home\n"))
		secondWrite <- err
	}()

	select {
	case err := <-secondWrite:
		assert.Equal(t, err, io.ErrClosedPipe)
	case <-time.After(time.Second):
		t.Fatal("Writing to a closed reader's stream should fail, it blocked instead")
	}
}
