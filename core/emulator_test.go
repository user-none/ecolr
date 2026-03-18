package core

import (
	"testing"

	"github.com/user-none/eblitui/coreif"
	"github.com/user-none/ecolr/core/tlcs900h"
)

// makeTestEmulator creates an Emulator with a minimal BIOS that loops at the
// reset vector. The BIOS contains a JR -2 (infinite loop) at the entry point.
func makeTestEmulator(t *testing.T) Emulator {
	t.Helper()
	bios := make([]byte, biosROMSize)
	// Reset vector at offset 0xFF00 -> address $FFFF00 points to $FF0010
	bios[0xFF00] = 0x10
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00
	// Place JR $-2 at $FF0010 (offset 0x0010) to loop forever.
	// TLCS-900/H JR cc,d8: opcode $C0+cc, displacement.
	// JR always (cc=8): $C8, displacement $FE (-2)
	bios[0x0010] = 0xC8
	bios[0x0011] = 0xFE

	emu, err := NewEmulator(nil)
	if err != nil {
		t.Fatalf("makeTestEmulator: %v", err)
	}
	emu.SetBIOS("system_bios", bios)
	return emu
}

func TestDACMixing(t *testing.T) {
	emu := makeTestEmulator(t)
	emu.Start()

	// Write non-center DAC values via Memory
	emu.mem.Write8(0xA2, 0xFF) // left = max
	emu.mem.Write8(0xA3, 0x00) // right = min

	emu.RunFrame()

	samples := emu.GetAudioSamples()
	if len(samples) == 0 {
		t.Fatal("no audio samples after RunFrame")
	}

	// With DAC at 0xFF (left) and 0x00 (right), and PSG silent,
	// we should see non-zero samples from DAC contribution.
	hasNonZeroL := false
	hasNonZeroR := false
	for i := 0; i < len(samples); i += 2 {
		if samples[i] != 0 {
			hasNonZeroL = true
		}
		if samples[i+1] != 0 {
			hasNonZeroR = true
		}
	}
	if !hasNonZeroL {
		t.Error("left channel should have non-zero samples from DAC at 0xFF")
	}
	if !hasNonZeroR {
		t.Error("right channel should have non-zero samples from DAC at 0x00")
	}
}

func TestDACSilenceAtCenter(t *testing.T) {
	emu := makeTestEmulator(t)
	emu.Start()

	// DAC at center value produces zero contribution
	emu.mem.Write8(0xA2, 0x80)
	emu.mem.Write8(0xA3, 0x80)

	emu.RunFrame()

	samples := emu.GetAudioSamples()
	if len(samples) == 0 {
		t.Fatal("no audio samples after RunFrame")
	}

	// PSG is silent (all volumes at 0x0F) and DAC is at center,
	// so all samples should be zero.
	for i, s := range samples {
		if s != 0 {
			t.Errorf("sample[%d] = %d, want 0 (DAC at center, PSG silent)", i, s)
			break
		}
	}
}

func TestNewEmulatorDefaultNVRAMBoot(t *testing.T) {
	emu := makeTestEmulator(t)
	emu.SetOption("fast_boot", "false")
	emu.Start()

	// PC should be set to NVRAM boot entry $FF1800
	regs := emu.cpu.Registers()
	if regs.PC != 0xFF1800 {
		t.Errorf("PC = 0x%06X, want 0xFF1800 (NVRAM boot path)", regs.PC)
	}
}

func TestSetOptionFirstBootHLE(t *testing.T) {
	// HLE mode: first_boot option should be ignored by Start
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator HLE: %v", err)
	}

	emu.SetOption("first_boot", "true")
	emu.Start()

	// In HLE mode, Start should apply options normally (first_boot ignored).
	// PC should be set to cart entry point by HLE install.
	if got := emu.cpu.Registers().PC; got != 0x200040 {
		t.Errorf("PC = 0x%06X, want 0x200040 (HLE should ignore first_boot)", got)
	}

	// Config marker should still be set (HLE + first_boot = normal path)
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x4E50 {
		t.Errorf("$6E95 = 0x%04X, want 0x4E50 (HLE ignores first_boot)", marker)
	}
}

// makeMinimalCart creates a minimal valid NGPC cartridge for HLE testing.
func makeMinimalCart() []byte {
	cart := make([]byte, 256)
	// Entry point at offset $1C (little-endian $200000 + offset)
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00
	return cart
}

func makeMinimalMonoCart() []byte {
	cart := makeMinimalCart()
	cart[0x23] = 0x00 // system code = monochrome
	return cart
}

