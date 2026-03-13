package core

import "testing"

// newTestK2GE creates a K2GE with fresh VRAM and framebuffer for testing.
func newTestK2GE() (*K2GE, *[k2geSize]byte, []byte) {
	var vram [k2geSize]byte
	fb := make([]byte, ScreenWidth*MaxScreenHeight*4)
	k := NewK2GE(&vram, fb)
	return k, &vram, fb
}

// setPaletteEntry writes a 12-bit color (R4,G4,B4) at the given VRAM offset.
func setPaletteEntry(vram *[k2geSize]byte, offset int, r, g, b uint8) {
	val := uint16(r&0x0F) | uint16(g&0x0F)<<4 | uint16(b&0x0F)<<8
	vram[offset] = uint8(val)
	vram[offset+1] = uint8(val >> 8)
}

// readFBPixel reads an RGBA pixel from the framebuffer at (x, scanline).
func readFBPixel(fb []byte, x, scanline int) (r, g, b, a uint8) {
	i := (scanline*ScreenWidth + x) * 4
	return fb[i], fb[i+1], fb[i+2], fb[i+3]
}

// writeTileRow writes a 2bpp tile row (8 pixels) into character RAM.
// pixels is an array of 8 color indices (0-3).
func writeTileRow(vram *[k2geSize]byte, tileIdx, row int, pixels [8]uint8) {
	// Pack into 16-bit LE word: pixel 0 in bits 15:14, pixel 7 in bits 1:0
	var hi, lo uint8
	for px := 0; px < 4; px++ {
		hi |= (pixels[px] & 0x03) << uint((3-px)*2)
	}
	for px := 4; px < 8; px++ {
		lo |= (pixels[px] & 0x03) << uint((7-px)*2)
	}
	addr := k2geCharRAM + tileIdx*16 + row*2
	vram[addr] = lo
	vram[addr+1] = hi
}

func TestK2GE_BackgroundEnabled(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable background: bit 7 set, bit 6 clear = $80, BGC index = 2
	vram[0x0118] = 0x82

	// Set BG palette entry 2 to red (R=15, G=0, B=0)
	setPaletteEntry(vram, k2geBGPal+2*2, 15, 0, 0)

	// Set window to full screen
	vram[0x0002] = 0   // WBA.H
	vram[0x0003] = 0   // WBA.V
	vram[0x0004] = 160 // WSI.H
	vram[0x0005] = 152 // WSI.V

	k.RenderScanline(0)

	r, g, b, a := readFBPixel(fb, 80, 0)
	if r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("expected (255,0,0,255), got (%d,%d,%d,%d)", r, g, b, a)
	}
}

func TestK2GE_BackgroundDisabled(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Background disabled: bits 7-6 = 0
	vram[0x0118] = 0x00

	// Set window to full screen
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	k.RenderScanline(0)

	r, g, b, a := readFBPixel(fb, 80, 0)
	if r != 0 || g != 0 || b != 0 || a != 255 {
		t.Errorf("expected (0,0,0,255), got (%d,%d,%d,%d)", r, g, b, a)
	}

	// With non-black palette entry 0, BG disabled should use palette index 0
	setPaletteEntry(vram, k2geBGPal+0*2, 15, 0, 5) // R=255, G=0, B=85
	k.RenderScanline(0)

	r, g, b, a = readFBPixel(fb, 80, 0)
	if r != 255 || g != 0 || b != 85 || a != 255 {
		t.Errorf("BG disabled with palette[0] set: expected (255,0,85,255), got (%d,%d,%d,%d)", r, g, b, a)
	}
}

func TestK2GE_TilePixelDecode(t *testing.T) {
	k, vram, _ := newTestK2GE()

	// Tile 0: row 0 = all pixels set to color index 3
	writeTileRow(vram, 0, 0, [8]uint8{3, 3, 3, 3, 3, 3, 3, 3})

	for px := 0; px < 8; px++ {
		ci := k.getTilePixel(0, px, 0, false, false)
		if ci != 3 {
			t.Errorf("pixel %d: expected 3, got %d", px, ci)
		}
	}

	// Test individual pixels
	writeTileRow(vram, 1, 0, [8]uint8{0, 1, 2, 3, 3, 2, 1, 0})
	expected := [8]uint8{0, 1, 2, 3, 3, 2, 1, 0}
	for px := 0; px < 8; px++ {
		ci := k.getTilePixel(1, px, 0, false, false)
		if ci != expected[px] {
			t.Errorf("pixel %d: expected %d, got %d", px, expected[px], ci)
		}
	}
}

