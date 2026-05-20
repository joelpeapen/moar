package internal

import (
	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/reader"
)

// For small searches or few cores, search will be fast no matter what we put
// here. For large searches on many-core systems, a larger cache will help
// performance. But a larger cache without a performance increase has no value.
// To evaluate:
//
//  go test -run='^$' -bench 'Warm' ./internal
//
// Results from Johan's laptop. The numbers are the test iteration counts for
// BenchmarkHighlightedWarmSearch and BenchmarkPlainTextWarmSearch. The
// optimization has been done to improve the sum of these two benchmarks.
//
//    100: 741+726=1467
//    500: 832+882=1714 <-- Best
//   1000: 754+810=1564
//   5000: 637+658=1295

const searchLineCacheSize = 500

type searchLineCache struct {
	lines []reader.NumberedLine
}

func (c *searchLineCache) GetLine(reader reader.Reader, index linemetadata.Index, direction SearchDirection) *reader.NumberedLine {
	// Do we have a cache hit?
	cacheHit := c.getLineFromCache(index)
	if cacheHit != nil {
		return cacheHit
	}

	// Cache miss, load new lines
	firstIndexToRequest := index
	if direction == SearchDirectionBackward {
		// Let's say we want index 10 to be in the cache. Cache size is 5.
		// Then, the first index must be 6, so that we get 6,7,8,9,10.
		// Or in other words, 10 - 5 + 1 = 6.
		firstIndexToRequest = index.NonWrappingAdd(-searchLineCacheSize + 1)
	}

	reader.GetLinesPreallocated(firstIndexToRequest, &c.lines)

	// Get the line from the cache
	return c.getLineFromCache(index)
}

// Or nil if that line isn't in the cache
func (c *searchLineCache) getLineFromCache(index linemetadata.Index) *reader.NumberedLine {
	if cap(c.lines) != searchLineCacheSize {
		// Initialize cache
		c.lines = make([]reader.NumberedLine, 0, searchLineCacheSize)
	}

	if len(c.lines) == 0 {
		return nil
	}

	firstCachedIndexInclusive := c.lines[0].Index
	if index.IsBefore(firstCachedIndexInclusive) {
		return nil
	}

	lastCachedIndexInclusive := c.lines[len(c.lines)-1].Index
	if index.IsAfter(lastCachedIndexInclusive) {
		return nil
	}

	return &c.lines[index.Index()-firstCachedIndexInclusive.Index()]
}
