package core

// K2GE implements the K2 Graphics Engine video renderer for the Neo Geo Pocket Color.
// It reads from memory-mapped VRAM ($8000-$BFFF) and composites scanlines into
// an RGBA framebuffer.
type K2GE struct {
	vram *[k2geSize]byte
	fb   []byte // 160*152*4 RGBA
}

// VRAM offsets (relative to $8000 base)
const (
	k2geSpriteAttr     = 0x0800 // $8800: sprite attribute table (64 x 4 bytes)
	k2geSpritePalAsn   = 0x0C00 // $8C00: sprite palette assignments (64 bytes, K2GE only)
	k2gePlane1Map      = 0x1000 // $9000: scroll plane 1 tile map
	k2gePlane2Map      = 0x1800 // $9800: scroll plane 2 tile map
	k2geCharRAM        = 0x2000 // $A000: character RAM (512 tiles x 16 bytes)
	k2geSpritePal      = 0x0200 // $8200: sprite palettes (K2GE mode)
	k2gePlane1Pal      = 0x0280 // $8280: scroll plane 1 palettes (K2GE mode)
	k2gePlane2Pal      = 0x0300 // $8300: scroll plane 2 palettes (K2GE mode)
	k1geSpritePal      = 0x0380 // $8380: K1GE compat sprite palettes
	k1gePlane1Pal      = 0x03A0 // $83A0: K1GE compat scroll plane 1 palettes
	k1gePlane2Pal      = 0x03C0 // $83C0: K1GE compat scroll plane 2 palettes
	k2geBGPal          = 0x03E0 // $83E0: background color palette
	k2geWindowPal      = 0x03F0 // $83F0: window/outside-window palette
	k1geShadeLUTSprite = 0x0100 // $8100: sprite shade palettes (2x4)
	k1geShadeLUTPlane1 = 0x0108 // $8108: scroll plane 1 shade palettes (2x4)
	k1geShadeLUTPlane2 = 0x0110 // $8110: scroll plane 2 shade palettes (2x4)
	k2geMaxSprites     = 64
	k2geTilesPerPlane  = 32 // 32x32 tile map
)

// NewK2GE creates a K2GE renderer referencing the given VRAM and framebuffer.
func NewK2GE(vram *[k2geSize]byte, fb []byte) *K2GE {
	return &K2GE{vram: vram, fb: fb}
}