func TestK2GE_TilePixelHFlip(t *testing.T) {
	k, vram, _ := newTestK2GE()

	writeTileRow(vram, 0, 0, [8]uint8{1, 0, 0, 0, 0, 0, 0, 2})

	// Without flip: px=0 -> 1, px=7 -> 2
	if ci := k.getTilePixel(0, 0, 0, false, false); ci != 1 {
		t.Errorf("no flip px=0: expected 1, got %d", ci)
	}
	// With hflip: px=0 reads from px=7 -> 2
	if ci := k.getTilePixel(0, 0, 0, true, false); ci != 2 {
		t.Errorf("hflip px=0: expected 2, got %d", ci)
	}
}

func TestK2GE_TilePixelVFlip(t *testing.T) {
	k, vram, _ := newTestK2GE()

	writeTileRow(vram, 0, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})
	writeTileRow(vram, 0, 7, [8]uint8{2, 2, 2, 2, 2, 2, 2, 2})

	// Without flip: row 0 -> 1
	if ci := k.getTilePixel(0, 0, 0, false, false); ci != 1 {
		t.Errorf("no flip row=0: expected 1, got %d", ci)
	}
	// With vflip: row 0 reads from row 7 -> 2
	if ci := k.getTilePixel(0, 0, 0, false, true); ci != 2 {
		t.Errorf("vflip row=0: expected 2, got %d", ci)
	}
}

func TestK2GE_ScrollPlaneBasicTile(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Set background to black (disabled)
	vram[0x0118] = 0x00

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Write tile 1: row 0, all pixels = color index 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Plane 1 tile map entry at (0,0): tile 1, palette 0, no flip
	mapAddr := k2gePlane1Map
	vram[mapAddr] = 1   // CC[7:0] = 1
	vram[mapAddr+1] = 0 // no flip, palette 0, CC[8]=0

	// Set plane 1 palette 0, color 1 to green (R=0, G=15, B=0)
	setPaletteEntry(vram, k2gePlane1Pal+0*8+1*2, 0, 15, 0)

	// No scroll offset
	vram[0x0032] = 0
	vram[0x0033] = 0

	// Plane 1 in front (P.F=0)
	vram[0x0030] = 0

	k.RenderScanline(0)

	// First 8 pixels should be green
	for x := 0; x < 8; x++ {
		r, g, b, _ := readFBPixel(fb, x, 0)
		if r != 0 || g != 255 || b != 0 {
			t.Errorf("pixel %d: expected (0,255,0), got (%d,%d,%d)", x, r, g, b)
		}
	}
}

