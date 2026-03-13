package core

// Flash emulates an AMD-style flash ROM chip as used in NGPC cartridges.
//
// The chip supports command sequences for chip ID read, byte program,
// block erase, chip erase, and block protection. Command addresses
// are relative to offset 0 within the chip (not the CPU address space).
//
// State machine:
//
//	READ -> CMD1 -> CMD2 -> ID_MODE / PROGRAM / ERASE1 / PROTECT1
//	                        ERASE1 -> ERASE2 -> ERASE3
//	                        PROTECT1 -> PROTECT2 -> PROTECT3
//
// The caller is responsible for translating CPU addresses to chip-relative
// offsets before calling Read/Write.
type Flash struct {
	data     []byte
	mfgID    uint8
	devID    uint8
	state    flashState
	chipSize int
}

type flashState int

const (
	flashRead     flashState = iota
	flashCMD1                // received $AA at $5555
	flashCMD2                // received $55 at $2AAA
	flashIDMode              // chip ID mode
	flashProgram             // next write programs a byte
	flashErase1              // received $80 at CMD2
	flashErase2              // received $AA at $5555 (erase)
	flashErase3              // received $55 at $2AAA (erase)
	flashProtect1            // received $9A at CMD2
	flashProtect2            // received $AA at $5555 (protect)
	flashProtect3            // received $55 at $2AAA (protect)
)

// NewFlash creates a flash chip using the provided data buffer.
// The buffer is used directly (not copied). The manufacturer and device
// IDs are determined from the data length per the NGPC cartridge spec.
func NewFlash(data []byte) *Flash {
	f := &Flash{
		data:     data,
		chipSize: len(data),
	}
	f.mfgID = 0x98 // Toshiba
	switch {
	case len(data) <= 512*1024:
		f.devID = 0xAB // 4 Mbit
	case len(data) <= 1024*1024:
		f.devID = 0x2C // 8 Mbit
	default:
		f.devID = 0x2F // 16 Mbit
	}
	return f
}

// Read returns a byte from the flash chip at the given offset.
// In ID mode, low address bits select the ID register instead of ROM data.
func (f *Flash) Read(offset uint32) uint8 {
	if f.state == flashIDMode {
		switch offset & 0x03 {
		case 0x00:
			return f.mfgID
		case 0x01:
			return f.devID
		case 0x02:
			return 0x00 // block protection status (not protected)
		case 0x03:
			return 0x80 // additional device info
		}
	}
	if int(offset) < len(f.data) {
		return f.data[offset]
	}
	return 0
}

// Write processes a command byte written to the flash chip at the given offset.
// This drives the AMD-style command state machine.
func (f *Flash) Write(offset uint32, val uint8) {
	// $F0 written to any address always resets to read mode
	if val == 0xF0 {
		f.state = flashRead
		return
	}

	switch f.state {
	case flashRead:
		if offset == 0x5555 && val == 0xAA {
			f.state = flashCMD1
		}

	case flashCMD1:
		if offset == 0x2AAA && val == 0x55 {
			f.state = flashCMD2
		} else {
			f.state = flashRead
		}

	case flashCMD2:
		if offset == 0x5555 {
			switch val {
			case 0x90:
				f.state = flashIDMode
			case 0xA0:
				f.state = flashProgram
			case 0x80:
				f.state = flashErase1
			case 0x9A:
				f.state = flashProtect1
			default:
				f.state = flashRead
			}
		} else {
			f.state = flashRead
		}

	case flashIDMode:
		// In ID mode, $AA to $5555 starts a new command sequence
		if offset == 0x5555 && val == 0xAA {
			f.state = flashCMD1
		}

	case flashProgram:
		// Program byte: can only clear bits (AND operation)
		if int(offset) < len(f.data) {
			f.data[offset] &= val
		}
		f.state = flashRead

	case flashErase1:
		if offset == 0x5555 && val == 0xAA {
			f.state = flashErase2
		} else {
			f.state = flashRead
		}

	case flashErase2:
		if offset == 0x2AAA && val == 0x55 {
			f.state = flashErase3
		} else {
			f.state = flashRead
		}

	case flashErase3:
		if offset == 0x5555 && val == 0x10 {
			// Chip erase: fill all with $FF
			for i := range f.data {
				f.data[i] = 0xFF
			}
		} else if val == 0x30 {
			// Block erase: fill the addressed block with $FF
			f.EraseBlock(offset)
		}
		f.state = flashRead

	case flashProtect1:
		if offset == 0x5555 && val == 0xAA {
			f.state = flashProtect2
		} else {
			f.state = flashRead
		}

	case flashProtect2:
		if offset == 0x2AAA && val == 0x55 {
			f.state = flashProtect3
		} else {
			f.state = flashRead
		}

	case flashProtect3:
		// Block protection is acknowledged but not tracked
		// (no games depend on protection readback)
		f.state = flashRead
	}
}

// Reset returns the flash chip to read mode.
func (f *Flash) Reset() {
	f.state = flashRead
}

// Data returns the underlying flash data buffer.
func (f *Flash) Data() []byte {
	return f.data
}

// EraseBlock fills the block containing offset with $FF.
func (f *Flash) EraseBlock(offset uint32) {
	start, end := f.blockBounds(offset)
	if start >= end {
		return
	}
	for i := start; i < end; i++ {
		f.data[i] = 0xFF
	}
}

// blockBounds returns the start (inclusive) and end (exclusive) offsets
// of the flash block containing offset.
func (f *Flash) blockBounds(offset uint32) (int, int) {
	size := f.chipSize
	if size <= 0 {
		return 0, 0
	}

	// Boot block region starts at chip_size - 64KB
	bootStart := size - 0x10000
	if bootStart < 0 {
		bootStart = 0
	}

	if int(offset) < bootStart {
		// Main block region: 64 KB blocks
		blk := int(offset) & ^0xFFFF
		end := blk + 0x10000
		if end > bootStart {
			end = bootStart
		}
		return blk, end
	}

	// Boot block region (top 64 KB, irregular sizes):
	// offset within boot region
	bootOff := int(offset) - bootStart
	switch {
	case bootOff < 0x8000: // 32 KB block
		return bootStart, bootStart + 0x8000
	case bootOff < 0xA000: // 8 KB block
		return bootStart + 0x8000, bootStart + 0xA000
	case bootOff < 0xC000: // 8 KB block
		return bootStart + 0xA000, bootStart + 0xC000
	default: // 16 KB block (top)
		return bootStart + 0xC000, size
	}
}
