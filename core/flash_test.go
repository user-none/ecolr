package core

import "testing"

func makeFlash512KB() *Flash {
	data := make([]byte, 512*1024)
	for i := range data {
		data[i] = uint8(i)
	}
	return NewFlash(data)
}

func makeFlash2MB() *Flash {
	data := make([]byte, 2*1024*1024)
	for i := range data {
		data[i] = uint8(i)
	}
	return NewFlash(data)
}

func TestFlashNewIDs(t *testing.T) {
	tests := []struct {
		name  string
		size  int
		devID uint8
	}{
		{"4Mbit", 512 * 1024, 0xAB},
		{"8Mbit", 1024 * 1024, 0x2C},
		{"16Mbit", 2 * 1024 * 1024, 0x2F},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFlash(make([]byte, tt.size))
			if f.mfgID != 0x98 {
				t.Errorf("mfgID = 0x%02X, want 0x98", f.mfgID)
			}
			if f.devID != tt.devID {
				t.Errorf("devID = 0x%02X, want 0x%02X", f.devID, tt.devID)
			}
		})
	}
}

func TestFlashReadMode(t *testing.T) {
	f := makeFlash512KB()
	// Normal read returns data
	if got := f.Read(0); got != 0x00 {
		t.Errorf("Read(0) = 0x%02X, want 0x00", got)
	}
	if got := f.Read(5); got != 0x05 {
		t.Errorf("Read(5) = 0x%02X, want 0x05", got)
	}
	// Out of bounds
	if got := f.Read(0x100000); got != 0x00 {
		t.Errorf("Read(out of bounds) = 0x%02X, want 0x00", got)
	}
}

func TestFlashIDMode(t *testing.T) {
	f := makeFlash512KB()

	// Enter ID mode
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x90)

	// Read ID registers
	if got := f.Read(0x00); got != 0x98 {
		t.Errorf("manufacturer ID = 0x%02X, want 0x98", got)
	}
	if got := f.Read(0x01); got != 0xAB {
		t.Errorf("device ID = 0x%02X, want 0xAB", got)
	}
	if got := f.Read(0x02); got != 0x00 {
		t.Errorf("protection = 0x%02X, want 0x00", got)
	}
	if got := f.Read(0x03); got != 0x80 {
		t.Errorf("additional info = 0x%02X, want 0x80", got)
	}

	// ID reads work from any block base
	if got := f.Read(0x10000); got != 0x98 {
		t.Errorf("manufacturer ID at block base = 0x%02X, want 0x98", got)
	}

	// Exit ID mode with $F0
	f.Write(0x0000, 0xF0)

	// Back to normal read
	if got := f.Read(0x00); got != 0x00 {
		t.Errorf("Read(0) after ID exit = 0x%02X, want 0x00", got)
	}
}

func TestFlashIDModeNewCommand(t *testing.T) {
	f := makeFlash512KB()

	// Enter ID mode
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x90)

	// Start new command from ID mode ($AA to $5555)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0xF0) // ID exit via command sequence

	// Should be back in read mode
	if got := f.Read(0x00); got != 0x00 {
		t.Errorf("Read(0) after command exit = 0x%02X, want 0x00", got)
	}
}

func TestFlashProgramByte(t *testing.T) {
	f := makeFlash512KB()

	// Pre-fill target byte to $FF (erased state)
	f.data[0x100] = 0xFF

	// Program byte command
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0xA0)
	f.Write(0x100, 0x42)

	if got := f.Read(0x100); got != 0x42 {
		t.Errorf("programmed byte = 0x%02X, want 0x42", got)
	}

	// Program again: AND operation (can only clear bits)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0xA0)
	f.Write(0x100, 0x0F)

	// $42 & $0F = $02
	if got := f.Read(0x100); got != 0x02 {
		t.Errorf("reprogrammed byte = 0x%02X, want 0x02", got)
	}
}

func TestFlashBlockErase(t *testing.T) {
	f := makeFlash512KB()

	// Verify data exists in first 64KB block
	if f.Read(0x100) == 0xFF {
		t.Fatal("test data should not be 0xFF before erase")
	}

	// Block erase command (6 cycles)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x80)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x0000, 0x30) // erase block containing offset 0

	// First block should be $FF
	for i := 0; i < 0x10000; i++ {
		if f.data[i] != 0xFF {
			t.Errorf("data[0x%X] = 0x%02X after block erase, want 0xFF", i, f.data[i])
			break
		}
	}

	// Second block should be untouched
	if f.data[0x10000] == 0xFF {
		t.Error("second block was erased, should be untouched")
	}
}

