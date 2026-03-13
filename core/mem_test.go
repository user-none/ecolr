package core

import (
	"testing"

	"github.com/user-none/ecolr/core/tlcs900h"
)

// makeTestMemory creates a Memory with a minimal BIOS containing a reset vector
// at $FFFF00 pointing to $FF0010.
func makeTestMemory(t *testing.T) *Memory {
	t.Helper()
	bios := make([]byte, biosROMSize)
	// Reset vector at offset 0xFF00 within the BIOS (address $FFFF00).
	// LE: 0x10, 0x00, 0xFF, 0x00 -> $00FF0010
	bios[0xFF00] = 0x10
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00
	m, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("makeTestMemory: %v", err)
	}
	m.SetBIOS(bios)
	return m
}

// makeTestCart creates a 256-byte cartridge with ascending byte values.
func makeTestCart(t *testing.T) []byte {
	t.Helper()
	cart := make([]byte, 256)
	for i := range cart {
		cart[i] = uint8(i)
	}
	return cart
}

func TestNewMemoryValidation(t *testing.T) {
	t.Run("cart too large", func(t *testing.T) {
		_, err := NewMemory(make([]byte, maxCartSize+1), nil)
		if err == nil {
			t.Fatal("expected error for oversized cart")
		}
	})
	t.Run("cart empty", func(t *testing.T) {
		_, err := NewMemory(make([]byte, 0), nil)
		if err == nil {
			t.Fatal("expected error for empty cart")
		}
	})
	t.Run("valid no cart", func(t *testing.T) {
		_, err := NewMemory(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestBIOSRead(t *testing.T) {
	m := makeTestMemory(t)

	// Byte read from BIOS
	got := m.Read(tlcs900h.Byte, 0xFF0000)
	if got != 0 {
		t.Errorf("BIOS byte read at 0xFF0000 = 0x%02X, want 0x00", got)
	}

	// Read byte from reset vector area
	got = m.Read(tlcs900h.Byte, 0xFFFF00)
	if got != 0x10 {
		t.Errorf("BIOS byte at 0xFFFF00 = 0x%02X, want 0x10", got)
	}

	// Word read (LE)
	got = m.Read(tlcs900h.Word, 0xFFFF00)
	if got != 0x0010 {
		t.Errorf("BIOS word at 0xFFFF00 = 0x%04X, want 0x0010", got)
	}

	// Long read (LE)
	got = m.Read(tlcs900h.Long, 0xFFFF00)
	if got != 0x00FF0010 {
		t.Errorf("BIOS long at 0xFFFF00 = 0x%08X, want 0x00FF0010", got)
	}
}

func TestBIOSReadOnly(t *testing.T) {
	m := makeTestMemory(t)
	m.Write(tlcs900h.Byte, 0xFFFF00, 0xAA)
	got := m.Read(tlcs900h.Byte, 0xFFFF00)
	if got != 0x10 {
		t.Errorf("BIOS write should be ignored: got 0x%02X, want 0x10", got)
	}
}

func TestResetVector(t *testing.T) {
	m := makeTestMemory(t)
	got := m.Read(tlcs900h.Long, 0xFFFF00)
	if got != 0x00FF0010 {
		t.Errorf("reset vector = 0x%08X, want 0x00FF0010", got)
	}
}

func TestWorkRAM(t *testing.T) {
	m := makeTestMemory(t)

	// Byte
	m.Write(tlcs900h.Byte, 0x4000, 0xAB)
	if got := m.Read(tlcs900h.Byte, 0x4000); got != 0xAB {
		t.Errorf("workRAM byte = 0x%02X, want 0xAB", got)
	}

	// Word (LE)
	m.Write(tlcs900h.Word, 0x4010, 0xBEEF)
	if got := m.Read(tlcs900h.Word, 0x4010); got != 0xBEEF {
		t.Errorf("workRAM word = 0x%04X, want 0xBEEF", got)
	}
	// Verify LE byte decomposition
	if got := m.Read(tlcs900h.Byte, 0x4010); got != 0xEF {
		t.Errorf("workRAM word low byte = 0x%02X, want 0xEF", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x4011); got != 0xBE {
		t.Errorf("workRAM word high byte = 0x%02X, want 0xBE", got)
	}

	// Long (LE)
	m.Write(tlcs900h.Long, 0x4020, 0xDEADBEEF)
	if got := m.Read(tlcs900h.Long, 0x4020); got != 0xDEADBEEF {
		t.Errorf("workRAM long = 0x%08X, want 0xDEADBEEF", got)
	}
}

func TestZ80RAM(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0x7000, 0x42)
	if got := m.Read(tlcs900h.Byte, 0x7000); got != 0x42 {
		t.Errorf("z80RAM byte = 0x%02X, want 0x42", got)
	}

	m.Write(tlcs900h.Word, 0x7010, 0x1234)
	if got := m.Read(tlcs900h.Word, 0x7010); got != 0x1234 {
		t.Errorf("z80RAM word = 0x%04X, want 0x1234", got)
	}

	m.Write(tlcs900h.Long, 0x7020, 0xCAFEBABE)
	if got := m.Read(tlcs900h.Long, 0x7020); got != 0xCAFEBABE {
		t.Errorf("z80RAM long = 0x%08X, want 0xCAFEBABE", got)
	}
}

func TestK2GE(t *testing.T) {
	m := makeTestMemory(t)

	// $8000 is masked to bits 7-6 on both write and read
	m.Write(tlcs900h.Byte, 0x8000, 0xC0)
	if got := m.Read(tlcs900h.Byte, 0x8000); got != 0xC0 {
		t.Errorf("k2ge byte = 0x%02X, want 0xC0", got)
	}

	m.Write(tlcs900h.Word, 0x8032, 0xABCD)
	if got := m.Read(tlcs900h.Word, 0x8032); got != 0xABCD {
		t.Errorf("k2ge word = 0x%04X, want 0xABCD", got)
	}

	m.Write(tlcs900h.Long, 0x8020, 0x11223344)
	if got := m.Read(tlcs900h.Long, 0x8020); got != 0x11223344 {
		t.Errorf("k2ge long = 0x%08X, want 0x11223344", got)
	}
}

func TestSFR(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0x20, 0x55)
	if got := m.Read(tlcs900h.Byte, 0x20); got != 0x55 {
		t.Errorf("SFR byte = 0x%02X, want 0x55", got)
	}
}

func TestCustomIO(t *testing.T) {
	m := makeTestMemory(t)

	// $80 is the clock gear register (clamped to 0-4), test a different address
	m.Write(tlcs900h.Byte, 0x81, 0xAA)
	if got := m.Read(tlcs900h.Byte, 0x81); got != 0xAA {
		t.Errorf("customIO byte = 0x%02X, want 0xAA", got)
	}

	m.Write(tlcs900h.Byte, 0xFF, 0xBB)
	if got := m.Read(tlcs900h.Byte, 0xFF); got != 0xBB {
		t.Errorf("customIO byte at 0xFF = 0x%02X, want 0xBB", got)
	}
}

func TestClockGear(t *testing.T) {
	m := makeTestMemory(t)

	// Default gear is 0 (divisor 1)
	if got := m.ClockGearDivisor(); got != 1 {
		t.Errorf("default ClockGearDivisor = %d, want 1", got)
	}

	// Set gear 1 (divisor 2)
	m.Write(tlcs900h.Byte, 0x80, 1)
	if got := m.ClockGearDivisor(); got != 2 {
		t.Errorf("gear 1 ClockGearDivisor = %d, want 2", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x80); got != 1 {
		t.Errorf("gear 1 read back = %d, want 1", got)
	}

	// Set gear 4 (divisor 16)
	m.Write(tlcs900h.Byte, 0x80, 4)
	if got := m.ClockGearDivisor(); got != 16 {
		t.Errorf("gear 4 ClockGearDivisor = %d, want 16", got)
	}

	// Values > 4 are clamped to 4
	m.Write(tlcs900h.Byte, 0x80, 0xFF)
	if got := m.ClockGearDivisor(); got != 16 {
		t.Errorf("gear 0xFF clamped ClockGearDivisor = %d, want 16", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x80); got != 4 {
		t.Errorf("gear 0xFF clamped read back = %d, want 4", got)
	}
}

func TestADRegisters(t *testing.T) {
	m := makeTestMemory(t)

	// Before conversion, result registers return zero-based packing.
	// ADREG low byte: (0<<6)|0x3F = 0x3F, high byte: 0>>2 = 0x00
	if got := m.Read(tlcs900h.Byte, 0x60); got != 0x3F {
		t.Errorf("A/D low before conversion = 0x%02X, want 0x3F", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x61); got != 0x00 {
		t.Errorf("A/D high before conversion = 0x%02X, want 0x00", got)
	}

	// Start A/D conversion on ch0 (AN0 defaults to $3FF) and complete it.
	c := tlcs900h.New(m)
	m.SetCPU(c)
	m.Write(tlcs900h.Byte, 0x6D, 0x04) // START bit
	c.AddCycles(160)
	m.Tick()

	// After conversion, result should reflect $3FF.
	// Low byte: ($3FF<<6)|0x3F = 0xFF, high byte: $3FF>>2 = 0xFF
	if got := m.Read(tlcs900h.Byte, 0x60); got != 0xFF {
		t.Errorf("A/D low after conversion = 0x%02X, want 0xFF", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x61); got != 0xFF {
		t.Errorf("A/D high after conversion = 0x%02X, want 0xFF", got)
	}
}

func TestInputPort(t *testing.T) {
	m := makeTestMemory(t)

	// Default: no buttons pressed, all bits zero (active-high)
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x00 {
		t.Errorf("input port default = 0x%02X, want 0x00", got)
	}

	// SetInput with Up pressed: bit 0 high (active-high)
	m.SetInput(0x01) // bit 0 = 1 (Up pressed)
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x01 {
		t.Errorf("input port with Up = 0x%02X, want 0x01", got)
	}

	// SetInput with A pressed: bit 4 high
	m.SetInput(0x10) // bit 4 = 1 (A pressed)
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x10 {
		t.Errorf("input port with A = 0x%02X, want 0x10", got)
	}

	// SetInput with multiple buttons: Up+Right+B (bits 0,3,5 high)
	m.SetInput(0x29) // 1|8|32 = 0x29
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x29 {
		t.Errorf("input port with Up+Right+B = 0x%02X, want 0x29", got)
	}

	// All buttons pressed (bits 0-6)
	m.SetInput(0x7F) // all buttons pressed
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x7F {
		t.Errorf("input port all pressed = 0x%02X, want 0x7F", got)
	}

	// Reset restores 0x00
	m.Reset()
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x00 {
		t.Errorf("input port after reset = 0x%02X, want 0x00", got)
	}
}

func TestInputMapping(t *testing.T) {
	m := makeTestMemory(t)

	tests := []struct {
		name    string
		emucore uint8 // emucore button bitmask (active-high)
		wantB0  uint8 // expected NGPC $B0 value (active-high)
	}{
		{"no buttons", 0x00, 0x00},
		{"Up", 0x01, 0x01},
		{"Down", 0x02, 0x02},
		{"Left", 0x04, 0x04},
		{"Right", 0x08, 0x08},
		{"A", 0x10, 0x10},
		{"B", 0x20, 0x20},
		{"Option (bit 7->6)", 0x80, 0x40},
		{"Up+A", 0x11, 0x11},
		{"all buttons", 0xBF, 0x7F},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Replicate the emulator SetInput conversion
			low6 := tc.emucore & 0x3F
			option := (tc.emucore >> 7) & 1
			packed := low6 | (option << 6)
			m.SetInput(packed)
			if got := m.Read(tlcs900h.Byte, 0xB0); got != uint32(tc.wantB0) {
				t.Errorf("$B0 = 0x%02X, want 0x%02X", got, tc.wantB0)
			}
		})
	}
}

func TestPowerStatus(t *testing.T) {
	m := makeTestMemory(t)

	if got := m.Read(tlcs900h.Byte, 0xB1); got != 0x02 {
		t.Errorf("power status = 0x%02X, want 0x02", got)
	}
}

func TestReadOnlyRegisters(t *testing.T) {
	m := makeTestMemory(t)

	// Write to A/D register, should be ignored (reads back zero-result packing)
	m.Write(tlcs900h.Byte, 0x60, 0x00)
	if got := m.Read(tlcs900h.Byte, 0x60); got != 0x3F {
		t.Errorf("A/D low after write = 0x%02X, want 0x3F", got)
	}

	// Write to input port, should be ignored
	m.Write(tlcs900h.Byte, 0xB0, 0x55)
	if got := m.Read(tlcs900h.Byte, 0xB0); got != 0x00 {
		t.Errorf("input port after write = 0x%02X, want 0x00", got)
	}

	// Write to power status, should be ignored
	m.Write(tlcs900h.Byte, 0xB1, 0x00)
	if got := m.Read(tlcs900h.Byte, 0xB1); got != 0x02 {
		t.Errorf("power status after write = 0x%02X, want 0x02", got)
	}
}

func TestCartCS0(t *testing.T) {
	cart := makeTestCart(t)
	m, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	// Byte read
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0x00 {
		t.Errorf("cart CS0 byte at 0 = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x200005); got != 0x05 {
		t.Errorf("cart CS0 byte at 5 = 0x%02X, want 0x05", got)
	}

	// Word read (LE)
	if got := m.Read(tlcs900h.Word, 0x200000); got != 0x0100 {
		t.Errorf("cart CS0 word = 0x%04X, want 0x0100", got)
	}
}

func TestCartCS1(t *testing.T) {
	// CS1 maps $800000+ to cart offset $200000+.
	// Need a cart large enough to have data at that offset.
	cart := make([]byte, 0x200010)
	cart[0x200000] = 0xAA
	cart[0x200001] = 0xBB

	m, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	if got := m.Read(tlcs900h.Byte, 0x800000); got != 0xAA {
		t.Errorf("cart CS1 byte = 0x%02X, want 0xAA", got)
	}
	if got := m.Read(tlcs900h.Word, 0x800000); got != 0xBBAA {
		t.Errorf("cart CS1 word = 0x%04X, want 0xBBAA", got)
	}
}

func TestCartReadOnly(t *testing.T) {
	cart := makeTestCart(t)
	m, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	m.Write(tlcs900h.Byte, 0x200000, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0x00 {
		t.Errorf("cart write should be ignored: got 0x%02X, want 0x00", got)
	}
}

func TestCartNil(t *testing.T) {
	m := makeTestMemory(t)
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0 {
		t.Errorf("nil cart read = 0x%02X, want 0x00", got)
	}
}

func TestCartFlashIDMode(t *testing.T) {
	cart := make([]byte, 512*1024)
	cart[0] = 0x42
	m, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	// Normal read returns cart data
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0x42 {
		t.Errorf("cart normal read = 0x%02X, want 0x42", got)
	}

	// Enter flash ID mode via command sequence (CPU addresses)
	m.Write(tlcs900h.Byte, 0x205555, 0xAA)
	m.Write(tlcs900h.Byte, 0x202AAA, 0x55)
	m.Write(tlcs900h.Byte, 0x205555, 0x90)

	// Read manufacturer ID at CS0 base
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0x98 {
		t.Errorf("flash manufacturer ID = 0x%02X, want 0x98", got)
	}
	// Read device ID
	if got := m.Read(tlcs900h.Byte, 0x200001); got != 0xAB {
		t.Errorf("flash device ID = 0x%02X, want 0xAB", got)
	}

	// Exit ID mode
	m.Write(tlcs900h.Byte, 0x200000, 0xF0)

	// Back to normal data
	if got := m.Read(tlcs900h.Byte, 0x200000); got != 0x42 {
		t.Errorf("cart read after ID exit = 0x%02X, want 0x42", got)
	}
}

func TestUnmappedAddresses(t *testing.T) {
	m := makeTestMemory(t)

	// Address in gap between I/O and workRAM
	if got := m.Read(tlcs900h.Byte, 0x1000); got != 0 {
		t.Errorf("unmapped read at 0x1000 = 0x%02X, want 0x00", got)
	}
	// Address in gap between k2ge and cart
	if got := m.Read(tlcs900h.Byte, 0x100000); got != 0 {
		t.Errorf("unmapped read at 0x100000 = 0x%02X, want 0x00", got)
	}
}

func TestResetClearsRAM(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0x4000, 0xAB)
	m.Write(tlcs900h.Byte, 0x7000, 0xCD)
	m.Write(tlcs900h.Byte, 0x8000, 0xEF)
	m.Write(tlcs900h.Byte, 0x20, 0x55)
	m.Write(tlcs900h.Byte, 0x80, 0xAA)

	m.Reset()

	if got := m.Read(tlcs900h.Byte, 0x4000); got != 0 {
		t.Errorf("workRAM after reset = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x7000); got != 0 {
		t.Errorf("z80RAM after reset = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x8000); got != 0 {
		t.Errorf("k2ge after reset = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x20); got != 0 {
		t.Errorf("SFR after reset = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x80); got != 0 {
		t.Errorf("customIO after reset = 0x%02X, want 0x00", got)
	}

	// BIOS should be preserved
	if got := m.Read(tlcs900h.Byte, 0xFFFF00); got != 0x10 {
		t.Errorf("BIOS after reset = 0x%02X, want 0x10", got)
	}
}

func TestMultiByteIORead(t *testing.T) {
	m := makeTestMemory(t)

	// Before conversion, word read at $60 assembles $60=0x3F and $61=0x00 in LE
	got := m.Read(tlcs900h.Word, 0x60)
	want := uint32(0x003F)
	if got != want {
		t.Errorf("word read at I/O $60 = 0x%04X, want 0x%04X", got, want)
	}
}

func TestCPUIntegration(t *testing.T) {
	m := makeTestMemory(t)
	c := tlcs900h.New(m)
	c.LoadResetVector()
	regs := c.Registers()

	// Reset vector at $FFFF00 contains $00FF0010, masked to 24-bit = $FF0010
	if regs.PC != 0xFF0010 {
		t.Errorf("CPU PC after reset = 0x%06X, want 0xFF0010", regs.PC)
	}
}

func TestSoundChipEnable(t *testing.T) {
	m := makeTestMemory(t)

	// Default is $AA (off)
	if got := m.Read(tlcs900h.Byte, 0xB8); got != 0xAA {
		t.Errorf("soundChipEn default = 0x%02X, want 0xAA", got)
	}

	// Write $55 (on), read back
	m.Write(tlcs900h.Byte, 0xB8, 0x55)
	if got := m.Read(tlcs900h.Byte, 0xB8); got != 0x55 {
		t.Errorf("soundChipEn after $55 = 0x%02X, want 0x55", got)
	}

	// Write $AA (off), read back
	m.Write(tlcs900h.Byte, 0xB8, 0xAA)
	if got := m.Read(tlcs900h.Byte, 0xB8); got != 0xAA {
		t.Errorf("soundChipEn after $AA = 0x%02X, want 0xAA", got)
	}
}

func TestZ80Activation(t *testing.T) {
	m := makeTestMemory(t)

	// Default is $AA (off)
	if got := m.Read(tlcs900h.Byte, 0xB9); got != 0xAA {
		t.Errorf("z80Active default = 0x%02X, want 0xAA", got)
	}

	// Write $55 (on), read back
	m.Write(tlcs900h.Byte, 0xB9, 0x55)
	if got := m.Read(tlcs900h.Byte, 0xB9); got != 0x55 {
		t.Errorf("z80Active after $55 = 0x%02X, want 0x55", got)
	}

	// Write $AA (off), read back
	m.Write(tlcs900h.Byte, 0xB9, 0xAA)
	if got := m.Read(tlcs900h.Byte, 0xB9); got != 0xAA {
		t.Errorf("z80Active after $AA = 0x%02X, want 0xAA", got)
	}
}

func TestT6W28WriteGate(t *testing.T) {
	psg := NewT6W28(3072000, 48000, 2048)
	m, err := NewMemory(nil, psg)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	// Gate closed by default ($B8=$AA, $B9=$AA): write should not reach PSG
	m.Write(tlcs900h.Byte, 0xA0, 0x80)
	m.Write(tlcs900h.Byte, 0xA1, 0x80)

	// Enable sound chip ($B8=$55) but keep Z80 inactive ($B9=$AA): gate open
	m.Write(tlcs900h.Byte, 0xB8, 0x55)
	m.Write(tlcs900h.Byte, 0xA0, 0x9F) // max vol ch0 right
	m.Write(tlcs900h.Byte, 0xA1, 0x9F) // max vol ch0 left

	// Activate Z80 ($B9=$55): gate should close (Z80 owns PSG now)
	m.Write(tlcs900h.Byte, 0xB9, 0x55)
	m.Write(tlcs900h.Byte, 0xA0, 0x80)
	m.Write(tlcs900h.Byte, 0xA1, 0x80)

	// Values still stored in customIO regardless of gate
	if got := m.Read(tlcs900h.Byte, 0xA0); got != 0x80 {
		t.Errorf("$A0 customIO = 0x%02X, want 0x80", got)
	}
	if got := m.Read(tlcs900h.Byte, 0xA1); got != 0x80 {
		t.Errorf("$A1 customIO = 0x%02X, want 0x80", got)
	}
}

func TestNMIEnable(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0xB3, 0x04) // bit 2 = power button NMI enable
	if got := m.Read(tlcs900h.Byte, 0xB3); got != 0x04 {
		t.Errorf("NMI enable = 0x%02X, want 0x04", got)
	}

	m.Write(tlcs900h.Byte, 0xB3, 0x00)
	if got := m.Read(tlcs900h.Byte, 0xB3); got != 0x00 {
		t.Errorf("NMI enable after clear = 0x%02X, want 0x00", got)
	}
}

func TestCommByte(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0xBC, 0x42)
	if got := m.Read(tlcs900h.Byte, 0xBC); got != 0x42 {
		t.Errorf("comm byte = 0x%02X, want 0x42", got)
	}
}

func TestRTSState(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0xB2, 0x01)
	if got := m.Read(tlcs900h.Byte, 0xB2); got != 0x01 {
		t.Errorf("RTS state = 0x%02X, want 0x01", got)
	}

	m.Write(tlcs900h.Byte, 0xB2, 0x00)
	if got := m.Read(tlcs900h.Byte, 0xB2); got != 0x00 {
		t.Errorf("RTS state after clear = 0x%02X, want 0x00", got)
	}
}

func TestZ80NMITrigger(t *testing.T) {
	m := makeTestMemory(t)

	// With Z80 inactive, write to $BA should be a no-op
	m.Write(tlcs900h.Byte, 0xBA, 0xFF)
	regs := m.z80cpu.Registers()
	if regs.PC != 0 {
		t.Errorf("Z80 PC after NMI while inactive = 0x%04X, want 0x0000", regs.PC)
	}

	// Enable Z80
	m.Write(tlcs900h.Byte, 0xB9, 0x55)

	// Put HALT at $0000 and RETN at $0066 (NMI vector)
	m.z80RAM[0x0000] = 0x76 // HALT
	m.z80RAM[0x0066] = 0xED // RETN prefix
	m.z80RAM[0x0067] = 0x45 // RETN

	// Step Z80 into HALT
	m.z80cpu.Step()
	regs = m.z80cpu.Registers()
	if !regs.Halted {
		t.Fatal("Z80 should be halted after executing HALT")
	}

	// Trigger NMI via $BA write
	m.Write(tlcs900h.Byte, 0xBA, 0xFF)

	// Step to service NMI - should jump to $0066
	m.z80cpu.Step()
	regs = m.z80cpu.Registers()
	if regs.PC != 0x0066 {
		t.Errorf("Z80 PC after NMI = 0x%04X, want 0x0066", regs.PC)
	}

	// Read returns 0 (not stored)
	if got := m.Read(tlcs900h.Byte, 0xBA); got != 0x00 {
		t.Errorf("Z80 NMI read = 0x%02X, want 0x00", got)
	}
}

func TestZ80ResetOnEnable(t *testing.T) {
	m := makeTestMemory(t)

	// Put a NOP+HALT sequence in Z80 RAM and step to advance PC
	m.z80RAM[0x0000] = 0x00 // NOP
	m.z80RAM[0x0001] = 0x76 // HALT
	m.z80cpu.Step()         // execute NOP, PC now at $0001

	regs := m.z80cpu.Registers()
	if regs.PC != 0x0001 {
		t.Fatalf("Z80 PC after NOP = 0x%04X, want 0x0001", regs.PC)
	}

	// Write $55 to $B9 should reset Z80 with zeroed registers
	m.Write(tlcs900h.Byte, 0xB9, 0x55)

	regs = m.z80cpu.Registers()
	if regs.PC != 0 {
		t.Errorf("Z80 PC after enable reset = 0x%04X, want 0x0000", regs.PC)
	}
	if regs.SP != 0 {
		t.Errorf("Z80 SP after enable reset = 0x%04X, want 0x0000", regs.SP)
	}
	if regs.AF != 0 {
		t.Errorf("Z80 AF after enable reset = 0x%04X, want 0x0000", regs.AF)
	}
}

func TestZ80FrameExecution(t *testing.T) {
	m := makeTestMemory(t)

	// Place HALT ($76) at Z80 RAM $0000
	m.z80RAM[0x0000] = 0x76

	// Enable Z80
	m.Write(tlcs900h.Byte, 0xB9, 0x55)

	// Step Z80 - should execute HALT and enter halted state
	m.z80cpu.Step()

	regs := m.z80cpu.Registers()
	if !regs.Halted {
		t.Error("Z80 should be halted after executing HALT at $0000")
	}
}

func TestDACRegisters(t *testing.T) {
	m := makeTestMemory(t)

	// Write to DAC L/R registers
	m.Write(tlcs900h.Byte, 0xA2, 0x42)
	m.Write(tlcs900h.Byte, 0xA3, 0xBE)

	if got := m.Read(tlcs900h.Byte, 0xA2); got != 0x42 {
		t.Errorf("DAC L = 0x%02X, want 0x42", got)
	}
	if got := m.Read(tlcs900h.Byte, 0xA3); got != 0xBE {
		t.Errorf("DAC R = 0x%02X, want 0xBE", got)
	}
}

func TestDACReset(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0xA2, 0xFF)
	m.Write(tlcs900h.Byte, 0xA3, 0xFF)
	m.Reset()

	if got := m.Read(tlcs900h.Byte, 0xA2); got != 0x80 {
		t.Errorf("DAC L after reset = 0x%02X, want 0x80", got)
	}
	if got := m.Read(tlcs900h.Byte, 0xA3); got != 0x80 {
		t.Errorf("DAC R after reset = 0x%02X, want 0x80", got)
	}
}

func TestDACAccessor(t *testing.T) {
	m := makeTestMemory(t)

	m.Write(tlcs900h.Byte, 0xA2, 0x10)
	m.Write(tlcs900h.Byte, 0xA3, 0x20)

	l, r := m.DACValues()
	if l != 0x10 {
		t.Errorf("DACValues L = 0x%02X, want 0x10", l)
	}
	if r != 0x20 {
		t.Errorf("DACValues R = 0x%02X, want 0x20", r)
	}
}

func TestRequestINT4(t *testing.T) {
	m := makeTestMemory(t)

	// INT4 is reg 1 (INTE45), low source (mask 0x08).
	// Set INT4 priority to 4.
	m.intc.WriteReg(1, 0x04) // low nibble = priority 4
	m.RequestINT4()

	got := m.intc.ReadReg(1)
	if got&0x08 == 0 {
		t.Error("INT4 pending bit should be set after RequestINT4")
	}
}

func TestSetVBlankStatus(t *testing.T) {
	m := makeTestMemory(t)

	// Initially bit 6 of $8010 should be clear.
	if got := m.Read(tlcs900h.Byte, 0x8010); got&0x40 != 0 {
		t.Errorf("VBlank status should be clear initially, got 0x%02X", got)
	}

	// Set VBlank active.
	m.SetVBlankStatus(true)
	if got := m.Read(tlcs900h.Byte, 0x8010); got&0x40 == 0 {
		t.Errorf("VBlank status should be set, got 0x%02X", got)
	}

	// Clear VBlank.
	m.SetVBlankStatus(false)
	if got := m.Read(tlcs900h.Byte, 0x8010); got&0x40 != 0 {
		t.Errorf("VBlank status should be clear, got 0x%02X", got)
	}
}

func TestSetVBlankStatusPreservesOtherBits(t *testing.T) {
	m := makeTestMemory(t)

	// Set bit 7 directly (CPU write to $8010 is now blocked as read-only).
	m.k2ge[0x0010] = 0x80
	m.SetVBlankStatus(true)

	got := m.Read(tlcs900h.Byte, 0x8010)
	if got != 0xC0 {
		t.Errorf("$8010 = 0x%02X, want 0xC0 (bits 7 and 6 set)", got)
	}

	// SetVBlankStatus(false) now clears both bit 6 (BLNK) and bit 7 (C.OVR).
	m.SetVBlankStatus(false)
	got = m.Read(tlcs900h.Byte, 0x8010)
	if got != 0x00 {
		t.Errorf("$8010 = 0x%02X, want 0x00 (C.OVR cleared at VBlank end)", got)
	}
}

func TestSoundControlReset(t *testing.T) {
	m := makeTestMemory(t)

	// Enable sound and Z80
	m.Write(tlcs900h.Byte, 0xB8, 0x55)
	m.Write(tlcs900h.Byte, 0xB9, 0x55)

	m.Reset()

	if got := m.Read(tlcs900h.Byte, 0xB8); got != 0xAA {
		t.Errorf("soundChipEn after reset = 0x%02X, want 0xAA", got)
	}
	if got := m.Read(tlcs900h.Byte, 0xB9); got != 0xAA {
		t.Errorf("z80Active after reset = 0x%02X, want 0xAA", got)
	}
}

func TestK2GEReadOnlyRegisters(t *testing.T) {
	m := makeTestMemory(t)

	// $8006 (REF) is set to $C6 by resetK2GERegisters; write should be dropped
	m.Write(tlcs900h.Byte, 0x8006, 0x00)
	if got := m.Read(tlcs900h.Byte, 0x8006); got != 0xC6 {
		t.Errorf("$8006 after write = 0x%02X, want 0xC6 (read-only)", got)
	}

	// $8008 (RAS.H) defaults to 0
	m.Write(tlcs900h.Byte, 0x8008, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8008); got != 0x00 {
		t.Errorf("$8008 after write = 0x%02X, want 0x00 (read-only)", got)
	}

	// $8009 (RAS.V) defaults to 0
	m.Write(tlcs900h.Byte, 0x8009, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8009); got != 0x00 {
		t.Errorf("$8009 after write = 0x%02X, want 0x00 (read-only)", got)
	}

	// $8010 (status) defaults to 0
	m.Write(tlcs900h.Byte, 0x8010, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8010); got != 0x00 {
		t.Errorf("$8010 after write = 0x%02X, want 0x00 (read-only)", got)
	}
}

func TestK2GEModeLock(t *testing.T) {
	m := makeTestMemory(t)

	// Default: mode locked, write to $87E2 should be blocked
	m.Write(tlcs900h.Byte, 0x87E2, 0x80)
	if got := m.Read(tlcs900h.Byte, 0x87E2); got != 0x00 {
		t.Errorf("$87E2 while locked = 0x%02X, want 0x00", got)
	}

	// Unlock with $AA to $87F0
	m.Write(tlcs900h.Byte, 0x87F0, 0xAA)
	m.Write(tlcs900h.Byte, 0x87E2, 0x80)
	if got := m.Read(tlcs900h.Byte, 0x87E2); got != 0x80 {
		t.Errorf("$87E2 after unlock = 0x%02X, want 0x80", got)
	}

	// Lock again with $55
	m.Write(tlcs900h.Byte, 0x87F0, 0x55)
	m.Write(tlcs900h.Byte, 0x87E2, 0x00)
	if got := m.Read(tlcs900h.Byte, 0x87E2); got != 0x80 {
		t.Errorf("$87E2 after re-lock = 0x%02X, want 0x80 (unchanged)", got)
	}

	// $87F0 is write-only, should read 0
	if got := m.Read(tlcs900h.Byte, 0x87F0); got != 0x00 {
		t.Errorf("$87F0 read = 0x%02X, want 0x00 (write-only)", got)
	}
}

func TestK2GESoftwareReset(t *testing.T) {
	m := makeTestMemory(t)

	// Write values to writable registers
	m.Write(tlcs900h.Byte, 0x8002, 0x42)
	m.Write(tlcs900h.Byte, 0x8004, 0x33)

	// Trigger software reset by writing $52 to $87E0
	m.Write(tlcs900h.Byte, 0x87E0, 0x52)

	// Registers should return to reset defaults
	if got := m.Read(tlcs900h.Byte, 0x8002); got != 0x00 {
		t.Errorf("$8002 after reset = 0x%02X, want 0x00", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x8004); got != 0xFF {
		t.Errorf("$8004 after reset = 0x%02X, want 0xFF (WSI.H default)", got)
	}
	if got := m.Read(tlcs900h.Byte, 0x8006); got != 0xC6 {
		t.Errorf("$8006 after reset = 0x%02X, want 0xC6 (REF default)", got)
	}

	// $87E0 is write-only, should read 0
	if got := m.Read(tlcs900h.Byte, 0x87E0); got != 0x00 {
		t.Errorf("$87E0 read = 0x%02X, want 0x00 (write-only)", got)
	}
}

func TestK2GEWordWriteAcrossReadOnly(t *testing.T) {
	m := makeTestMemory(t)

	// Word write at $8010: offset 0x0010 (read-only), offset 0x0011 (writable)
	m.Write(tlcs900h.Word, 0x8010, 0xBEEF)

	// 0x0010 should be unchanged (read-only)
	if got := m.Read(tlcs900h.Byte, 0x8010); got != 0x00 {
		t.Errorf("$8010 = 0x%02X, want 0x00 (read-only)", got)
	}
	// 0x0011 should be written (high byte of LE word = 0xBE)
	if got := m.Read(tlcs900h.Byte, 0x8011); got != 0xBE {
		t.Errorf("$8011 = 0x%02X, want 0xBE (writable)", got)
	}
}

func TestHBlank(t *testing.T) {
	m := makeTestMemory(t)
	c := tlcs900h.New(m)
	m.SetCPU(c)

	// Enable HBlank: set $8000 bit 6
	m.k2ge[0x0000] |= 0x40

	// Configure Timer 0: external clock (T0CLK=00), TREG0=2
	// Need 2 ticks to overflow so first HBlank increments but doesn't fire
	m.Write(tlcs900h.Byte, 0x22, 2)    // TREG0 = 2
	m.Write(tlcs900h.Byte, 0x24, 0x00) // T01MOD: T0CLK=00(external)
	m.Write(tlcs900h.Byte, 0x20, 0x81) // TRUN: T0 run + prescaler

	// Set INTT0 priority so pending bit is visible
	m.intc.WriteReg(3, 0x04) // INTT0 priority=4

	// First HBlank: counter goes to 1, no overflow
	m.HBlank()
	if m.timers.counter8[0] != 1 {
		t.Fatalf("T0 counter should be 1 after first HBlank, got %d", m.timers.counter8[0])
	}

	// Second HBlank: counter reaches TREG0=2, overflow fires INTT0
	m.HBlank()
	// After overflow, counter resets to 0
	if m.timers.counter8[0] != 0 {
		t.Fatalf("T0 counter should be 0 after overflow, got %d", m.timers.counter8[0])
	}
}

func TestHBlankDisabled(t *testing.T) {
	m := makeTestMemory(t)
	c := tlcs900h.New(m)
	m.SetCPU(c)

	// HBlank disabled: $8000 bit 6 clear (default)

	// Configure Timer 0: external clock (T0CLK=00), TREG0=1
	m.Write(tlcs900h.Byte, 0x22, 1)    // TREG0 = 1
	m.Write(tlcs900h.Byte, 0x24, 0x00) // T01MOD: T0CLK=00(external)
	m.Write(tlcs900h.Byte, 0x20, 0x81) // TRUN: T0 run + prescaler

	// Set INTT0 priority
	m.intc.WriteReg(3, 0x04)

	m.HBlank()

	// INTT0 should NOT be pending
	if m.intc.ReadReg(3)&0x08 != 0 {
		t.Fatal("INTT0 should not be pending when HBlank is disabled")
	}
}

func TestVBlankEnabled(t *testing.T) {
	m := makeTestMemory(t)

	// Default: bit 7 clear
	if m.VBlankEnabled() {
		t.Fatal("VBlankEnabled should be false by default")
	}

	// Set $8000 bit 7
	m.k2ge[0x0000] |= 0x80
	if !m.VBlankEnabled() {
		t.Fatal("VBlankEnabled should be true when bit 7 is set")
	}

	// Clear bit 7
	m.k2ge[0x0000] &^= 0x80
	if m.VBlankEnabled() {
		t.Fatal("VBlankEnabled should be false when bit 7 is cleared")
	}
}

func TestSetRasterPosition(t *testing.T) {
	m := makeTestMemory(t)

	m.SetRasterPosition(100, 515)

	if m.k2ge[0x0009] != 100 {
		t.Errorf("RAS.V = %d, want 100", m.k2ge[0x0009])
	}
	if m.k2ge[0x0008] != 128 {
		t.Errorf("RAS.H = %d, want 128 (515>>2)", m.k2ge[0x0008])
	}
}

func TestCOVRClearedAtVBlankEnd(t *testing.T) {
	m := makeTestMemory(t)

	// Set C.OVR (bit 7) directly
	m.k2ge[0x0010] = 0x80
	m.SetVBlankStatus(false)

	if m.k2ge[0x0010]&0x80 != 0 {
		t.Errorf("C.OVR bit 7 = 0x%02X, want cleared", m.k2ge[0x0010])
	}
}

func TestK2GEReadLEDRegisters(t *testing.T) {
	m := makeTestMemory(t)

	// LED_CTL ($8400): bits 2-0 always read as 1
	m.Write(tlcs900h.Byte, 0x8400, 0x00)
	if got := m.Read(tlcs900h.Byte, 0x8400); got != 0x07 {
		t.Errorf("$8400 after write $00 = 0x%02X, want 0x07", got)
	}

	m.Write(tlcs900h.Byte, 0x8400, 0xF8)
	if got := m.Read(tlcs900h.Byte, 0x8400); got != 0xFF {
		t.Errorf("$8400 after write $F8 = 0x%02X, want 0xFF", got)
	}

	// LED_FLC ($8402): reset default is $80
	if got := m.Read(tlcs900h.Byte, 0x8402); got != 0x80 {
		t.Errorf("$8402 reset default = 0x%02X, want 0x80", got)
	}
}

func TestK2GEReadShadeLUTMask(t *testing.T) {
	m := makeTestMemory(t)

	// Shade LUT ($8100-$8117): 3-bit values, bits 7-3 read as 0
	m.Write(tlcs900h.Byte, 0x8100, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8100); got != 0x07 {
		t.Errorf("$8100 after write $FF = 0x%02X, want 0x07", got)
	}

	m.Write(tlcs900h.Byte, 0x8117, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8117); got != 0x07 {
		t.Errorf("$8117 after write $FF = 0x%02X, want 0x07", got)
	}

	m.Write(tlcs900h.Byte, 0x8108, 0xA5)
	if got := m.Read(tlcs900h.Byte, 0x8108); got != 0x05 {
		t.Errorf("$8108 after write $A5 = 0x%02X, want 0x05", got)
	}
}

func TestK2GEReadModeRegisterMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8000: bits 5-0 are unused, only bits 7-6 readable
	m.Write(tlcs900h.Byte, 0x8000, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8000); got != 0xC0 {
		t.Errorf("$8000 after write $FF = 0x%02X, want 0xC0", got)
	}

	m.Write(tlcs900h.Byte, 0x8000, 0xC0)
	if got := m.Read(tlcs900h.Byte, 0x8000); got != 0xC0 {
		t.Errorf("$8000 after write $C0 = 0x%02X, want 0xC0", got)
	}

	m.Write(tlcs900h.Byte, 0x8000, 0x3F)
	if got := m.Read(tlcs900h.Byte, 0x8000); got != 0x00 {
		t.Errorf("$8000 after write $3F = 0x%02X, want 0x00", got)
	}
}

func TestK2GEReadUnmaskedPassthrough(t *testing.T) {
	m := makeTestMemory(t)

	// Unmasked address should pass through unchanged
	m.Write(tlcs900h.Byte, 0x8020, 0xAB)
	if got := m.Read(tlcs900h.Byte, 0x8020); got != 0xAB {
		t.Errorf("$8020 after write $AB = 0x%02X, want 0xAB", got)
	}
}

func TestK2GEReadStatusMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8010 is read-only, write is dropped; raw value is 0
	m.Write(tlcs900h.Byte, 0x8010, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8010); got != 0x00 {
		t.Errorf("$8010 after write $FF = 0x%02X, want 0x00", got)
	}

	// Set VBlank status via API, read back should show bit 6 only
	m.SetVBlankStatus(true)
	if got := m.Read(tlcs900h.Byte, 0x8010); got != 0x40 {
		t.Errorf("$8010 with VBlank = 0x%02X, want 0x40", got)
	}

	// Set all bits directly in backing array, read should mask to bits 7-6
	m.k2ge[0x0010] = 0xFF
	if got := m.Read(tlcs900h.Byte, 0x8010); got != 0xC0 {
		t.Errorf("$8010 with $FF raw = 0x%02X, want 0xC0", got)
	}
}

func TestK2GEReadControlMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8012: bits 7, 6, 3, 2-0 valid (6 and 3 are reserved RAM), bits 5-4 unused
	m.Write(tlcs900h.Byte, 0x8012, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8012); got != 0xCF {
		t.Errorf("$8012 after write $FF = 0x%02X, want 0xCF", got)
	}

	// Writing only truly unused bits (5-4) yields 0
	m.Write(tlcs900h.Byte, 0x8012, 0x30)
	if got := m.Read(tlcs900h.Byte, 0x8012); got != 0x00 {
		t.Errorf("$8012 after write $30 = 0x%02X, want 0x00", got)
	}
}

func TestK2GEReadScrollPriorityMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8030: bit 7 valid, bits 6-0 unused
	m.Write(tlcs900h.Byte, 0x8030, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8030); got != 0x80 {
		t.Errorf("$8030 after write $FF = 0x%02X, want 0x80", got)
	}

	m.Write(tlcs900h.Byte, 0x8030, 0x7F)
	if got := m.Read(tlcs900h.Byte, 0x8030); got != 0x00 {
		t.Errorf("$8030 after write $7F = 0x%02X, want 0x00", got)
	}
}

func TestK2GEReadBGSelectionMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8118: bits 7-6 + 2-0 valid, bits 5-3 reserved
	m.Write(tlcs900h.Byte, 0x8118, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8118); got != 0xC7 {
		t.Errorf("$8118 after write $FF = 0x%02X, want 0xC7", got)
	}

	m.Write(tlcs900h.Byte, 0x8118, 0x38)
	if got := m.Read(tlcs900h.Byte, 0x8118); got != 0x00 {
		t.Errorf("$8118 after write $38 = 0x%02X, want 0x00", got)
	}
}

func TestK2GEReadModeMask(t *testing.T) {
	m := makeTestMemory(t)

	// Unlock mode register
	m.Write(tlcs900h.Byte, 0x87F0, 0xAA)

	// $87E2: bit 7 valid, bits 6-0 read as 0
	m.Write(tlcs900h.Byte, 0x87E2, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x87E2); got != 0x80 {
		t.Errorf("$87E2 after write $FF = 0x%02X, want 0x80", got)
	}

	m.Write(tlcs900h.Byte, 0x87E2, 0x7F)
	if got := m.Read(tlcs900h.Byte, 0x87E2); got != 0x00 {
		t.Errorf("$87E2 after write $7F = 0x%02X, want 0x00", got)
	}
}

func TestK2GEReadSpritePalAsnMask(t *testing.T) {
	m := makeTestMemory(t)

	// $8C00-$8C3F: bits 3-0 valid, bits 7-4 read as 0
	m.Write(tlcs900h.Byte, 0x8C00, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8C00); got != 0x0F {
		t.Errorf("$8C00 after write $FF = 0x%02X, want 0x0F", got)
	}

	m.Write(tlcs900h.Byte, 0x8C3F, 0xFF)
	if got := m.Read(tlcs900h.Byte, 0x8C3F); got != 0x0F {
		t.Errorf("$8C3F after write $FF = 0x%02X, want 0x0F", got)
	}
}

func TestK2GEPaletteWordAccess(t *testing.T) {
	m := makeTestMemory(t)

	// Word write to palette RAM should work
	m.Write(tlcs900h.Word, 0x8200, 0x0FFF)
	if got := m.Read(tlcs900h.Word, 0x8200); got != 0x0FFF {
		t.Errorf("palette word at $8200 = 0x%04X, want 0x0FFF", got)
	}

	// Word write at end of palette range
	m.Write(tlcs900h.Word, 0x83FE, 0x0ABC)
	if got := m.Read(tlcs900h.Word, 0x83FE); got != 0x0ABC {
		t.Errorf("palette word at $83FE = 0x%04X, want 0x0ABC", got)
	}
}

func TestK2GEPaletteByteAccess(t *testing.T) {
	m := makeTestMemory(t)

	// First write a valid word to palette
	m.Write(tlcs900h.Word, 0x8200, 0x0FFF)

	// Byte write should be dropped
	m.Write(tlcs900h.Byte, 0x8200, 0xFF)

	// Byte read should return actual stored value
	if got := m.Read(tlcs900h.Byte, 0x8200); got != 0xFF {
		t.Errorf("palette byte read at $8200 = 0x%02X, want 0xFF", got)
	}

	// Word read should still return the original word value
	if got := m.Read(tlcs900h.Word, 0x8200); got != 0x0FFF {
		t.Errorf("palette word at $8200 after byte write = 0x%04X, want 0x0FFF", got)
	}

	// Byte read at end of palette range (byte write is dropped, so value
	// comes from whatever was previously stored)
	m.Write(tlcs900h.Word, 0x83FE, 0x0ABC)
	if got := m.Read(tlcs900h.Byte, 0x83FF); got != 0x0A {
		t.Errorf("palette byte read at $83FF = 0x%02X, want 0x0A", got)
	}
}

func TestK2GEResetLEDFLC(t *testing.T) {
	m := makeTestMemory(t)

	// Overwrite LED_FLC
	m.Write(tlcs900h.Byte, 0x8402, 0x00)
	if got := m.Read(tlcs900h.Byte, 0x8402); got != 0x00 {
		t.Errorf("$8402 after write $00 = 0x%02X, want 0x00", got)
	}

	// Software reset restores default
	m.Write(tlcs900h.Byte, 0x87E0, 0x52)
	if got := m.Read(tlcs900h.Byte, 0x8402); got != 0x80 {
		t.Errorf("$8402 after reset = 0x%02X, want 0x80", got)
	}
}
