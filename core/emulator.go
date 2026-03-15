package core

import (
	"github.com/user-none/eblitui/coreif"
	"github.com/user-none/ecolr/core/tlcs900h"
)

const (
	// NGPC display is 160x152 pixels
	ScreenWidth     = 160
	MaxScreenHeight = 152
	sampleRate      = 48000
	activeScanlines = 152 // Maximum active display scanlines (render limit)
)

// Emulator contains the emulator core components.
type Emulator struct {
	cpu  *tlcs900h.CPU
	mem  *Memory
	psg  *T6W28
	k2ge *K2GE

	// Frame timing
	cpuCyclesPerScanline   int // Hardware constant (515)
	z80CyclesPerScanlineFP int // Fixed-point 16.16 for Z80+PSG

	// Timing
	timing    RegionTiming
	scanlines int

	// Framebuffer: 160x152 RGBA
	framebuffer []byte

	// Pre-allocated audio buffers
	audioBuffer []int16

	// DAC audio
	dacBufferL []float32
	dacBufferR []float32
	dacGain    float32

	// BIOS state
	hasBIOS bool

	// Pending option state (applied by Start)
	pendingFirstBoot bool
	pendingFastBoot  bool
	pendingLanguage  string
	pendingPalette   string
}

// NewEmulator creates and initializes the emulator components.
// Call SetBIOS before Start() to use a real BIOS; otherwise HLE mode is used.
func NewEmulator(rom []byte, region coreif.Region) (Emulator, error) {
	timing := NGPCTiming

	samplesPerFrame := sampleRate / timing.FPS
	psg := NewT6W28(timing.Z80ClockHz, sampleRate, samplesPerFrame*2)

	mem, err := NewMemory(rom, psg)
	if err != nil {
		return Emulator{}, err
	}

	c := tlcs900h.New(mem)
	mem.SetCPU(c)

	cpuCyclesPerScanline := timing.ClocksPerScanline
	// Z80 fixed-point 16.16: (Z80ClockHz * ClocksPerScanline << 16) / CPUClockHz
	z80CyclesPerScanlineFP := int(int64(timing.Z80ClockHz) * int64(timing.ClocksPerScanline) << 16 / int64(timing.CPUClockHz))

	framebuffer := make([]byte, ScreenWidth*MaxScreenHeight*4)
	k2ge := NewK2GE(&mem.k2ge, framebuffer)

	dacBufSize := samplesPerFrame * 2

	return Emulator{
		cpu:                    c,
		mem:                    mem,
		psg:                    psg,
		k2ge:                   k2ge,
		cpuCyclesPerScanline:   cpuCyclesPerScanline,
		z80CyclesPerScanlineFP: z80CyclesPerScanlineFP,
		timing:                 timing,
		scanlines:              timing.Scanlines,
		framebuffer:            framebuffer,
		audioBuffer:            make([]int16, 0, 2048),
		dacBufferL:             make([]float32, dacBufSize),
		dacBufferR:             make([]float32, dacBufSize),
		dacGain:                0.5,
		pendingFastBoot:        true,
		pendingLanguage:        "English",
		pendingPalette:         "Black & White",
	}, nil
}

// SetBIOS stores BIOS data to be applied when Start() is called.
func (e *Emulator) SetBIOS(key string, data []byte) {
	if key == "system_bios" {
		e.mem.SetBIOS(data)
		e.hasBIOS = true
	}
}

