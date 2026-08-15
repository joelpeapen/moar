package twin

type ProgressState int

const (
	ProgressStateRemove ProgressState = iota
	ProgressStateSet
	ProgressStateError
	ProgressStateIndeterminate
	ProgressStatePause
)

// Terminal progress bar state
//
// Ref: https://rockorager.dev/misc/osc-9-4-progress-bars/
type Progress struct {
	State   ProgressState
	Percent int
}

// percent will be ignored for states Remove and Indeterminate
func (screen *UnixScreen) SetProgress(state ProgressState, percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	screen.progress = Progress{
		State:   state,
		Percent: percent,
	}
}