// RenderScanline composites one scanline into the framebuffer.
func (k *K2GE) RenderScanline(scanline int) {
	if scanline < 0 || scanline >= MaxScreenHeight {
		return
	}

	k1ge := k.vram[0x07E2]&0x80 != 0

	var line [ScreenWidth]uint32

	// 1. Background
	k.renderBackground(&line)

	// 2. Determine plane ordering from $8030 bit 7 and palette bases from mode
	pf := k.vram[0x0030] & 0x80
	var rearMapBase, frontMapBase int
	var rearPalBase, frontPalBase int
	var rearScrollX, rearScrollY uint8
	var frontScrollX, frontScrollY uint8
	var spritePalBase int

	if k1ge {
		spritePalBase = k1geSpritePal
	} else {
		spritePalBase = k2geSpritePal
	}

	if pf == 0 {
		// Plane 1 in front, Plane 2 in rear
		rearMapBase = k2gePlane2Map
		frontMapBase = k2gePlane1Map
		rearScrollX = k.vram[0x0034]
		rearScrollY = k.vram[0x0035]
		frontScrollX = k.vram[0x0032]
		frontScrollY = k.vram[0x0033]
		if k1ge {
			rearPalBase = k1gePlane2Pal
			frontPalBase = k1gePlane1Pal
		} else {
			rearPalBase = k2gePlane2Pal
			frontPalBase = k2gePlane1Pal
		}
	} else {
		// Plane 2 in front, Plane 1 in rear
		rearMapBase = k2gePlane1Map
		frontMapBase = k2gePlane2Map
		rearScrollX = k.vram[0x0032]
		rearScrollY = k.vram[0x0033]
		frontScrollX = k.vram[0x0034]
		frontScrollY = k.vram[0x0035]
		if k1ge {
			rearPalBase = k1gePlane1Pal
			frontPalBase = k1gePlane2Pal
		} else {
			rearPalBase = k2gePlane1Pal
			frontPalBase = k2gePlane2Pal
		}
	}

	// Shade LUT bases for K1GE mode
	var rearShadeLUT, frontShadeLUT int
	if pf == 0 {
		rearShadeLUT = k1geShadeLUTPlane2
		frontShadeLUT = k1geShadeLUTPlane1
	} else {
		rearShadeLUT = k1geShadeLUTPlane1
		frontShadeLUT = k1geShadeLUTPlane2
	}

	// Compute sprite positions once for all three PRC passes
	var spritePosX, spritePosY [k2geMaxSprites]int
	k.computeSpritePositions(&spritePosX, &spritePosY)

	// 3. Sprites PR.C=01 (behind both planes)
	k.renderSprites(&line, scanline, 0x01, k1ge, spritePalBase, k1geShadeLUTSprite, &spritePosX, &spritePosY)

	// 4. Rear scroll plane
	k.renderScrollPlane(&line, scanline, rearMapBase, rearPalBase, rearScrollX, rearScrollY, k1ge, rearShadeLUT)

	// 5. Sprites PR.C=10 (between planes)
	k.renderSprites(&line, scanline, 0x02, k1ge, spritePalBase, k1geShadeLUTSprite, &spritePosX, &spritePosY)

	// 6. Front scroll plane
	k.renderScrollPlane(&line, scanline, frontMapBase, frontPalBase, frontScrollX, frontScrollY, k1ge, frontShadeLUT)

	// 7. Sprites PR.C=11 (in front of both planes)
	k.renderSprites(&line, scanline, 0x03, k1ge, spritePalBase, k1geShadeLUTSprite, &spritePosX, &spritePosY)

	// 8. Window masking
	k.applyWindow(&line, scanline)

	// 9. NEG inversion
	k.applyNEG(&line)

	// 10. Copy to framebuffer
	off := scanline * ScreenWidth * 4
	dst := k.fb[off : off+ScreenWidth*4 : off+ScreenWidth*4]
	for x := 0; x < ScreenWidth; x++ {
		rgba := line[x]
		dst[0] = uint8(rgba >> 24)
		dst[1] = uint8(rgba >> 16)
		dst[2] = uint8(rgba >> 8)
		dst[3] = uint8(rgba)
		dst = dst[4:]
	}
}

// paletteToRGBA converts a 12-bit palette entry at the given VRAM offset to packed RGBA.
// Palette entries are 16-bit LE: lo byte has R (bits 3-0) and G (bits 7-4),
// hi byte has B (bits 3-0).
func (k *K2GE) paletteToRGBA(offset int) uint32 {
	lo := k.vram[offset]
	hi := k.vram[offset+1]

	r := uint32(lo&0x0F) * 17
	g := uint32(lo>>4) * 17
	b := uint32(hi&0x0F) * 17

	return r<<24 | g<<16 | b<<8 | 0xFF
}

// shadeToRGBA reads a shade value from the LUT and looks up the
// corresponding color from the K1GE compat palette.
// palIdx (from P.C bit) selects both the shade LUT palette and
// the compat color palette group: P.C=0 uses entries 0-7,
// P.C=1 uses entries 8-15.
func (k *K2GE) shadeToRGBA(shadeLUTBase, compatPalBase, palIdx, colorIdx int) uint32 {
	shade := int(k.vram[shadeLUTBase+palIdx*4+colorIdx] & 0x07)
	return k.paletteToRGBA(compatPalBase + palIdx*16 + shade*2)
}