func TestSetOptionMonoPaletteBW(t *testing.T) {
	emu, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	emu.SetOption("mono_palette", "Black & White")
	emu.Start()

	// Shade 0 (brightest) should be $0FFF in all K1GE compat areas
	for _, addr := range []uint32{0x8380, 0x83A0, 0x83C0} {
		got := emu.mem.Read16(addr)
		if got != 0x0FFF {
			t.Errorf("$%04X shade 0 = 0x%04X, want 0x0FFF", addr, got)
		}
	}
	// $6F94 should be 0
	if idx := emu.mem.Read8(0x6F94); idx != 0x00 {
		t.Errorf("$6F94 = 0x%02X, want 0x00", idx)
	}
}

func TestSetOptionMonoPaletteBlue(t *testing.T) {
	emu, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	emu.SetOption("mono_palette", "Blue")
	emu.Start()

	// Shade 1 for Blue: $0FCC from BIOS ROM (R=C, G=C, B=F in hardware format)
	got := emu.mem.Read16(0x8382)
	if got != 0x0FCC {
		t.Errorf("$8382 shade 1 = 0x%04X, want 0x0FCC (Blue)", got)
	}
	// $6F94 should be 3 (Blue index)
	if idx := emu.mem.Read8(0x6F94); idx != 0x03 {
		t.Errorf("$6F94 = 0x%02X, want 0x03", idx)
	}
}

func TestSetOptionMonoPaletteAllSchemes(t *testing.T) {
	emu, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	for _, pal := range monoPalettes {
		emu.SetOption("mono_palette", pal.Name)
		emu.Start()

		// Check sprite compat palette group 0
		for i, want := range pal.Shades {
			addr := uint32(0x8380) + uint32(i)*2
			got := emu.mem.Read16(addr)
			if got != uint16(want) {
				t.Errorf("%s: $%04X shade %d = 0x%04X, want 0x%04X",
					pal.Name, addr, i, got, want)
			}
		}

		// $6F94 index should match
		gotIdx := uint32(emu.mem.Read8(0x6F94))
		if gotIdx != pal.Index {
			t.Errorf("%s: $6F94 = 0x%02X, want 0x%02X", pal.Name, gotIdx, pal.Index)
		}
	}
}

func TestSetOptionMonoPaletteInvalid(t *testing.T) {
	emu, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	// Apply Black & White via Start
	emu.SetOption("mono_palette", "Black & White")
	emu.Start()
	before := emu.mem.Read16(0x8382)

	// Invalid name should be a no-op (Start applies invalid -> applyMonoPalette returns early)
	emu.SetOption("mono_palette", "InvalidName")
	emu.Start()
	after := emu.mem.Read16(0x8382)
	if after != before {
		t.Errorf("invalid palette changed $8382: 0x%04X -> 0x%04X", before, after)
	}
}

func TestSetOptionLanguageEnglish(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	emu.SetOption("language", "English")
	emu.Start()

	got := emu.mem.Read8(0x6F87)
	if got != 0x01 {
		t.Errorf("$6F87 = 0x%02X, want 0x01 (English)", got)
	}
}

func TestSetOptionLanguageJapanese(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	emu.SetOption("language", "Japanese")
	emu.Start()

	got := emu.mem.Read8(0x6F87)
	if got != 0x00 {
		t.Errorf("$6F87 = 0x%02X, want 0x00 (Japanese)", got)
	}
}

func TestSetOptionLanguageChecksum(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	// $6C25=$DC, $6C26-$6C2B=$00, $6F94=$00
	// English ($6F87=$01): checksum = $01 + $DC = $DD
	emu.SetOption("language", "English")
	emu.Start()
	got := emu.mem.Read16(0x6C14)
	if got != 0x00DD {
		t.Errorf("checksum after English = 0x%04X, want 0x00DD", got)
	}

	// Japanese ($6F87=$00): checksum = $00 + $DC = $DC
	emu.SetOption("language", "Japanese")
	emu.Start()
	got = emu.mem.Read16(0x6C14)
	if got != 0x00DC {
		t.Errorf("checksum after Japanese = 0x%04X, want 0x00DC", got)
	}
}

func TestSetOptionLanguageRealBIOSFastBoot(t *testing.T) {
	emu := makeTestEmulator(t)

	// Language should be applied for real BIOS fast boot
	emu.SetOption("language", "Japanese")
	emu.Start()

	got := emu.mem.Read8(0x6F87)
	if got != 0x00 {
		t.Errorf("$6F87 = 0x%02X, want 0x00 (Japanese applied for fast boot)", got)
	}

	// Config marker should be set
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x4E50 {
		t.Errorf("$6E95 = 0x%04X, want 0x4E50", marker)
	}
}

