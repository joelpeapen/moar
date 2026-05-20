package search

// MatchRanges collects match indices
type MatchRanges struct {
	Matches [][2]int
}

// InRange says true if the index is part of a regexp match
func (mr *MatchRanges) InRange(index int) bool {
	if mr == nil {
		return false
	}

	for _, match := range mr.Matches {
		matchFirstIndex := match[0]
		matchLastIndex := match[1] - 1

		if index < matchFirstIndex {
			continue
		}

		if index > matchLastIndex {
			continue
		}

		return true
	}

	return false
}

func (mr *MatchRanges) Empty() bool {
	return mr == nil || len(mr.Matches) == 0
}