// RunFrame executes one frame of emulation.
func (e *Emulator) RunFrame() {
	e.audioBuffer = e.audioBuffer[:0]
	e.psg.ResetBuffer()

	prevPos := 0
	var z80AccumFP int
	var cpuAccumFP int
	gearDiv := e.mem.ClockGearDivisor()
	cpuPerScanlineFP := (e.cpuCyclesPerScanline << 16) / gearDiv

	for i := 0; i < e.scanlines; i++ {
		e.mem.SetRasterPosition(i, e.cpuCyclesPerScanline)

		// Run TLCS-900/H CPU for this scanline, scaled by clock gear
		cpuAccumFP += cpuPerScanlineFP
		budget := cpuAccumFP >> 16
		cpuAccumFP &= 0xFFFF
		for budget > 0 {
			budget -= e.cpu.StepCycles(budget)
			e.mem.Tick()
		}

		// HBlank signal (152 per frame, none at line 151)
		if i <= 150 || i == 198 {
			e.mem.HBlank()
		}

		// VBlank transition detection
		if i == activeScanlines {
			e.mem.SetVBlankStatus(true)
			if e.mem.VBlankEnabled() {
				e.mem.RequestINT4()
			}
			e.mem.CheckInterrupts()
		} else if i == 0 {
			e.mem.SetVBlankStatus(false)
		}

		// Compute Z80 cycles for this scanline from fixed-point accumulator
		z80AccumFP += e.z80CyclesPerScanlineFP
		z80budget := z80AccumFP >> 16
		z80AccumFP &= 0xFFFF

		// Run Z80 sound CPU and PSG in lockstep so that PSG register
		// writes from the Z80 take effect at the correct audio sample
		// position rather than being applied to the entire scanline.
		if e.mem.Z80Active() {
			z80run := z80budget
			z80cpu := e.mem.Z80CPU()
			pending := e.mem.Z80IRQPending()

			for z80run > 0 {
				// Assert INT when there are pending IRQs to deliver.
				if pending > 0 {
					z80cpu.INT(true, 0xFF)
				}

				wasIFF1 := z80cpu.Registers().IFF1
				stepped := z80cpu.Step()
				z80run -= stepped

				// Advance PSG by the same number of clocks the Z80
				// just consumed so register writes land at the
				// correct sample position.
				e.psg.Run(stepped)

				// If IFF1 transitioned from true to false while INT
				// was asserted, the interrupt was serviced.
				if pending > 0 && wasIFF1 && !z80cpu.Registers().IFF1 {
					pending--
					if pending == 0 {
						z80cpu.INT(false, 0xFF)
					}
				}
			}

			if pending > 0 {
				z80cpu.INT(false, 0xFF)
			}
		} else {
			// PSG still runs when Z80 is inactive (free-running oscillator).
			e.psg.Run(z80budget)
		}

		if i < activeScanlines {
			e.k2ge.RenderScanline(i)
		}

		// Fill DAC buffer for samples generated this scanline
		newPos := e.psg.BufferPos()
		if newPos > prevPos {
			dacL, dacR := e.mem.DACValues()
			dacFloatL := (float32(dacL) - 128.0) / 128.0
			dacFloatR := (float32(dacR) - 128.0) / 128.0
			for j := prevPos; j < newPos && j < len(e.dacBufferL); j++ {
				e.dacBufferL[j] = dacFloatL
				e.dacBufferR[j] = dacFloatR
			}
			prevPos = newPos
		}
	}

	e.mixAudio()
}

// mixAudio converts T6W28 stereo float32 output plus DAC to int16 stereo PCM.
func (e *Emulator) mixAudio() {
	bufL, bufR, count := e.psg.GetBuffers()
	for i := 0; i < count; i++ {
		left := bufL[i]
		right := bufR[i]
		if i < len(e.dacBufferL) {
			left += e.dacBufferL[i] * e.dacGain
			right += e.dacBufferR[i] * e.dacGain
		}
		li := int32(left * 32767)
		ri := int32(right * 32767)
		if li > 32767 {
			li = 32767
		} else if li < -32768 {
			li = -32768
		}
		if ri > 32767 {
			ri = 32767
		} else if ri < -32768 {
			ri = -32768
		}
		e.audioBuffer = append(e.audioBuffer, int16(li), int16(ri))
	}
}

// GetAudioSamples returns accumulated audio samples as 16-bit stereo PCM.
func (e *Emulator) GetAudioSamples() []int16 {
	return e.audioBuffer
}

// GetFramebuffer returns raw RGBA pixel data for current frame.
func (e *Emulator) GetFramebuffer() []byte {
	return e.framebuffer
}

// GetFramebufferStride returns the stride (bytes per row) of the framebuffer.
func (e *Emulator) GetFramebufferStride() int {
	return ScreenWidth * 4
}

// GetActiveHeight returns the current active display height in pixels.
func (e *Emulator) GetActiveHeight() int {
	return MaxScreenHeight
}

// SetInput sets controller state as a button bitmask for the given player.
func (e *Emulator) SetInput(player int, buttons uint32) {
	if player != 0 {
		return
	}
	// Extract coreif bits: 0-5 direct (Up,Down,Left,Right,A,B), bit 7 is Option.
	// NGPC $B0 format: bits 0-5 same, bit 6 = Option, bit 7 = Power (always 0).
	// Both are active-high (1=pressed).
	low6 := uint8(buttons) & 0x3F
	option := uint8((buttons >> 7) & 1)
	packed := low6 | (option << 6)
	e.mem.SetInput(packed)
}

// GetRegion returns NTSC. The NGPC has no PAL variant.
func (e *Emulator) GetRegion() coreif.Region {
	return coreif.RegionNTSC
}

// SetRegion is a no-op. The NGPC is NTSC-only.
func (e *Emulator) SetRegion(region coreif.Region) {
}