func TestSetOptionMonoPaletteChecksum(t *testing.T) {
	emu, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	// Set language to English and Blue palette
	// $6F87=$01, $6F94=$03: checksum = $01 + $DC + $03 = $E0
	emu.SetOption("language", "English")
	emu.SetOption("mono_palette", "Blue")
	emu.Start()
	got := emu.mem.Read16(0x6C14)
	if got != 0x00E0 {
		t.Errorf("checksum after Blue palette = 0x%04X, want 0x00E0", got)
	}
}

func TestDACBufferAlignment(t *testing.T) {
	emu := makeTestEmulator(t)
	emu.Start()

	emu.RunFrame()

	_, _, psgCount := emu.psg.GetBuffers()
	samples := emu.GetAudioSamples()

	// Audio buffer should have exactly psgCount stereo pairs
	if len(samples) != psgCount*2 {
		t.Errorf("audio buffer length = %d, want %d (psgCount=%d)",
			len(samples), psgCount*2, psgCount)
	}
}

func TestStartRealBIOSNormalBoot(t *testing.T) {
	emu := makeTestEmulator(t)

	emu.SetOption("fast_boot", "false")
	emu.SetOption("language", "English")
	emu.SetOption("mono_palette", "Black & White")
	emu.Start()

	// Config marker should be $4E50
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x4E50 {
		t.Errorf("$6E95 = 0x%04X, want 0x4E50", marker)
	}

	// PC should be $FF1800 (NVRAM boot path)
	if pc := emu.cpu.Registers().PC; pc != 0xFF1800 {
		t.Errorf("PC = 0x%06X, want 0xFF1800", pc)
	}

	// Checksum should be valid: $01 + $DC + $00 = $DD
	checksum := emu.mem.Read16(0x6C14)
	if checksum != 0x00DD {
		t.Errorf("checksum = 0x%04X, want 0x00DD", checksum)
	}
}

func TestStartRealBIOSFirstBoot(t *testing.T) {
	emu := makeTestEmulator(t)

	emu.SetOption("fast_boot", "false")
	emu.SetOption("first_boot", "true")
	emu.Start()

	// Config marker should be zeroed for first boot
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x0000 {
		t.Errorf("$6E95 = 0x%04X, want 0x0000 (first boot)", marker)
	}

	// PC should still be $FF1800 (Start does not change PC)
	if pc := emu.cpu.Registers().PC; pc != 0xFF1800 {
		t.Errorf("PC = 0x%06X, want 0xFF1800", pc)
	}
}

