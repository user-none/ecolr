package debug

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/user-none/ecolr/core"
)

// WritePNGFrame renders the current K2GE state and writes it as a PNG image.
func WritePNGFrame(mem *core.Memory, path string, frameNum int) {
	fb := make([]byte, core.ScreenWidth*core.MaxScreenHeight*4)
	k := core.NewK2GE(mem.VRAM(), fb)
	for i := 0; i < core.MaxScreenHeight; i++ {
		k.RenderScanline(i)
	}

	img := image.NewRGBA(image.Rect(0, 0, core.ScreenWidth, core.MaxScreenHeight))
	for y := 0; y < core.MaxScreenHeight; y++ {
		for x := 0; x < core.ScreenWidth; x++ {
			off := (y*core.ScreenWidth + x) * 4
			img.SetRGBA(x, y, color.RGBA{
				R: fb[off], G: fb[off+1], B: fb[off+2], A: fb[off+3],
			})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating frame file: %v\n", err)
		return
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding PNG: %v\n", err)
		return
	}
	fmt.Printf("  [Frame %d rendered to %s]\n", frameNum, path)
}
