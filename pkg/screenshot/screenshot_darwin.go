package screenshot

import (
	"github.com/suutaku/screenshot/internal/cg"
)

type Screenshot struct {
	*cg.CoreGraph
}

func NewScreenshot(x, y, w, h int) *Screenshot {
	return &Screenshot{
		cg.NewCoreGraph(x, y, w, h),
	}
}
