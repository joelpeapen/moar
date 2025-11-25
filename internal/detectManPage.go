package internal

import (
	"github.com/walles/moor/v2/internal/linemetadata"
)

func (p *Pager) haveLoadedManPage() bool {
	reader := p.Reader()
	if reader == nil {
		return false
	}

	for _, line := range reader.GetLines(linemetadata.Index{}, 10).Lines {
		if line.Line.HasManPageFormatting() {
			return true
		}
	}
	return false
}
