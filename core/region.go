package core

// VideoTiming holds timing constants for the video standard.
// The NGPC has a TLCS-900/H main CPU and a Z80 sound CPU.
type VideoTiming struct {
	CPUClockHz        int // TLCS-900/H clock frequency
	Z80ClockHz        int // Z80 sound CPU clock frequency
	ClocksPerScanline int // CPU clocks per horizontal scanline period
	Scanlines         int // Total scanlines per frame
	FPS               int // Approximate frames per second (for UI pacing)
}

// NGPCTiming holds the fixed timing constants for the Neo Geo Pocket Color.
// CPU 6.144 MHz, Z80 3.072 MHz, 515 clocks/scanline,
// 199 scanlines, ~59.95 Hz (6144000 / (515*199))
var NGPCTiming = VideoTiming{
	CPUClockHz:        6144000,
	Z80ClockHz:        3072000,
	ClocksPerScanline: 515,
	Scanlines:         199,
	FPS:               60,
}