// GetTiming returns FPS and scanline count for the current region.
func (e *Emulator) GetTiming() coreif.Timing {
	return coreif.Timing{
		FPS:       e.timing.FPS,
		Scanlines: e.timing.Scanlines,
	}
}

// monoPalette defines a monochrome color scheme for NGP games.
type monoPalette struct {
	Name   string    // Display name (matches BIOS color selection screen)
	Index  uint32    // BIOS palette index stored at $6F94
	Shades [8]uint32 // 8 K1GE shade colors in K2GE hardware format ($0BGR)
}

// monoPalettes lists the available monochrome color schemes.
// Shade values extracted directly from BIOS ROM at $FF50B5.
var monoPalettes = []monoPalette{
	{"Black & White", 0x00, [8]uint32{0x0FFF, 0x0DDD, 0x0BBB, 0x0999, 0x0777, 0x0444, 0x0333, 0x0000}},
	{"Red", 0x01, [8]uint32{0x0FFF, 0x0CCF, 0x099F, 0x055F, 0x011D, 0x0009, 0x0006, 0x0000}},
	{"Green", 0x02, [8]uint32{0x0FFF, 0x0BFB, 0x07F7, 0x03D3, 0x00B0, 0x0080, 0x0050, 0x0000}},
	{"Blue", 0x03, [8]uint32{0x0FFF, 0x0FCC, 0x0FAA, 0x0F88, 0x0E55, 0x0B22, 0x0700, 0x0000}},
	{"Classic", 0x04, [8]uint32{0x0FFF, 0x0ADE, 0x08BD, 0x059B, 0x0379, 0x0157, 0x0034, 0x0000}},
}

// SetOption applies a core option change identified by key.
// Values are stored and applied when Start() is called.
func (e *Emulator) SetOption(key string, value string) {
	switch key {
	case "first_boot":
		e.pendingFirstBoot = (value == "true")
	case "fast_boot":
		e.pendingFastBoot = (value == "true")
	case "mono_palette":
		e.pendingPalette = value
	case "language":
		e.pendingLanguage = value
	}
}

// applyLanguage writes the language setting to system RAM.
func (e *Emulator) applyLanguage(value string) {
	var lang uint32
	if value == "Japanese" {
		lang = 0x00
	} else {
		lang = 0x01
	}
	e.mem.Write8(0x6F87, uint8(lang))
}

// Start finalizes emulator state after all options are applied.
// If a real BIOS was provided via SetBIOS, it initializes system RAM
// and sets PC to the NVRAM boot path (or cart entry for fast boot).
// Otherwise, HLE BIOS mode is used.
func (e *Emulator) Start() {
	e.mem.InitSystemState()

	if e.hasBIOS {
		if e.pendingFastBoot {
			// Enter the BIOS after all animation screens (setup, SNK logo,
			// startup) but before cart handoff. This address begins the
			// post-animation sequence: RAM vector table init (mode 0),
			// flash re-probe, license validation, and cart entry jump.
			// The vector table setup is needed so games have the correct
			// default interrupt handlers in place.
			e.cpu.SetPC(0xFF1BFC)
		} else {
			e.cpu.SetPC(0xFF1800)

			if e.pendingFirstBoot {
				e.mem.Write16(0x6E95, 0x0000)
				return
			}
		}
	} else {
		hle := newHLEBIOS(e.mem)
		synthBIOS := hle.generateBIOS()
		e.mem.SetBIOS(synthBIOS)
		hle.install(e.cpu)
		hle.initVectorTable(e.mem)
	}

	e.applyLanguage(e.pendingLanguage)
	e.applyMonoPalette(e.pendingPalette)
	e.mem.Write16(0x6E95, 0x4E50)
	e.updateSetupChecksum()
}

// applyMonoPalette writes the selected color scheme to the K1GE
// compatibility palette areas in K2GE VRAM. This sets the shade-to-color
// mapping used when rendering monochrome (NGP) games.
func (e *Emulator) applyMonoPalette(name string) {
	pal := &monoPalettes[0]
	for i := range monoPalettes {
		if monoPalettes[i].Name == name {
			pal = &monoPalettes[i]
			break
		}
	}
	e.mem.WriteK1GEPalettes(pal.Shades, pal.Index)
}

// updateSetupChecksum recomputes the BIOS setup completion checksum at
// $6C14-$6C15. The 16-bit sum covers $6F87 (language) + $6C25-$6C2B
// (interrupt priority config) + $6F94 (mono palette index).
func (e *Emulator) updateSetupChecksum() {
	var sum uint32
	sum += uint32(e.mem.Read8(0x6F87))
	for addr := uint32(0x6C25); addr <= 0x6C2B; addr++ {
		sum += uint32(e.mem.Read8(addr))
	}
	sum += uint32(e.mem.Read8(0x6F94))
	e.mem.Write8(0x6C14, uint8(sum))
	e.mem.Write8(0x6C15, uint8(sum>>8))
}

