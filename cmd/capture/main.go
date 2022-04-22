package main

import (
	"log"

	"github.com/suutaku/screenshot/pkg/screenshot"
)

func main() {
	log.SetFlags(log.Lshortfile)
	impl := screenshot.NewScreenshot(0, 0, 0, 0)
	defer impl.Close()
	for {
		img, err := impl.Capture()
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(img.Rect.Max)
	}
}