func TestStartOrderIndependence(t *testing.T) {
	// Order 1: language then palette
	emu1, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu1.SetOption("language", "Japanese")
	emu1.SetOption("mono_palette", "Blue")
	emu1.Start()

	// Order 2: palette then language
	emu2, err := NewEmulator(makeMinimalMonoCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu2.SetOption("mono_palette", "Blue")
	emu2.SetOption("language", "Japanese")
	emu2.Start()

	// Language should match
	lang1 := emu1.mem.Read8(0x6F87)
	lang2 := emu2.mem.Read8(0x6F87)
	if lang1 != lang2 {
		t.Errorf("language mismatch: order1=0x%02X, order2=0x%02X", lang1, lang2)
	}

	// Palette index should match
	pal1 := emu1.mem.Read8(0x6F94)
	pal2 := emu2.mem.Read8(0x6F94)
	if pal1 != pal2 {
		t.Errorf("palette mismatch: order1=0x%02X, order2=0x%02X", pal1, pal2)
	}

	// Checksum should match
	cs1 := emu1.mem.Read16(0x6C14)
	cs2 := emu2.mem.Read16(0x6C14)
	if cs1 != cs2 {
		t.Errorf("checksum mismatch: order1=0x%04X, order2=0x%04X", cs1, cs2)
	}

	// Config marker should match
	m1 := emu1.mem.Read16(0x6E95)
	m2 := emu2.mem.Read16(0x6E95)
	if m1 != m2 {
		t.Errorf("marker mismatch: order1=0x%04X, order2=0x%04X", m1, m2)
	}
}

func makeTestEmulatorWithCart(t *testing.T) Emulator {
	t.Helper()
	bios := make([]byte, biosROMSize)
	bios[0xFF00] = 0x10
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00
	bios[0x0010] = 0xC8
	bios[0x0011] = 0xFE

	cart := makeMinimalCart()
	emu, err := NewEmulator(cart)
	if err != nil {
		t.Fatalf("makeTestEmulatorWithCart: %v", err)
	}
	emu.SetBIOS("system_bios", bios)
	return emu
}

func TestStartRealBIOSFastBoot(t *testing.T) {
	emu := makeTestEmulatorWithCart(t)
	// fast_boot defaults to true
	emu.Start()

	// PC should be BIOS post-animation entry point $FF1BFC
	regs := emu.cpu.Registers()
	if regs.PC != 0xFF1BFC {
		t.Errorf("PC = 0x%06X, want 0xFF1BFC (BIOS post-animation entry)", regs.PC)
	}
	// XSP should be $6C00
	if regs.XSP != 0x6C00 {
		t.Errorf("XSP = 0x%04X, want 0x6C00", regs.XSP)
	}
	// SR should have IFF7, SYSM, and max
	wantSR := tlcs900h.SRInitMax | tlcs900h.SRInitSYSM | tlcs900h.SRInitIFF7
	if regs.SR != wantSR {
		t.Errorf("SR = 0x%04X, want 0x%04X", regs.SR, wantSR)
	}
	// Config marker should be set
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x4E50 {
		t.Errorf("$6E95 = 0x%04X, want 0x4E50", marker)
	}
}

func TestFastBootPrecedenceOverFirstBoot(t *testing.T) {
	emu := makeTestEmulatorWithCart(t)
	emu.SetOption("first_boot", "true")
	// fast_boot defaults to true, should take precedence
	emu.Start()

	// PC should be BIOS post-animation entry (fast boot), not $FF1800
	regs := emu.cpu.Registers()
	if regs.PC != 0xFF1BFC {
		t.Errorf("PC = 0x%06X, want 0xFF1BFC (fast boot overrides first_boot)", regs.PC)
	}
	// Config marker should be set (fast_boot skips first_boot zeroing)
	marker := emu.mem.Read16(0x6E95)
	if marker != 0x4E50 {
		t.Errorf("$6E95 = 0x%04X, want 0x4E50", marker)
	}
}

func TestFastBootIgnoredInHLE(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	// fast_boot defaults to true, but HLE has no BIOS to skip
	emu.Start()

	// HLE always boots to cart entry point
	regs := emu.cpu.Registers()
	if regs.PC != 0x200040 {
		t.Errorf("PC = 0x%06X, want 0x200040 (HLE ignores fast_boot)", regs.PC)
	}
}

func TestGetSRAMNoChanges(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	got := emu.GetSRAM()
	if got != nil {
		t.Errorf("GetSRAM with no flash writes should return nil, got %d bytes", len(got))
	}
}

func TestGetSetSRAMRoundTrip(t *testing.T) {
	cart := make([]byte, 512*1024)
	for i := range cart {
		cart[i] = byte(i)
	}
	// Set up valid entry point
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00

	emu, err := NewEmulator(cart)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	// Simulate a flash write by directly modifying cart data
	emu.mem.cart[0x100] = 0xAB
	emu.mem.cart[0x101] = 0xCD

	// Save
	sram := emu.GetSRAM()
	if sram == nil {
		t.Fatal("GetSRAM should return non-nil after flash modification")
	}

	// Create a new emulator with the same ROM
	emu2, err := NewEmulator(cart)
	if err != nil {
		t.Fatalf("NewEmulator 2: %v", err)
	}

	// Load save data
	emu2.SetSRAM(sram)

	// Verify modifications restored
	if emu2.mem.cart[0x100] != 0xAB {
		t.Errorf("cart[0x100] = 0x%02X, want 0xAB", emu2.mem.cart[0x100])
	}
	if emu2.mem.cart[0x101] != 0xCD {
		t.Errorf("cart[0x101] = 0x%02X, want 0xCD", emu2.mem.cart[0x101])
	}

	// Verify unmodified data is intact
	if emu2.mem.cart[0x200] != byte(0x200&0xFF) {
		t.Errorf("cart[0x200] = 0x%02X, want 0x%02X", emu2.mem.cart[0x200], byte(0x200&0xFF))
	}
}

func TestSetSRAMResetsToOrig(t *testing.T) {
	cart := make([]byte, 512*1024)
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00
	cart[0x50] = 0x42

	emu, err := NewEmulator(cart)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	// Modify cart
	emu.mem.cart[0x50] = 0xFF

	// SetSRAM with nil/empty should reset to original
	emu.SetSRAM(nil)
	if emu.mem.cart[0x50] != 0x42 {
		t.Errorf("cart[0x50] = 0x%02X after SetSRAM(nil), want 0x42", emu.mem.cart[0x50])
	}
}

func TestReadWriteRegionDecoupled(t *testing.T) {
	cart := make([]byte, 512*1024)
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00

	emu, err := NewEmulator(cart)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	// ReadRegion should return raw cart data (not NGF)
	region := emu.ReadRegion(coreif.MemorySaveRAM)
	if region == nil {
		t.Fatal("ReadRegion(MemorySaveRAM) returned nil")
	}
	if len(region) != len(cart) {
		t.Errorf("ReadRegion length = %d, want %d", len(region), len(cart))
	}

	// WriteRegion should write raw cart data
	region[0x100] = 0xBE
	emu.WriteRegion(coreif.MemorySaveRAM, region)
	if emu.mem.cart[0x100] != 0xBE {
		t.Errorf("cart[0x100] = 0x%02X after WriteRegion, want 0xBE", emu.mem.cart[0x100])
	}

	// GetSRAM should return NGF (not raw), reflecting the change
	sram := emu.GetSRAM()
	if sram == nil {
		t.Fatal("GetSRAM should return non-nil after WriteRegion modification")
	}
	// NGF starts with magic
	if len(sram) < 2 || sram[0] != 0x53 || sram[1] != 0x00 {
		t.Errorf("GetSRAM did not return NGF format, first bytes: %v", sram[:min(len(sram), 4)])
	}
}

func TestMemoryMapSystemRAMSize(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}

	regions := emu.MemoryMap()
	found := false
	for _, r := range regions {
		if r.Type == coreif.MemorySystemRAM {
			found = true
			// rcheevos expects 16 KB: work RAM (12 KB) + Z80 RAM (4 KB)
			want := workRAMSize + z80RAMSize
			if r.Size != want {
				t.Errorf("SystemRAM size = %d, want %d", r.Size, want)
			}
		}
	}
	if !found {
		t.Error("MemoryMap missing MemorySystemRAM region")
	}
}