func TestK2GE_ScrollPlaneWithOffset(t *testing.T) {
	k, vram, fb := newTestK2GE()

	vram[0x0118] = 0x00

	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Write tile 1: row 0, pixel 0 = color 2, rest transparent
	writeTileRow(vram, 1, 0, [8]uint8{2, 0, 0, 0, 0, 0, 0, 0})

	// Plane 1 tile (0,0) = tile 1
	mapAddr := k2gePlane1Map
	vram[mapAddr] = 1
	vram[mapAddr+1] = 0

	// Set palette
	setPaletteEntry(vram, k2gePlane1Pal+0*8+2*2, 0, 0, 15) // blue

	// Scroll X = 3: pixel at map position 3 appears at screen X=0
	// Tile (0,0) pixel 3 is transparent. But the first pixel of tile 1 is at map X=0,
	// which would appear at screen X = (0 - 3) & 0xFF = 253 (off screen).
	// Screen X=0 maps to mapX=3 which is tile (0,0) pixel 3 = transparent.
	// Screen X=5 maps to mapX=8 which is tile (1,0) pixel 0 = transparent (no tile set).
	// Let's use scroll X=0 and check that tile pixel 0 is at screen X=0.
	vram[0x0032] = 0
	vram[0x0033] = 0
	vram[0x0030] = 0

	k.RenderScanline(0)

	// Screen X=0 should be blue (tile 1, pixel 0, color 2)
	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("pixel 0: expected (0,0,255), got (%d,%d,%d)", r, g, b)
	}

	// Screen X=1 should be black (transparent -> background)
	r, g, b, _ = readFBPixel(fb, 1, 0)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("pixel 1: expected (0,0,0), got (%d,%d,%d)", r, g, b)
	}

	// Now with scroll X=1, screen X=0 maps to map X=1 which is pixel 1 (transparent)
	vram[0x0032] = 1
	k.RenderScanline(0)

	r, g, b, _ = readFBPixel(fb, 0, 0)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("scrolled pixel 0: expected (0,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_SpriteBasic(t *testing.T) {
	k, vram, fb := newTestK2GE()

	vram[0x0118] = 0x00

	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Write tile 5: row 0, all pixels = color 1
	writeTileRow(vram, 5, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Sprite 0: tile 5, PR.C=11 (in front), position (10, 0)
	attr := k2geSpriteAttr
	vram[attr] = 5      // CC[7:0] = 5
	vram[attr+1] = 0x18 // PR.C=11, no flip, no chain, CC[8]=0
	vram[attr+2] = 10   // H.P = 10
	vram[attr+3] = 0    // V.P = 0

	// Sprite palette assignment: palette 0
	vram[k2geSpritePalAsn] = 0

	// Set sprite palette 0, color 1 to cyan (R=0, G=15, B=15)
	setPaletteEntry(vram, k2geSpritePal+0*8+1*2, 0, 15, 15)

	// PO.H=0, PO.V=0
	vram[0x0020] = 0
	vram[0x0021] = 0

	k.RenderScanline(0)

	// Pixels 10-17 should be cyan
	for x := 10; x < 18; x++ {
		r, g, b, _ := readFBPixel(fb, x, 0)
		if r != 0 || g != 255 || b != 255 {
			t.Errorf("pixel %d: expected (0,255,255), got (%d,%d,%d)", x, r, g, b)
		}
	}

	// Pixel 9 should be black (background)
	r, g, b, _ := readFBPixel(fb, 9, 0)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("pixel 9: expected (0,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_SpritePriority(t *testing.T) {
	k, vram, fb := newTestK2GE()

	vram[0x0118] = 0x00

	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Tile 1: all pixels = color 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})
	// Tile 2: all pixels = color 1
	writeTileRow(vram, 2, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Sprite 0: tile 1, PR.C=11, position (0, 0), palette 0
	vram[k2geSpriteAttr+0] = 1
	vram[k2geSpriteAttr+1] = 0x18 // PR.C=11
	vram[k2geSpriteAttr+2] = 0
	vram[k2geSpriteAttr+3] = 0
	vram[k2geSpritePalAsn+0] = 0

	// Sprite 1: tile 2, PR.C=11, position (0, 0), palette 1
	vram[k2geSpriteAttr+4] = 2
	vram[k2geSpriteAttr+5] = 0x18 // PR.C=11
	vram[k2geSpriteAttr+6] = 0
	vram[k2geSpriteAttr+7] = 0
	vram[k2geSpritePalAsn+1] = 1

	// Palette 0, color 1 = red
	setPaletteEntry(vram, k2geSpritePal+0*8+1*2, 15, 0, 0)
	// Palette 1, color 1 = green
	setPaletteEntry(vram, k2geSpritePal+1*8+1*2, 0, 15, 0)

	vram[0x0020] = 0
	vram[0x0021] = 0

	k.RenderScanline(0)

	// Sprite 0 (lower numbered) should win -> red
	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_SpriteChaining(t *testing.T) {
	k, vram, fb := newTestK2GE()

	vram[0x0118] = 0x00

	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Tile 1: row 0, all = color 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Sprite 0: tile 1, PR.C=11, position (20, 0), palette 0
	vram[k2geSpriteAttr+0] = 1
	vram[k2geSpriteAttr+1] = 0x18
	vram[k2geSpriteAttr+2] = 20
	vram[k2geSpriteAttr+3] = 0
	vram[k2geSpritePalAsn+0] = 0

	// Sprite 1: tile 1, PR.C=11, H.ch=1, V.ch=1, offset (+8, 0)
	vram[k2geSpriteAttr+4] = 1
	vram[k2geSpriteAttr+5] = 0x18 | 0x04 | 0x02 // PR.C=11, H.ch, V.ch
	vram[k2geSpriteAttr+6] = 8                  // H offset = +8
	vram[k2geSpriteAttr+7] = 0                  // V offset = 0
	vram[k2geSpritePalAsn+1] = 1

	// Palette 0, color 1 = red; Palette 1, color 1 = green
	setPaletteEntry(vram, k2geSpritePal+0*8+1*2, 15, 0, 0)
	setPaletteEntry(vram, k2geSpritePal+1*8+1*2, 0, 15, 0)

	vram[0x0020] = 0
	vram[0x0021] = 0

	k.RenderScanline(0)

	// Sprite 0 at x=20-27 -> red
	r, g, b, _ := readFBPixel(fb, 20, 0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("sprite 0 at 20: expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}

	// Sprite 1 chained at x=28-35 -> green
	r, g, b, _ = readFBPixel(fb, 28, 0)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("sprite 1 at 28: expected (0,255,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_WindowMasking(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable background with color
	vram[0x0118] = 0x82
	setPaletteEntry(vram, k2geBGPal+2*2, 15, 0, 0) // red

	// Window: origin (10, 5), size (100, 100)
	vram[0x0002] = 10
	vram[0x0003] = 5
	vram[0x0004] = 100
	vram[0x0005] = 100

	// OOWC = index 1, set window palette entry 1 to blue
	vram[0x0012] = 0x01
	setPaletteEntry(vram, k2geWindowPal+1*2, 0, 0, 15)

	// Scanline 0 is outside window vertically (window starts at V=5)
	k.RenderScanline(0)

	// All pixels should be OOWC (blue)
	r, g, b, _ := readFBPixel(fb, 50, 0)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("outside window V: expected (0,0,255), got (%d,%d,%d)", r, g, b)
	}

	// Scanline 10 is inside window vertically
	k.RenderScanline(10)

	// Pixel 5 is outside window horizontally (window starts at H=10)
	r, g, b, _ = readFBPixel(fb, 5, 10)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("outside window H: expected (0,0,255), got (%d,%d,%d)", r, g, b)
	}

	// Pixel 50 is inside window -> should be red (background)
	r, g, b, _ = readFBPixel(fb, 50, 10)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("inside window: expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_NEGInversion(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Background enabled, color index 0
	vram[0x0118] = 0x80
	setPaletteEntry(vram, k2geBGPal+0*2, 15, 0, 5) // R=255, G=0, B=85

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Enable NEG
	vram[0x0012] = 0x80

	k.RenderScanline(0)

	// Inverted: R=255->0, G=0->255, B=85->170
	r, g, b, _ := readFBPixel(fb, 80, 0)
	if r != 0 || g != 255 || b != 170 {
		t.Errorf("expected (0,255,170), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_Transparency(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable background with red
	vram[0x0118] = 0x82
	setPaletteEntry(vram, k2geBGPal+2*2, 15, 0, 0) // red

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Tile 1: pixel 0 = color 1 (opaque), pixel 1 = color 0 (transparent)
	writeTileRow(vram, 1, 0, [8]uint8{1, 0, 0, 0, 0, 0, 0, 0})

	// Plane 1 at (0,0) = tile 1
	vram[k2gePlane1Map] = 1
	vram[k2gePlane1Map+1] = 0

	setPaletteEntry(vram, k2gePlane1Pal+0*8+1*2, 0, 15, 0) // green

	vram[0x0032] = 0
	vram[0x0033] = 0
	vram[0x0030] = 0

	k.RenderScanline(0)

	// Pixel 0: opaque green from plane
	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("pixel 0: expected (0,255,0), got (%d,%d,%d)", r, g, b)
	}

	// Pixel 1: transparent -> shows red background
	r, g, b, _ = readFBPixel(fb, 1, 0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("pixel 1: expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_K1GEShadeLUT(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable K1GE mode
	vram[0x07E2] = 0x80

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Background disabled
	vram[0x0118] = 0x00

	// Write tile 1: row 0, pixel 0 = color 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 0, 0, 0, 0, 0, 0, 0})

	// Plane 1 at (0,0) = tile 1, palette 0 (P.C bit 5 = 0)
	vram[k2gePlane1Map] = 1
	vram[k2gePlane1Map+1] = 0

	// Shade LUT for plane 1: palette 0, color 1 -> shade 3
	vram[k1geShadeLUTPlane1+0*4+1] = 3

	// K1GE compat plane 1 palette: shade 3 = red
	setPaletteEntry(vram, k1gePlane1Pal+3*2, 15, 0, 0)

	// No scroll
	vram[0x0032] = 0
	vram[0x0033] = 0
	vram[0x0030] = 0 // P.F=0, plane 1 in front

	k.RenderScanline(0)

	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("K1GE shade LUT scroll: expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_K1GEShadeChange(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable K1GE mode
	vram[0x07E2] = 0x80

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Background disabled
	vram[0x0118] = 0x00

	// Write tile 1: row 0, pixel 0 = color 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 0, 0, 0, 0, 0, 0, 0})

	// Plane 1 at (0,0) = tile 1, palette 0
	vram[k2gePlane1Map] = 1
	vram[k2gePlane1Map+1] = 0

	// Shade LUT: palette 0, color 1 -> shade 2
	vram[k1geShadeLUTPlane1+0*4+1] = 2

	// Shade 2 = red
	setPaletteEntry(vram, k1gePlane1Pal+2*2, 15, 0, 0)
	// Shade 5 = blue
	setPaletteEntry(vram, k1gePlane1Pal+5*2, 0, 0, 15)

	vram[0x0032] = 0
	vram[0x0033] = 0
	vram[0x0030] = 0

	k.RenderScanline(0)

	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("shade 2 render: expected (255,0,0), got (%d,%d,%d)", r, g, b)
	}

	// Change shade to 5
	vram[k1geShadeLUTPlane1+0*4+1] = 5

	k.RenderScanline(0)

	r, g, b, _ = readFBPixel(fb, 0, 0)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("shade 5 render: expected (0,0,255), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_K1GESpriteShade(t *testing.T) {
	k, vram, fb := newTestK2GE()

	// Enable K1GE mode
	vram[0x07E2] = 0x80

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Background disabled
	vram[0x0118] = 0x00

	// Write tile 5: row 0, all pixels = color 1
	writeTileRow(vram, 5, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Sprite 0: tile 5, PR.C=11, position (10, 0), palette 0 (P.C bit 5 = 0)
	attr := k2geSpriteAttr
	vram[attr] = 5
	vram[attr+1] = 0x18 // PR.C=11
	vram[attr+2] = 10   // H.P = 10
	vram[attr+3] = 0    // V.P = 0

	vram[0x0020] = 0 // PO.H
	vram[0x0021] = 0 // PO.V

	// Shade LUT for sprites: palette 0, color 1 -> shade 4
	vram[k1geShadeLUTSprite+0*4+1] = 4

	// K1GE compat sprite palette: shade 4 = green
	setPaletteEntry(vram, k1geSpritePal+4*2, 0, 15, 0)

	k.RenderScanline(0)

	r, g, b, _ := readFBPixel(fb, 10, 0)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("K1GE sprite shade: expected (0,255,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_K1GEShadePalette1(t *testing.T) {
	// P.C=1 should select from the second half of the K1GE compat
	// palette (entries 8-15) rather than the first half (entries 0-7).
	k, vram, fb := newTestK2GE()

	// Enable K1GE mode
	vram[0x07E2] = 0x80

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Background disabled
	vram[0x0118] = 0x00

	// Write tile 1: row 0, all pixels = color 1
	writeTileRow(vram, 1, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Plane 1 at (0,0) = tile 1, P.C=1 (bit 5 set in byte1)
	vram[k2gePlane1Map] = 1
	vram[k2gePlane1Map+1] = 0x20 // P.C bit 5 = 1

	// Shade LUT for plane 1 palette 1: color 1 -> shade 2
	vram[k1geShadeLUTPlane1+1*4+1] = 2

	// K1GE compat plane 1 palette: set shade 2 in BOTH halves
	// to different colors so we can tell which half is used.
	// First half (P.C=0): entry 2 = red
	setPaletteEntry(vram, k1gePlane1Pal+2*2, 15, 0, 0)
	// Second half (P.C=1): entry 8+2=10 = green
	setPaletteEntry(vram, k1gePlane1Pal+10*2, 0, 15, 0)

	// No scroll
	vram[0x0032] = 0
	vram[0x0033] = 0
	vram[0x0030] = 0

	k.RenderScanline(0)

	// With P.C=1, shade 2 should look up from entry 10 (green), not entry 2 (red)
	r, g, b, _ := readFBPixel(fb, 0, 0)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("K1GE P.C=1 shade LUT: expected (0,255,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_K1GESpriteShadePalette1(t *testing.T) {
	// Sprite with P.C=1 should select from entries 8-15 of compat palette.
	k, vram, fb := newTestK2GE()

	// Enable K1GE mode
	vram[0x07E2] = 0x80

	// Full screen window
	vram[0x0002] = 0
	vram[0x0003] = 0
	vram[0x0004] = 160
	vram[0x0005] = 152

	// Background disabled
	vram[0x0118] = 0x00

	// Write tile 5: row 0, all pixels = color 1
	writeTileRow(vram, 5, 0, [8]uint8{1, 1, 1, 1, 1, 1, 1, 1})

	// Sprite 0: tile 5, PR.C=11, P.C=1 (bit 5 set), position (10, 0)
	attr := k2geSpriteAttr
	vram[attr] = 5
	vram[attr+1] = 0x38 // PR.C=11, P.C=1
	vram[attr+2] = 10
	vram[attr+3] = 0

	vram[0x0020] = 0
	vram[0x0021] = 0

	// Shade LUT for sprites palette 1: color 1 -> shade 3
	vram[k1geShadeLUTSprite+1*4+1] = 3

	// First half (P.C=0) entry 3 = red
	setPaletteEntry(vram, k1geSpritePal+3*2, 15, 0, 0)
	// Second half (P.C=1) entry 8+3=11 = blue
	setPaletteEntry(vram, k1geSpritePal+11*2, 0, 0, 15)

	k.RenderScanline(0)

	// With P.C=1, shade 3 should read from entry 11 (blue)
	r, g, b, _ := readFBPixel(fb, 10, 0)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("K1GE sprite P.C=1 shade: expected (0,0,255), got (%d,%d,%d)", r, g, b)
	}
}

func TestK2GE_PaletteColorConversion(t *testing.T) {
	k, vram, _ := newTestK2GE()

	// Test that 4-bit value N maps to N*17 in 8-bit
	tests := []struct {
		r4, g4, b4 uint8
		r8, g8, b8 uint8
	}{
		{0, 0, 0, 0, 0, 0},
		{15, 15, 15, 255, 255, 255},
		{8, 4, 12, 136, 68, 204},
		{1, 1, 1, 17, 17, 17},
	}

	for _, tc := range tests {
		setPaletteEntry(vram, k2geBGPal, tc.r4, tc.g4, tc.b4)
		rgba := k.paletteToRGBA(k2geBGPal)
		r := uint8(rgba >> 24)
		g := uint8(rgba >> 16)
		b := uint8(rgba >> 8)
		if r != tc.r8 || g != tc.g8 || b != tc.b8 {
			t.Errorf("palette (%d,%d,%d): expected (%d,%d,%d), got (%d,%d,%d)",
				tc.r4, tc.g4, tc.b4, tc.r8, tc.g8, tc.b8, r, g, b)
		}
	}
}
