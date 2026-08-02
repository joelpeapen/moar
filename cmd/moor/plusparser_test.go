package main

import (
	"testing"

	"github.com/walles/moor/v2/internal/linemetadata"
	"gotest.tools/v3/assert"
)

func TestParsePlusArgs_base(t *testing.T) {
	index, _, remaining := parsePlusArgs([]string{})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{})

	index, _, remaining = parsePlusArgs([]string{"+"})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{"+"})
}

func TestParsePlusArgs_targetLine(t *testing.T) {
	// Ref: https://github.com/walles/moor/issues/316
	index, _, remaining := parsePlusArgs([]string{"+0"})
	assert.Equal(t, *index, linemetadata.IndexFromOneBased(1))
	assert.DeepEqual(t, remaining, []string{})

	index, _, remaining = parsePlusArgs([]string{"+1"})
	assert.Equal(t, *index, linemetadata.IndexFromOneBased(1))
	assert.DeepEqual(t, remaining, []string{})
}

func TestParsePlusArgs_initialSearch(t *testing.T) {
	_, pattern, remaining := parsePlusArgs([]string{"+/"})
	assert.Equal(t, *pattern, "")
	assert.DeepEqual(t, remaining, []string{})

	_, pattern, remaining = parsePlusArgs([]string{"+/hej"})
	assert.Equal(t, *pattern, "hej")
	assert.DeepEqual(t, remaining, []string{})
}