func TestReadWriteRegionSystemRAM(t *testing.T) {
	emu, err := NewEmulator(makeMinimalCart())
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	// Write known values to work RAM and Z80 RAM
	emu.mem.workRAM[0] = 0xAA
	emu.mem.workRAM[workRAMSize-1] = 0xBB
	emu.mem.z80RAM[0] = 0xCC
	emu.mem.z80RAM[z80RAMSize-1] = 0xDD

	region := emu.ReadRegion(coreif.MemorySystemRAM)
	if region == nil {
		t.Fatal("ReadRegion(MemorySystemRAM) returned nil")
	}

	wantLen := workRAMSize + z80RAMSize
	if len(region) != wantLen {
		t.Fatalf("ReadRegion length = %d, want %d", len(region), wantLen)
	}

	// Work RAM at offset 0
	if region[0] != 0xAA {
		t.Errorf("region[0] = 0x%02X, want 0xAA (workRAM start)", region[0])
	}
	if region[workRAMSize-1] != 0xBB {
		t.Errorf("region[0x%X] = 0x%02X, want 0xBB (workRAM end)", workRAMSize-1, region[workRAMSize-1])
	}

	// Z80 RAM at offset workRAMSize
	if region[workRAMSize] != 0xCC {
		t.Errorf("region[0x%X] = 0x%02X, want 0xCC (z80RAM start)", workRAMSize, region[workRAMSize])
	}
	if region[wantLen-1] != 0xDD {
		t.Errorf("region[0x%X] = 0x%02X, want 0xDD (z80RAM end)", wantLen-1, region[wantLen-1])
	}

	// WriteRegion round-trip
	region[0] = 0x11
	region[workRAMSize] = 0x22
	emu.WriteRegion(coreif.MemorySystemRAM, region)

	if emu.mem.workRAM[0] != 0x11 {
		t.Errorf("workRAM[0] = 0x%02X after WriteRegion, want 0x11", emu.mem.workRAM[0])
	}
	if emu.mem.z80RAM[0] != 0x22 {
		t.Errorf("z80RAM[0] = 0x%02X after WriteRegion, want 0x22", emu.mem.z80RAM[0])
	}
}

func TestFastBootNoCart(t *testing.T) {
	emu := makeTestEmulator(t)
	// fast_boot defaults to true, no cart loaded
	emu.Start()

	// PC should be BIOS post-animation entry regardless of cart presence;
	// the BIOS handles cart detection from this point.
	regs := emu.cpu.Registers()
	if regs.PC != 0xFF1BFC {
		t.Errorf("PC = 0x%06X, want 0xFF1BFC (BIOS post-animation entry)", regs.PC)
	}
}
