package reader

// This value affects BenchmarkReadLargeFile() performance. Validate changes
// like this:
//
//	go test -benchmem -run='^$' -bench 'BenchmarkReadLargeFile' ./internal/reader
const linePoolSize = 1000

type linePool struct {
	pool []Line
}

func (lp *linePool) create(raw []byte) *Line {
	if len(lp.pool) == 0 {
		lp.pool = make([]Line, linePoolSize)
	}

	line := &lp.pool[0]
	lp.pool = lp.pool[1:]

	line.raw = raw
	return line
}
