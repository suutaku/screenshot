package x11

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"github.com/gen2brain/shm"
	"github.com/jezek/xgb"
	mshm "github.com/jezek/xgb/shm"
	"github.com/jezek/xgb/xinerama"
	"github.com/jezek/xgb/xproto"
	"github.com/suutaku/screenshot/internal/utils"
)

type X11Window struct {
	c         *xgb.Conn
	useShm    bool
	intersect image.Rectangle
	screen    *xproto.ScreenInfo
	x         int
	x0        int
	y         int
	y0        int
	reply     *xinerama.QueryScreensReply
	w         int
	h         int
}

func NewX11Window(x, y, w, h int) *X11Window {
	c, err := xgb.NewConn()
	if err != nil {
		panic(err)
	}

	err = xinerama.Init(c)
	if err != nil {
		panic(err)
	}

	reply, err := xinerama.QueryScreens(c).Reply()
	if err != nil {
		panic(err)
	}

	primary := reply.ScreenInfo[0]
	x0 := int(primary.XOrg)
	y0 := int(primary.YOrg)

	useShm := true
	err = mshm.Init(c)
	if err != nil {
		useShm = false
	}

	screen := xproto.Setup(c).DefaultScreen(c)

	return &X11Window{
		c:      c,
		useShm: useShm,
		screen: screen,
		x:      x,
		x0:     x0,
		y:      y,
		y0:     y0,
		reply:  reply,
		w:      w,
		h:      h,
	}
}

func (x11 *X11Window) Close() {
	if x11.c != nil {
		log.Println("close conn")
		x11.c.Close()
	}
}

func (x11 *X11Window) Capture() (img *image.RGBA, err error) {
	wholeScreenBounds := image.Rect(0, 0, int(x11.screen.WidthInPixels), int(x11.screen.HeightInPixels))
	if x11.w > 1 && x11.h > 1 {
		targetBounds := image.Rect(x11.x+x11.x0, x11.y+x11.y0, x11.x+x11.x0+x11.w, x11.y+x11.y0+x11.h)
		x11.intersect = wholeScreenBounds.Intersect(targetBounds)
	} else {
		x11.intersect = wholeScreenBounds
		x11.w = wholeScreenBounds.Dx()
		x11.h = wholeScreenBounds.Dy()
	}
	if x11.intersect.Empty() {
		err = fmt.Errorf("select invalid range")
		return
	}

	var data []byte
	rect := image.Rect(0, 0, x11.w, x11.h)
	img, err = utils.CreateImage(rect)
	if err != nil {
		return nil, err
	}
	if x11.useShm {
		shmSize := x11.intersect.Dx() * x11.intersect.Dy() * 4
		shmId, err := shm.Get(shm.IPC_PRIVATE, shmSize, shm.IPC_CREAT|0777)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		seg, err := mshm.NewSegId(x11.c)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		data, err = shm.At(shmId, 0, 0)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		mshm.Attach(x11.c, seg, uint32(shmId), false)

		defer mshm.Detach(x11.c, seg)
		defer shm.Rm(shmId)
		defer shm.Dt(data)

		_, err = mshm.GetImage(x11.c, xproto.Drawable(x11.screen.Root),
			int16(x11.intersect.Min.X), int16(x11.intersect.Min.Y),
			uint16(x11.intersect.Dx()), uint16(x11.intersect.Dy()), 0xffffffff,
			byte(xproto.ImageFormatZPixmap), seg, 0).Reply()
		if err != nil {
			log.Println(err)
			return nil, err
		}
	} else {
		xImg, err := xproto.GetImage(x11.c, xproto.ImageFormatZPixmap, xproto.Drawable(x11.screen.Root),
			int16(x11.intersect.Min.X), int16(x11.intersect.Min.Y),
			uint16(x11.intersect.Dx()), uint16(x11.intersect.Dy()), 0xffffffff).Reply()
		if err != nil {
			log.Println(err)
			return nil, err
		}

		data = xImg.Data
	}
	offset := 0
	for iy := x11.intersect.Min.Y; iy < x11.intersect.Max.Y; iy++ {
		for ix := x11.intersect.Min.X; ix < x11.intersect.Max.X; ix++ {
			r := data[offset+2]
			g := data[offset+1]
			b := data[offset]
			img.SetRGBA(ix-(x11.x+x11.x0), iy-(x11.y+x11.y0), color.RGBA{r, g, b, 255})
			offset += 4
		}
	}

	return img, err
}

func (x11 *X11Window) GetDisplayNumber() int {
	return int(x11.reply.Number)
}

func (x11 *X11Window) GetDisplayBounds(num int) image.Rectangle {
	if num >= int(x11.reply.Number) {
		return image.ZR
	}
	screen := x11.reply.ScreenInfo[num]
	x := int(screen.XOrg) - x11.x0
	y := int(screen.YOrg) - x11.y0
	w := int(screen.Width)
	h := int(screen.Height)
	return image.Rect(x, y, x+w, y+h)
}