// Close releases any resources held by the emulator.
func (e *Emulator) Close() {
}

// =============================================================================
// Battery Save (Flash)
// =============================================================================

// HasSRAM reports whether the loaded ROM uses battery-backed save.
// NGPC cartridges use flash memory for saves.
func (e *Emulator) HasSRAM() bool {
	return e.mem.cart != nil
}

// GetSRAM returns save data in NGF format containing only modified flash
// blocks. Returns nil if no blocks have been modified.
func (e *Emulator) GetSRAM() []byte {
	if e.mem.cart == nil {
		return nil
	}
	return buildNGF(e.mem.origCart, e.mem.cart)
}

// SetSRAM restores flash state from NGF-format save data. The cart is
// reset to the original ROM contents, then NGF block deltas are applied.
func (e *Emulator) SetSRAM(data []byte) {
	if e.mem.cart == nil {
		return
	}
	copy(e.mem.cart, e.mem.origCart)
	if len(data) > 0 {
		_ = applyNGF(e.mem.cart, data)
	}
	e.mem.initFlash()
}

// =============================================================================
// Memory Inspector
// =============================================================================

// Flat address boundaries for ReadMemory.
const (
	workRAMFlatStart = 0x0000
	workRAMFlatEnd   = 0x2FFF // 12 KB work RAM
	z80RAMFlatStart  = 0x3000
	z80RAMFlatEnd    = 0x3FFF // 4 KB Z80 shared RAM
	k2geFlatStart    = 0x4000
	k2geFlatEnd      = 0x7FFF // 16 KB K2GE VRAM
)

// ReadMemory reads from a flat address into buf and returns the number
// of bytes read. NGPC flat address mapping:
//
//	0x0000-0x2FFF -> Work RAM (12 KB)
//	0x3000-0x3FFF -> Z80 shared RAM (4 KB)
//	0x4000-0x7FFF -> K2GE VRAM (16 KB)
func (e *Emulator) ReadMemory(addr uint32, buf []byte) uint32 {
	var count uint32
	for i := range buf {
		cur := addr + uint32(i)
		switch {
		case cur <= workRAMFlatEnd:
			buf[i] = e.mem.workRAM[cur]
		case cur >= z80RAMFlatStart && cur <= z80RAMFlatEnd:
			buf[i] = e.mem.z80RAM[cur-z80RAMFlatStart]
		case cur >= k2geFlatStart && cur <= k2geFlatEnd:
			buf[i] = e.mem.k2ge[cur-k2geFlatStart]
		default:
			return count
		}
		count++
	}
	return count
}

// =============================================================================
// Memory Mapper
// =============================================================================

// MemoryMap returns a list of available memory regions with sizes.
// SystemRAM includes work RAM (12 KB) + Z80 shared RAM (4 KB) = 16 KB,
// matching the rcheevos memory map for Neo Geo Pocket ($0000-$3FFF -> $4000-$7FFF).
func (e *Emulator) MemoryMap() []coreif.MemoryRegion {
	regions := []coreif.MemoryRegion{
		{Type: coreif.MemorySystemRAM, Size: workRAMSize + z80RAMSize},
	}
	if e.mem.cart != nil {
		regions = append(regions, coreif.MemoryRegion{
			Type: coreif.MemorySaveRAM,
			Size: len(e.mem.cart),
		})
	}
	return regions
}

// ReadRegion returns a copy of the specified memory region.
func (e *Emulator) ReadRegion(regionType int) []byte {
	switch regionType {
	case coreif.MemorySystemRAM:
		out := make([]byte, workRAMSize+z80RAMSize)
		copy(out, e.mem.workRAM[:])
		copy(out[workRAMSize:], e.mem.z80RAM[:])
		return out
	case coreif.MemorySaveRAM:
		if e.mem.cart == nil {
			return nil
		}
		out := make([]byte, len(e.mem.cart))
		copy(out, e.mem.cart)
		return out
	default:
		return nil
	}
}

// WriteRegion writes data to the specified memory region.
func (e *Emulator) WriteRegion(regionType int, data []byte) {
	switch regionType {
	case coreif.MemorySystemRAM:
		copy(e.mem.workRAM[:], data)
		if len(data) > workRAMSize {
			copy(e.mem.z80RAM[:], data[workRAMSize:])
		}
	case coreif.MemorySaveRAM:
		if e.mem.cart != nil {
			copy(e.mem.cart, data)
		}
	}
}
