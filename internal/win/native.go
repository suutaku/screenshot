package win

import (
	"errors"
	"image"
	"syscall"
	"unsafe"

	winapi "github.com/lxn/win"
	"github.com/suutaku/screenshot/internal/utils"
)

var (
	libUser32, _               = syscall.LoadLibrary("user32.dll")
	funcGetDesktopWindow, _    = syscall.GetProcAddress(syscall.Handle(libUser32), "GetDesktopWindow")
	funcEnumDisplayMonitors, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplayMonitors")
	funcGetMonitorInfo, _      = syscall.GetProcAddress(syscall.Handle(libUser32), "GetMonitorInfoW")
	funcEnumDisplaySettings, _ = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplaySettingsW")
)

type WinNative struct {
	x       int
	y       int
	w       int
	h       int
	hwnd    *winapi.HWND
	hdc     *winapi.HWND
	mem_dev *winapi.HDC
}

func NewWinNative(x, y, w, h int) *WinNative {
	hwnd := getDesktopWindow()
	hdc := winapi.GetDC(hwnd)
	mem_dev := winapi.CreateCompatibleDC(hdc)
	if mem_dev == 0 {
		panic("create compatible dc failed")
	}

	return &WinNative{
		x:       x,
		y:       y,
		w:       w,
		h:       h,
		hwnd:    hwnd,
		hdc:     hdc,
		mem_dev: mem_dev,
	}
}

