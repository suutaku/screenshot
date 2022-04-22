package screenshot

import (
	"github.com/suutaku/screenshot/internal/x11"
)

type Screenshot struct {
	*x11.X11Window
}

func NewScreenshot(x, y, w, h int) *Screenshot {
	return &Screenshot{
		x11.NewX11Window(x, y, w, h),
	}
}
