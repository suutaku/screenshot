package x11

import (
	"fmt"
	"image"
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
	img       *image.RGBA
	seg       *mshm.Seg
	data      []byte
	shmId     int
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
		fmt.Println("not use shm")
		useShm = false
	}

	screen := xproto.Setup(c).DefaultScreen(c)
	var intersect image.Rectangle
	wholeScreenBounds := image.Rect(0, 0, int(screen.WidthInPixels), int(screen.HeightInPixels))
	if w > 1 && h > 1 {
		targetBounds := image.Rect(x+x0, y+y0, x+x0+w, y+y0+h)
		intersect = wholeScreenBounds.Intersect(targetBounds)
	} else {
		intersect = wholeScreenBounds
		w = wholeScreenBounds.Dx()
		h = wholeScreenBounds.Dy()
	}
	if intersect.Empty() {
		err = fmt.Errorf("select invalid range")
		panic(err)
	}
	rect := image.Rect(0, 0, w, h)
	img, err := utils.CreateImage(rect)
	if err != nil {
		panic(err)
	}
	ret := &X11Window{
		c:         c,
		useShm:    useShm,
		screen:    screen,
		x:         x,
		x0:        x0,
		y:         y,
		y0:        y0,
		reply:     reply,
		w:         w,
		h:         h,
		img:       img,
		intersect: intersect,
	}

	if useShm {
		shmSize := intersect.Dx() * intersect.Dy() * 4
		ret.shmId, err = shm.Get(shm.IPC_PRIVATE, shmSize, shm.IPC_CREAT|0777)
		if err != nil {
			panic(err)

		}

		seg, err := mshm.NewSegId(c)
		if err != nil {
			panic(err)
		}
		ret.seg = &seg
		ret.data, err = shm.At(ret.shmId, 0, 0)
		if err != nil {
			panic(err)
		}

		mshm.Attach(c, seg, uint32(ret.shmId), false)
	}
	return ret
}

func (x11 *X11Window) Close() {
	if x11.c != nil {
		log.Println("close conn")
		x11.c.Close()
	}
	if x11.useShm {
		mshm.Detach(x11.c, *x11.seg)
		shm.Rm(x11.shmId)
		shm.Dt(x11.data)
	}
}

func (x11 *X11Window) toRGBA() *image.RGBA {
	for i := 0; i < len(x11.data); i += 4 {
		x11.img.Pix[i] = x11.data[i+2]
		x11.img.Pix[i+1] = x11.data[i+1]
		x11.img.Pix[i+2] = x11.data[i]
		x11.img.Pix[i+3] = 255
	}
	return x11.img
}

func (x11 *X11Window) Capture() (img *image.RGBA, err error) {

	if x11.useShm {
		_, err = mshm.GetImage(x11.c, xproto.Drawable(x11.screen.Root),
			int16(x11.intersect.Min.X), int16(x11.intersect.Min.Y),
			uint16(x11.intersect.Dx()), uint16(x11.intersect.Dy()), 0xffffffff,
			byte(xproto.ImageFormatZPixmap), *x11.seg, 0).Reply()
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

		x11.data = xImg.Data
	}

	return x11.toRGBA(), err
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