func (wn *WinNative) Capture() (*image.RGBA, error) {
	var rect image.Rectangle
	if wn.w < 1 || wn.h < 1 {
		rect = wn.GetDisplayBounds(0)
		wn.w = rect.Dx()
		wn.y = rect.Dy()
	} else {
		rect = image.Rect(0, 0, wn.w, wn.h)
	}

	img, err := utils.CreateImage(rect)
	if err != nil {
		return nil, err
	}
	bitmap := winapi.CreateCompatibleBitmap(wn.hdc, int32(wn.w), int32(wn.h))
	if bitmap == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer winapi.DeleteObject(winapi.HGDIOBJ(bitmap))
	var header winapi.BITMAPINFOHEADER
	header.BiSize = uint32(unsafe.Sizeof(header))
	header.BiPlanes = 1
	header.BiBitCount = 32
	header.BiWidth = int32(wn.w)
	header.BiHeight = int32(-wn.h)
	header.BiCompression = winapi.BI_RGB
	header.BiSizeImage = 0
	// GetDIBits balks at using Go memory on some systems. The MSDN example uses
	// GlobalAlloc, so we'll do that too. See:
	// https://docs.microsoft.com/en-gb/windows/desktop/gdi/capturing-an-image
	bitmapDataSize := uintptr(((int64(wn.w)*int64(header.BiBitCount) + 31) / 32) * 4 * int64(wn.h))
	hmem := winapi.GlobalAlloc(winapi.GMEM_MOVEABLE, bitmapDataSize)
	defer winapi.GlobalFree(hmem)
	memptr := winapi.GlobalLock(hmem)
	defer winapi.GlobalUnlock(hmem)

	old := winapi.SelectObject(wn.mem_dev, winapi.HGDIOBJ(bitmap))
	if old == 0 {
		return nil, errors.New("SelectObject failed")
	}
	defer winapi.SelectObject(wn.mem_dev, old)

	if !winapi.BitBlt(wn.mem_dev, 0, 0, int32(wn.w), int32(wn.h), wn.hdc, int32(wn.x), int32(wn.y), winapi.SRCCOPY) {
		return nil, errors.New("BitBlt failed")
	}

	if winapi.GetDIBits(wn.hdc, bitmap, 0, uint32(wn.h), (*uint8)(memptr), (*winapi.BITMAPINFO)(unsafe.Pointer(&header)), winapi.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	i := 0
	src := uintptr(memptr)
	for y := 0; y < wn.h; y++ {
		for x := 0; x < wn.w; x++ {
			v0 := *(*uint8)(unsafe.Pointer(src))
			v1 := *(*uint8)(unsafe.Pointer(src + 1))
			v2 := *(*uint8)(unsafe.Pointer(src + 2))

			// BGRA => RGBA, and set A to 255
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255

			i += 4
			src += 4
		}
	}

	return img, nil
}
func (wn *WinNative) Close() {
	winapi.ReleaseDC(wn.hwnd, wn.hdc)
	winapi.DeleteDC(wn.mem_dev)
}
func (wn *WinNative) GetDisplayBounds(num int) image.Rectangle {
	var ctx getMonitorBoundsContext
	ctx.Index = num
	ctx.Count = 0
	enumDisplayMonitors(winapi.HDC(0), nil, syscall.NewCallback(getMonitorBoundsCallback), uintptr(unsafe.Pointer(&ctx)))
	return image.Rect(
		int(ctx.Rect.Left), int(ctx.Rect.Top),
		int(ctx.Rect.Right), int(ctx.Rect.Bottom))
}
func (wn *WinNative) GetDisplayNumber() int {
	var count int = 0
	enumDisplayMonitors(winapi.HDC(0), nil, syscall.NewCallback(countupMonitorCallback), uintptr(unsafe.Pointer(&count)))
	return count
}

func getDesktopWindow() winapi.HWND {
	ret, _, _ := syscall.Syscall(funcGetDesktopWindow, 0, 0, 0, 0)
	return winapi.HWND(ret)
}

func enumDisplayMonitors(hdc winapi.HDC, lprcClip *winapi.RECT, lpfnEnum uintptr, dwData uintptr) bool {
	ret, _, _ := syscall.Syscall6(funcEnumDisplayMonitors, 4,
		uintptr(hdc),
		uintptr(unsafe.Pointer(lprcClip)),
		lpfnEnum,
		dwData,
		0,
		0)
	return int(ret) != 0
}

func countupMonitorCallback(hMonitor winapi.HMONITOR, hdcMonitor winapi.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var count *int
	count = (*int)(unsafe.Pointer(dwData))
	*count = *count + 1
	return uintptr(1)
}

type getMonitorBoundsContext struct {
	Index int
	Rect  winapi.RECT
	Count int
}

func getMonitorBoundsCallback(hMonitor winapi.HMONITOR, hdcMonitor winapi.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var ctx *getMonitorBoundsContext
	ctx = (*getMonitorBoundsContext)(unsafe.Pointer(dwData))
	if ctx.Count != ctx.Index {
		ctx.Count = ctx.Count + 1
		return uintptr(1)
	}

	if realSize := getMonitorRealSize(hMonitor); realSize != nil {
		ctx.Rect = *realSize
	} else {
		ctx.Rect = *lprcMonitor
	}

	return uintptr(0)
}

type _MONITORINFOEX struct {
	winapi.MONITORINFO
	DeviceName [winapi.CCHDEVICENAME]uint16
}

const _ENUM_CURRENT_SETTINGS = 0xFFFFFFFF

type _DEVMODE struct {
	_            [68]byte
	DmSize       uint16
	_            [6]byte
	DmPosition   winapi.POINT
	_            [86]byte
	DmPelsWidth  uint32
	DmPelsHeight uint32
	_            [40]byte
}

// getMonitorRealSize makes a call to GetMonitorInfo
// to obtain the device name for the monitor handle
// provided to the method.
//
// With the device name, EnumDisplaySettings is called to
// obtain the current configuration for the monitor, this
// information includes the real resolution of the monitor
// rather than the scaled version based on DPI.
//
// If either handle calls fail, it will return a nil
// allowing the calling method to use the bounds information
// returned by EnumDisplayMonitors which may be affected
// by DPI.
func getMonitorRealSize(hMonitor winapi.HMONITOR) *winapi.RECT {
	info := _MONITORINFOEX{}
	info.CbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := syscall.Syscall(funcGetMonitorInfo, 2, uintptr(hMonitor), uintptr(unsafe.Pointer(&info)), 0)
	if ret == 0 {
		return nil
	}

	devMode := _DEVMODE{}
	devMode.DmSize = uint16(unsafe.Sizeof(devMode))

	if ret, _, _ := syscall.Syscall(funcEnumDisplaySettings, 3, uintptr(unsafe.Pointer(&info.DeviceName[0])), _ENUM_CURRENT_SETTINGS, uintptr(unsafe.Pointer(&devMode))); ret == 0 {
		return nil
	}

	return &winapi.RECT{
		Left:   devMode.DmPosition.X,
		Right:  devMode.DmPosition.X + int32(devMode.DmPelsWidth),
		Top:    devMode.DmPosition.Y,
		Bottom: devMode.DmPosition.Y + int32(devMode.DmPelsHeight),
	}
}
