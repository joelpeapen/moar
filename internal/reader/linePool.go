package reader

// This value affects BenchmarkReadLargeFile() performance. Validate changes
// like this:
//
//	go test -benchmem -run='^$' -bench 'BenchmarkReadLargeFile' ./internal/reader
const linePoolSize = 1000

// Pre-allocator for Line objects. Allocating them in batches improves file
// reading performance.
type linePool struct {
	pool []Line
}

// Returns a Line aliasing raw. Note that the raw bytes are not copied, so raw
// must not be modified afterwards.
func (lp *linePool) create(raw []byte) *Line {
	if len(lp.pool) == 0 {
		lp.pool = make([]Line, linePoolSize)
	}

	line := &lp.pool[0]
	lp.pool = lp.pool[1:]

	// Clamp the capacity so appending to this line always reallocates rather
	// than writing into raw's backing array. Costs nothing, and keeps the
	// append path safe no matter how the caller manages its buffer.
	line.raw = raw[:len(raw):len(raw)]

	return line
}
