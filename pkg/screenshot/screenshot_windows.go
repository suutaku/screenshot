package screenshot

import (
	"github.com/suutaku/screenshot/internal/win"
)

type Screenshot struct {
	*win.WinNative
}

func NewScreenshot(x, y, w, h int) *Screenshot {
	return &Screenshot{
		win.NewWinNative(x, y, w, h),
	}
}