func TestFlashChipErase(t *testing.T) {
	f := NewFlash(make([]byte, 512*1024))
	// Fill with non-FF data
	for i := range f.data {
		f.data[i] = 0x42
	}

	// Chip erase
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x80)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x10)

	for i := 0; i < len(f.data); i++ {
		if f.data[i] != 0xFF {
			t.Errorf("data[0x%X] = 0x%02X after chip erase, want 0xFF", i, f.data[i])
			break
		}
	}
}

func TestFlashBootBlockErase(t *testing.T) {
	f := makeFlash512KB()
	chipSize := 512 * 1024
	bootStart := chipSize - 0x10000

	// Erase the 32KB boot block (first boot block)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x80)
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(uint32(bootStart), 0x30)

	// 32KB boot block should be erased
	for i := bootStart; i < bootStart+0x8000; i++ {
		if f.data[i] != 0xFF {
			t.Errorf("boot block data[0x%X] = 0x%02X, want 0xFF", i, f.data[i])
			break
		}
	}

	// 8KB block right after should be untouched
	if f.data[bootStart+0x8000] == 0xFF {
		t.Error("adjacent boot block was erased, should be untouched")
	}
}

func TestFlashResetFromAnyState(t *testing.T) {
	f := makeFlash512KB()

	// Enter CMD1 state
	f.Write(0x5555, 0xAA)
	// Reset with $F0
	f.Write(0x0000, 0xF0)

	// Should be in read mode (normal data returned)
	if got := f.Read(0x05); got != 0x05 {
		t.Errorf("Read after reset = 0x%02X, want 0x05", got)
	}
}

func TestFlashAbortOnBadSequence(t *testing.T) {
	f := makeFlash512KB()

	// Start command sequence but write wrong value
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x77) // invalid command -> abort to READ

	// Should be back in read mode
	if f.state != flashRead {
		t.Errorf("state after bad command = %d, want flashRead", f.state)
	}
}

func TestFlashReset(t *testing.T) {
	f := makeFlash512KB()

	// Enter ID mode
	f.Write(0x5555, 0xAA)
	f.Write(0x2AAA, 0x55)
	f.Write(0x5555, 0x90)

	f.Reset()

	if f.state != flashRead {
		t.Errorf("state after Reset() = %d, want flashRead", f.state)
	}
}

func TestFlashEraseBlockDirect(t *testing.T) {
	f := makeFlash512KB()

	// Verify data exists in block 1
	if f.data[0x10000] == 0xFF {
		t.Fatal("test data should not be 0xFF before erase")
	}

	f.EraseBlock(0x10000)

	// Block 1 (0x10000-0x1FFFF) should be all 0xFF
	for i := 0x10000; i < 0x20000; i++ {
		if f.data[i] != 0xFF {
			t.Errorf("data[0x%X] = 0x%02X after EraseBlock, want 0xFF", i, f.data[i])
			break
		}
	}

	// Adjacent blocks should be untouched
	if f.data[0x00100] == 0xFF {
		t.Error("block 0 data was erased, should be untouched")
	}
	if f.data[0x20000] == 0xFF {
		t.Error("block 2 first byte was erased, should be untouched")
	}
}

func TestFlash2MBBlockBounds(t *testing.T) {
	f := makeFlash2MB()

	// Main block at offset 0
	start, end := f.blockBounds(0x100)
	if start != 0 || end != 0x10000 {
		t.Errorf("main block 0: got [0x%X, 0x%X), want [0x0, 0x10000)", start, end)
	}

	// Boot block region starts at 2MB - 64KB = 0x1F0000
	// 32KB boot block
	start, end = f.blockBounds(0x1F0000)
	if start != 0x1F0000 || end != 0x1F8000 {
		t.Errorf("boot block 32KB: got [0x%X, 0x%X), want [0x1F0000, 0x1F8000)", start, end)
	}

	// 8KB boot block
	start, end = f.blockBounds(0x1F8000)
	if start != 0x1F8000 || end != 0x1FA000 {
		t.Errorf("boot block 8KB: got [0x%X, 0x%X), want [0x1F8000, 0x1FA000)", start, end)
	}

	// Second 8KB boot block
	start, end = f.blockBounds(0x1FA000)
	if start != 0x1FA000 || end != 0x1FC000 {
		t.Errorf("boot block 8KB #2: got [0x%X, 0x%X), want [0x1FA000, 0x1FC000)", start, end)
	}

	// 16KB boot block (top)
	start, end = f.blockBounds(0x1FC000)
	if start != 0x1FC000 || end != 0x200000 {
		t.Errorf("boot block 16KB: got [0x%X, 0x%X), want [0x1FC000, 0x200000)", start, end)
	}
}
