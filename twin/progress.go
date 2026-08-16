package twin

import (
	"fmt"
	"strconv"
)

type ProgressState int

// Numbers here match the ones from
// https://rockorager.dev/misc/osc-9-4-progress-bars/.
const (
	ProgressStateRemove        ProgressState = 0
	ProgressStateSet           ProgressState = 1
	ProgressStateError         ProgressState = 2
	ProgressStateIndeterminate ProgressState = 3
	ProgressStatePause         ProgressState = 4
)

// Tell the terminal to remove the progress bar.
//
// See renderProgress() below for details.
const progressRemoveSequence = "\x1b]9;4;0\x07"

// Terminal progress bar state
//
// Ref: https://rockorager.dev/misc/osc-9-4-progress-bars/
type Progress struct {
	State   ProgressState
	Percent int
}

// percent will be ignored for states Remove and Indeterminate
func (screen *UnixScreen) SetProgress(state ProgressState, percent int) {
	if state < ProgressStateRemove || state > ProgressStatePause {
		panic(fmt.Errorf("invalid progress state: %d", state))
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	screen.renderLock.Lock()
	defer screen.renderLock.Unlock()

	screen.progress = Progress{
		State:   state,
		Percent: percent,
	}
}

// You must hold renderLock when calling this method.
func (screen *UnixScreen) renderProgressLocked() string {
	osc := "\x1b]9;4;"
	osc += strconv.Itoa(int(screen.progress.State))

	if screen.progress.State != ProgressStateRemove && screen.progress.State != ProgressStateIndeterminate {
		osc += ";"
		osc += strconv.Itoa(screen.progress.Percent)
	}

	osc += "\x07"

	return osc
}