// getTilePixel returns the 2-bit color index for a pixel within a tile.
// tileIdx is the 9-bit tile number, px/py are pixel coordinates (0-7).
func (k *K2GE) getTilePixel(tileIdx, px, py int, hflip, vflip bool) uint8 {
	if vflip {
		py = 7 - py
	}
	if hflip {
		px = 7 - px
	}
	rowAddr := k2geCharRAM + tileIdx*16 + py*2
	if rowAddr+1 >= k2geSize {
		return 0
	}
	lo := k.vram[rowAddr]
	hi := k.vram[rowAddr+1]
	var colorIdx uint8
	if px < 4 {
		colorIdx = (hi >> uint((3-px)*2)) & 0x03
	} else {
		colorIdx = (lo >> uint((7-px)*2)) & 0x03
	}
	return colorIdx
}

// renderBackground fills the line buffer with the background color.
func (k *K2GE) renderBackground(line *[ScreenWidth]uint32) {
	bgReg := k.vram[0x0118]
	bgColorIdx := 0
	// Background enabled when bits 7-6 = $80 (bit 7 set, bit 6 clear)
	if bgReg&0xC0 == 0x80 {
		bgColorIdx = int(bgReg & 0x07)
	}
	rgba := k.paletteToRGBA(k2geBGPal + bgColorIdx*2)
	for x := 0; x < ScreenWidth; x++ {
		line[x] = rgba
	}
}

// renderScrollPlane renders a scroll plane onto the line buffer.
// In K1GE compat mode, P.C (bit 5) selects palette 0 or 1 from the compat area.
// In K2GE mode, CP.C (bits 4-1) selects palette 0-15 from the standard area.
func (k *K2GE) renderScrollPlane(line *[ScreenWidth]uint32, scanline, mapBase, palBase int, scrollX, scrollY uint8, k1ge bool, shadeLUTBase int) {
	mapY := (scanline + int(scrollY)) & 0xFF
	tileRow := mapY >> 3
	pixY := mapY & 7
	rowBase := mapBase + tileRow*k2geTilesPerPlane*2

	for screenX := 0; screenX < ScreenWidth; {
		mapX := (screenX + int(scrollX)) & 0xFF
		tileCol := mapX >> 3
		startPx := mapX & 7
		count := 8 - startPx
		if screenX+count > ScreenWidth {
			count = ScreenWidth - screenX
		}

		entryOffset := rowBase + tileCol*2
		byte0 := k.vram[entryOffset]
		byte1 := k.vram[entryOffset+1]

		tileIdx := int(byte0) | (int(byte1&0x01) << 8)
		hflip := byte1&0x80 != 0
		vflip := byte1&0x40 != 0

		var palIdx int
		if k1ge {
			palIdx = int((byte1 >> 5) & 0x01)
		} else {
			palIdx = int((byte1 >> 1) & 0x0F)
		}

		py := pixY
		if vflip {
			py = 7 - py
		}
		rowAddr := k2geCharRAM + tileIdx*16 + py*2
		row := uint16(k.vram[rowAddr+1])<<8 | uint16(k.vram[rowAddr])

		tilePalBase := palBase + palIdx*8

		for i := 0; i < count; i++ {
			px := startPx + i
			if hflip {
				px = 7 - px
			}

			colorIdx := uint8((row >> uint((7-px)*2)) & 0x03)

			if colorIdx == 0 {
				screenX++
				continue
			}

			if k1ge {
				line[screenX] = k.shadeToRGBA(shadeLUTBase, palBase, palIdx, int(colorIdx))
			} else {
				line[screenX] = k.paletteToRGBA(tilePalBase + int(colorIdx)*2)
			}
			screenX++
		}
	}
}

// computeSpritePositions resolves chaining to produce absolute positions
// for all 64 sprites. Called once per scanline before the PRC render passes.
func (k *K2GE) computeSpritePositions(posX, posY *[k2geMaxSprites]int) {
	pohReg := k.vram[0x0020]
	povReg := k.vram[0x0021]

	for i := 0; i < k2geMaxSprites; i++ {
		attrBase := k2geSpriteAttr + i*4
		byte1 := k.vram[attrBase+1]
		hp := k.vram[attrBase+2]
		vp := k.vram[attrBase+3]

		if i == 0 || byte1&0x04 == 0 {
			posX[i] = int(hp) + int(pohReg)
		} else {
			posX[i] = posX[i-1] + int(int8(hp))
		}
		if i == 0 || byte1&0x02 == 0 {
			posY[i] = int(vp) + int(povReg)
		} else {
			posY[i] = posY[i-1] + int(int8(vp))
		}
	}
}

