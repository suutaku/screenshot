package screenshot

import (
	"github.com/suutaku/screenshot/internal/win"
)

type Screenshot struct {
	*x11.X11Window
}

func NewScreenshot(x, y, w, h int) *Screenshot {
	return &Screenshot{
		win.NewWinNative(x, y, w, h),
	}
}
