package main

import (
	"testing"

	"github.com/walles/moor/v2/internal/linemetadata"
	"github.com/walles/moor/v2/internal/textstyles"
	"github.com/walles/moor/v2/twin"
	"gotest.tools/v3/assert"
)

func TestParseScrollHint(t *testing.T) {
	token, err := parseScrollHint("ESC[7m>")
	assert.NilError(t, err)
	assert.Equal(t, token, textstyles.CellWithMetadata{
		Rune:  '>',
		Style: twin.StyleDefault.WithAttr(twin.AttrReverse),
	})
}

func TestPageOneInputFile(t *testing.T) {
	pager, screen, _, formatter, _, err := pagerFromArgs(
		[]string{"", "moor_test.go"},
		func(_ twin.MouseMode, _ twin.ColorCount) (twin.Screen, error) {
			return twin.NewFakeScreen(80, 24), nil
		},
		false, // stdin is redirected
		false, // stdout is redirected
	)

	assert.NilError(t, err)
	assert.Assert(t, pager != nil)
	assert.Assert(t, screen != nil)
	assert.Assert(t, formatter != nil)
}

func TestGetTargetLine(t *testing.T) {
	index, remaining := getTargetLine([]string{})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{})

	index, remaining = getTargetLine([]string{"+"})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{"+"})

	// Ref: https://github.com/walles/moor/issues/316
	index, remaining = getTargetLine([]string{"+0"})
	assert.Assert(t, index == nil)
	assert.DeepEqual(t, remaining, []string{})

	index, remaining = getTargetLine([]string{"+1"})
	assert.Equal(t, *index, linemetadata.IndexFromOneBased(1))
	assert.DeepEqual(t, remaining, []string{})
}