// renderSprites renders sprites with the given PR.C value onto the line buffer.
// posX/posY are pre-computed absolute sprite positions from computeSpritePositions.
// In K1GE compat mode, P.C (bit 5) selects palette 0 or 1 from the compat area.
// In K2GE mode, $8C00+N selects palette 0-15 from the standard area.
func (k *K2GE) renderSprites(line *[ScreenWidth]uint32, scanline int, targetPRC uint8, k1ge bool, spritePalBase int, shadeLUTBase int, posX, posY *[k2geMaxSprites]int) {
	// Render in reverse order (63->0) so lower-numbered sprites overwrite
	for i := k2geMaxSprites - 1; i >= 0; i-- {
		attrBase := k2geSpriteAttr + i*4
		byte0 := k.vram[attrBase]
		byte1 := k.vram[attrBase+1]

		prc := (byte1 >> 3) & 0x03
		if prc != targetPRC {
			continue
		}

		// Check if this sprite intersects the current scanline
		sy := posY[i] & 0xFF
		relY := scanline - sy
		if relY < 0 || relY >= 8 {
			relY = scanline - (sy - 256)
			if relY < 0 || relY >= 8 {
				continue
			}
		}

		tileIdx := int(byte0) | (int(byte1&0x01) << 8)
		hflip := byte1&0x80 != 0
		vflip := byte1&0x40 != 0

		var palIdx int
		if k1ge {
			palIdx = int((byte1 >> 5) & 0x01)
		} else {
			palIdx = int(k.vram[k2geSpritePalAsn+i] & 0x0F)
		}

		sx := posX[i] & 0xFF
		spritePalOff := spritePalBase + palIdx*8

		for px := 0; px < 8; px++ {
			screenX := (sx + px) & 0xFF
			if screenX >= ScreenWidth {
				continue
			}

			colorIdx := k.getTilePixel(tileIdx, px, relY, hflip, vflip)
			if colorIdx == 0 {
				continue
			}

			if k1ge {
				line[screenX] = k.shadeToRGBA(shadeLUTBase, spritePalBase, palIdx, int(colorIdx))
			} else {
				line[screenX] = k.paletteToRGBA(spritePalOff + int(colorIdx)*2)
			}
		}
	}
}

// applyWindow masks pixels outside the window rectangle with the OOWC color.
func (k *K2GE) applyWindow(line *[ScreenWidth]uint32, scanline int) {
	wbaH := int(k.vram[0x0002])
	wbaV := int(k.vram[0x0003])
	wsiH := int(k.vram[0x0004])
	wsiV := int(k.vram[0x0005])

	oowcIdx := int(k.vram[0x0012] & 0x07)
	oowcColor := k.paletteToRGBA(k2geWindowPal + oowcIdx*2)

	// Check if scanline is outside window vertically
	if scanline < wbaV || scanline >= wbaV+wsiV {
		for x := 0; x < ScreenWidth; x++ {
			line[x] = oowcColor
		}
		return
	}

	// Fill left strip outside window
	for x := 0; x < wbaH && x < ScreenWidth; x++ {
		line[x] = oowcColor
	}
	// Fill right strip outside window
	wEnd := wbaH + wsiH
	if wEnd < ScreenWidth {
		for x := wEnd; x < ScreenWidth; x++ {
			line[x] = oowcColor
		}
	}
}

// applyNEG inverts all RGB channels if $8012 bit 7 is set.
// Alpha is always 0xFF so XOR on the upper 24 bits is equivalent to 255-ch per channel.
func (k *K2GE) applyNEG(line *[ScreenWidth]uint32) {
	if k.vram[0x0012]&0x80 == 0 {
		return
	}
	for x := 0; x < ScreenWidth; x++ {
		line[x] ^= 0xFFFFFF00
	}
}
