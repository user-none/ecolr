package core

import (
	"testing"

	"github.com/user-none/eblitui/coreif"
	"github.com/user-none/ecolr/core/tlcs900h"
)

func TestGenerateBIOSSize(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()
	if len(bios) != biosROMSize {
		t.Errorf("generateBIOS() length = %d, want %d", len(bios), biosROMSize)
	}
}

func TestGenerateBIOSTrapBytes(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()

	// SWI 1 dispatcher at offset $0080.
	if bios[hleSWI1Offset] != hleOpcode {
		t.Errorf("SWI1 offset 0x%04X = 0x%02X, want 0x%02X", hleSWI1Offset, bios[hleSWI1Offset], hleOpcode)
	}

	// System call handlers at offsets $0100-$011A.
	for i := 0; i < hleSysCallCount; i++ {
		off := hleSysCallBase + i
		if bios[off] != hleOpcode {
			t.Errorf("syscall[%d] offset 0x%04X = 0x%02X, want 0x%02X", i, off, bios[off], hleOpcode)
		}
	}

	// Interrupt handlers at offsets $0200-$020F.
	for i := 0; i < hleIntCount; i++ {
		off := hleIntBase + i
		if bios[off] != hleOpcode {
			t.Errorf("int[%d] offset 0x%04X = 0x%02X, want 0x%02X", i, off, bios[off], hleOpcode)
		}
	}

	// Default handler at offset $0300.
	if bios[hleDefaultHandler] != hleOpcode {
		t.Errorf("default handler offset 0x%04X = 0x%02X, want 0x%02X", hleDefaultHandler, bios[hleDefaultHandler], hleOpcode)
	}
}

func TestGenerateBIOSVectorTable(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()

	readLE24 := func(buf []byte) uint32 {
		return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16
	}

	// Entry 0: reset vector -> default handler.
	got := readLE24(bios[hleVectorTable:])
	want := uint32(hleBIOSBase + hleDefaultHandler)
	if got != want {
		t.Errorf("vector[0] = 0x%06X, want 0x%06X", got, want)
	}

	// Entry 1: SWI 1 handler.
	got = readLE24(bios[hleVectorTable+4:])
	want = uint32(hleBIOSBase + hleSWI1Offset)
	if got != want {
		t.Errorf("vector[1] = 0x%06X, want 0x%06X", got, want)
	}

	// Entries 2-15: interrupt handlers.
	for i := 2; i < hleIntCount; i++ {
		got = readLE24(bios[hleVectorTable+i*4:])
		want = uint32(hleBIOSBase + hleIntBase + i)
		if got != want {
			t.Errorf("vector[%d] = 0x%06X, want 0x%06X", i, got, want)
		}
	}
}

func TestGenerateBIOSSWI1JumpTable(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()

	readLE24 := func(buf []byte) uint32 {
		return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16
	}

	for i := 0; i < hleSysCallCount; i++ {
		got := readLE24(bios[hleSWI1JumpTable+i*4:])
		want := uint32(hleBIOSBase + hleSysCallBase + i)
		if got != want {
			t.Errorf("swi1_jump[%d] = 0x%06X, want 0x%06X", i, got, want)
		}
	}
}

// newHLETestCPU creates a minimal hleBIOS+CPU for dispatch testing.
// The CPU is backed by a Memory with the synthetic BIOS installed.
func newHLETestCPU(t *testing.T) (*hleBIOS, *tlcs900h.CPU) {
	t.Helper()
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	hle := newHLEBIOS(mem)
	synthBIOS := hle.generateBIOS()
	mem.SetBIOS(synthBIOS)
	c := tlcs900h.New(mem)
	mem.SetCPU(c)
	hle.install(c)
	return hle, c
}

func TestDispatchSWI1(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Set PC to just after the SWI 1 handler address (simulating fetchPC
	// having advanced past the 0xF6 byte).
	regs := c.Registers()
	regs.PC = hleBIOSBase + hleSWI1Offset + 1
	c.SetState(regs)

	before := c.Cycles()
	hle.dispatch(c)
	after := c.Cycles()

	if after <= before {
		t.Error("dispatch at SWI1 address should consume cycles")
	}
}

func TestDispatchSysCall(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Test a few system call addresses.
	for _, idx := range []int{0x00, 0x05, 0x1A} {
		regs := c.Registers()
		regs.PC = uint32(hleBIOSBase + hleSysCallBase + idx + 1)
		c.SetState(regs)

		before := c.Cycles()
		hle.dispatch(c)
		after := c.Cycles()

		if after <= before {
			t.Errorf("dispatch at syscall[0x%02X] should consume cycles", idx)
		}
	}
}

func TestDispatchInterrupt(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Test interrupt handler addresses (entries 2-15).
	for i := 2; i < hleIntCount; i++ {
		regs := c.Registers()
		regs.PC = uint32(hleBIOSBase + hleIntBase + i + 1)
		c.SetState(regs)

		before := c.Cycles()
		hle.dispatch(c)
		after := c.Cycles()

		if after <= before {
			t.Errorf("dispatch at int[%d] should consume cycles", i)
		}
	}
}

func TestDispatchDefaultHandler(t *testing.T) {
	hle, c := newHLETestCPU(t)

	regs := c.Registers()
	regs.PC = hleBIOSBase + hleDefaultHandler + 1
	c.SetState(regs)

	before := c.Cycles()
	hle.dispatch(c)
	after := c.Cycles()

	if after <= before {
		t.Error("dispatch at default handler should consume cycles")
	}
}

func TestHLEEmulatorCreation(t *testing.T) {
	cart := make([]byte, 256)
	// Write entry point $20ABCD at cart header offset $1C (24-bit LE).
	cart[0x1C] = 0xCD
	cart[0x1D] = 0xAB
	cart[0x1E] = 0x20

	emu, err := NewEmulator(cart, coreif.RegionNTSC)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	regs := emu.cpu.Registers()
	if regs.PC != 0x20ABCD {
		t.Errorf("PC = 0x%06X, want 0x20ABCD", regs.PC)
	}

	if emu.mem.bios == nil {
		t.Fatal("BIOS ROM should be set after HLE init")
	}
	if len(emu.mem.bios) != biosROMSize {
		t.Errorf("BIOS size = %d, want %d", len(emu.mem.bios), biosROMSize)
	}
}

func TestHLEEmulatorStep(t *testing.T) {
	cart := make([]byte, 256)

	emu, err := NewEmulator(cart, coreif.RegionNTSC)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.Start()

	for i := 0; i < 10; i++ {
		cycles := emu.cpu.Step()
		if cycles == 0 {
			t.Fatalf("step %d: zero cycles", i)
		}
	}
}

func TestRealBIOSEmulatorCreation(t *testing.T) {
	bios := make([]byte, biosROMSize)
	bios[0xFF00] = 0x10
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00

	emu, err := NewEmulator(nil, coreif.RegionNTSC)
	if err != nil {
		t.Fatalf("NewEmulator: %v", err)
	}
	emu.SetBIOS("system_bios", bios)
	emu.SetOption("fast_boot", "false")
	emu.Start()

	if !emu.hasBIOS {
		t.Error("expected hasBIOS == true for real BIOS")
	}

	// NVRAM boot path ($FF1800) when fast_boot is disabled
	regs := emu.cpu.Registers()
	if regs.PC != 0xFF1800 {
		t.Errorf("PC = 0x%06X, want 0xFF1800", regs.PC)
	}
}

func TestNewMemoryNoBIOS(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	if mem.bios != nil {
		t.Error("bios should be nil before SetBIOS")
	}
}

func TestSetBIOS(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	data := make([]byte, biosROMSize)
	data[0] = 0x42
	mem.SetBIOS(data)

	if mem.bios == nil {
		t.Fatal("bios should not be nil after SetBIOS")
	}
	if mem.bios[0] != 0x42 {
		t.Errorf("bios[0] = 0x%02X, want 0x42", mem.bios[0])
	}
}

// newHLETestCPUWithCart creates an hleBIOS+CPU with a cartridge for flash testing.
func newHLETestCPUWithCart(t *testing.T, cartSize int) (*hleBIOS, *tlcs900h.CPU, *Memory) {
	t.Helper()
	cart := make([]byte, cartSize)
	for i := range cart {
		cart[i] = uint8(i)
	}
	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	hle := newHLEBIOS(mem)
	synthBIOS := hle.generateBIOS()
	mem.SetBIOS(synthBIOS)
	c := tlcs900h.New(mem)
	mem.SetCPU(c)
	hle.install(c)
	return hle, c, mem
}

func TestHLESysShutdown(t *testing.T) {
	hle, c := newHLETestCPU(t)

	hle.hleSysShutdown(c)

	if !c.Halted() {
		t.Error("CPU should be halted after shutdown")
	}
}

func TestHLESysIntLvSet(t *testing.T) {
	hle, c := newHLETestCPU(t)

	tests := []struct {
		source uint8
		level  uint8
		reg    int
		high   bool
	}{
		{0, 3, 0, false}, // INT0 -> $70 low
		{1, 5, 1, true},  // INT5 -> $71 high
		{6, 2, 9, false}, // INTTC0 -> $79 low
		{9, 4, 10, true}, // INTTC3 -> $7A high
	}

	for _, tt := range tests {
		regs := c.Registers()
		regs.Bank[3].XBC = uint32(tt.level)<<8 | uint32(tt.source)
		c.SetState(regs)

		hle.hleSysIntLvSet(c)

		got := hle.mem.intc.ReadReg(tt.reg)
		if tt.high {
			gotLevel := (got >> 4) & 0x07
			if gotLevel != tt.level {
				t.Errorf("source %d: high nibble level = %d, want %d", tt.source, gotLevel, tt.level)
			}
		} else {
			gotLevel := got & 0x07
			if gotLevel != tt.level {
				t.Errorf("source %d: low nibble level = %d, want %d", tt.source, gotLevel, tt.level)
			}
		}
	}
}

func TestHLESysIntLvSetOutOfRange(t *testing.T) {
	hle, c := newHLETestCPU(t)

	regs := c.Registers()
	regs.Bank[3].XBC = 0x0300 | 0x0A // source 10, level 3
	c.SetState(regs)

	before := c.Cycles()
	hle.hleSysIntLvSet(c)
	after := c.Cycles()

	if after <= before {
		t.Error("IntLvSet with out-of-range source should still consume cycles")
	}
}

func TestHLESysRTCGet(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Point XHL3 to work RAM
	regs := c.Registers()
	regs.Bank[3].XHL = 0x5000
	c.SetState(regs)

	hle.hleSysRTCGet(c)

	// Verify bytes 0-5 are BCD format (byte 6 is leap year + day of week, not BCD)
	for i := uint32(0); i < 6; i++ {
		val := hle.mem.Read8(0x5000 + i)
		// Each BCD byte should have valid nibbles (0-9)
		hi := val >> 4
		lo := val & 0x0F
		if hi > 9 || lo > 9 {
			t.Errorf("byte %d = 0x%02X: invalid BCD (hi=%d, lo=%d)", i, val, hi, lo)
		}
	}

	// Byte 6: upper nibble = years since leap year (0-3), lower nibble = day of week (0-6)
	b6 := hle.mem.Read8(0x5006)
	leapYears := b6 >> 4
	dow := b6 & 0x0F
	if leapYears > 3 {
		t.Errorf("leap year offset = %d, want 0-3", leapYears)
	}
	if dow > 6 {
		t.Errorf("day of week = %d, want 0-6", dow)
	}
}

func TestToBCD(t *testing.T) {
	tests := []struct {
		in   int
		want uint8
	}{
		{0, 0x00},
		{9, 0x09},
		{10, 0x10},
		{25, 0x25},
		{59, 0x59},
		{99, 0x99},
	}
	for _, tt := range tests {
		got := toBCD(tt.in)
		if got != tt.want {
			t.Errorf("toBCD(%d) = 0x%02X, want 0x%02X", tt.in, got, tt.want)
		}
	}
}

func TestHLESysGEModeSetColor(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Set system code >= $10 for color mode
	hle.mem.Write8(0x6F91, 0x10)

	hle.hleSysGEModeSet(c)

	mode := hle.mem.Read8(0x87E2)
	if mode != 0x00 {
		t.Errorf("K2GE mode = 0x%02X, want 0x00 (color)", mode)
	}
	flag := hle.mem.Read8(0x6F95)
	if flag != 0x10 {
		t.Errorf("mode flag = 0x%02X, want 0x10", flag)
	}
}

func TestHLESysGEModeSetMono(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Set system code < $10 for mono mode
	hle.mem.Write8(0x6F91, 0x00)

	hle.hleSysGEModeSet(c)

	mode := hle.mem.Read8(0x87E2)
	if mode != 0x80 {
		t.Errorf("K2GE mode = 0x%02X, want 0x80 (mono)", mode)
	}
	flag := hle.mem.Read8(0x6F95)
	if flag != 0x00 {
		t.Errorf("mode flag = 0x%02X, want 0x00", flag)
	}
}

func TestHLESysComOnOffRTS(t *testing.T) {
	hle, c := newHLETestCPU(t)

	hle.hleSysComOnRTS(c)
	got := hle.mem.Read8(0xB2)
	if got != 0x00 {
		t.Errorf("ComOnRTS: $B2 = 0x%02X, want 0x00", got)
	}

	hle.hleSysComOffRTS(c)
	got = hle.mem.Read8(0xB2)
	if got != 0x01 {
		t.Errorf("ComOffRTS: $B2 = 0x%02X, want 0x01", got)
	}
}

func TestHLESysComStatus(t *testing.T) {
	_, c := newHLETestCPU(t)
	hle, _ := newHLETestCPU(t)

	hle.hleSysComSendStatus(c)
	regs := c.Registers()
	if uint16(regs.Bank[3].XWA) != 0 {
		t.Errorf("ComSendStatus: WA3 = 0x%04X, want 0x0000", uint16(regs.Bank[3].XWA))
	}

	hle.hleSysComRecvStatus(c)
	regs = c.Registers()
	if uint16(regs.Bank[3].XWA) != 0 {
		t.Errorf("ComRecvStatus: WA3 = 0x%04X, want 0x0000", uint16(regs.Bank[3].XWA))
	}
}

func TestHLESysComGetDataEmpty(t *testing.T) {
	hle, c := newHLETestCPU(t)

	hle.hleSysComGetData(c)
	if c.ReadBank3RA() != 0x01 {
		t.Errorf("ComGetData: RA3 = 0x%02X, want 0x01", c.ReadBank3RA())
	}

	hle.hleSysComGetBufData(c)
	if c.ReadBank3RA() != 0x01 {
		t.Errorf("ComGetBufData: RA3 = 0x%02X, want 0x01", c.ReadBank3RA())
	}
}

func TestHLESysFlashWrite(t *testing.T) {
	hle, c, mem := newHLETestCPUWithCart(t, 512*1024)

	// Write some source data into work RAM
	for i := uint32(0); i < 512; i++ {
		mem.Write8(0x5000+i, 0xA0+uint8(i))
	}

	// Set up bank 3 registers: bank=0, pageCount=2 (512 bytes), dest=0x1000, src=$5000
	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000000 // RA3=bank 0
	regs.Bank[3].XBC = 0x00000002 // BC3=page count 2
	regs.Bank[3].XDE = 0x00001000 // XDE3=dest offset
	regs.Bank[3].XHL = 0x00005000 // XHL3=src addr
	c.SetState(regs)

	hle.hleSysFlashWrite(c)

	if c.ReadBank3RA() != 0x00 {
		t.Errorf("FlashWrite: RA3 = 0x%02X, want 0x00 (success)", c.ReadBank3RA())
	}

	// Verify flash data was written
	data := mem.cs0.Data()
	for i := 0; i < 512; i++ {
		want := uint8(0xA0 + uint8(i))
		if data[0x1000+i] != want {
			t.Errorf("flash[0x%X] = 0x%02X, want 0x%02X", 0x1000+i, data[0x1000+i], want)
			break
		}
	}
}

func TestHLESysFlashWriteNilChip(t *testing.T) {
	hle, c := newHLETestCPU(t)

	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000000 // bank 0
	c.SetState(regs)

	hle.hleSysFlashWrite(c)

	if c.ReadBank3RA() != 0xFF {
		t.Errorf("FlashWrite nil chip: RA3 = 0x%02X, want 0xFF", c.ReadBank3RA())
	}
}

func TestHLESysFlashAllErs(t *testing.T) {
	hle, c, mem := newHLETestCPUWithCart(t, 512*1024)

	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000000 // bank 0
	c.SetState(regs)

	hle.hleSysFlashAllErs(c)

	if c.ReadBank3RA() != 0x00 {
		t.Errorf("FlashAllErs: RA3 = 0x%02X, want 0x00", c.ReadBank3RA())
	}

	data := mem.cs0.Data()
	for i := 0; i < len(data); i++ {
		if data[i] != 0xFF {
			t.Errorf("flash[0x%X] = 0x%02X after FlashAllErs, want 0xFF", i, data[i])
			break
		}
	}
}

func TestHLESysFlashErs(t *testing.T) {
	hle, c, mem := newHLETestCPUWithCart(t, 512*1024)

	// Erase block 1 (offset 0x10000)
	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000000 // bank 0
	regs.Bank[3].XBC = 0x00000100 // RB3=block 1 (bits 8-15)
	c.SetState(regs)

	hle.hleSysFlashErs(c)

	if c.ReadBank3RA() != 0x00 {
		t.Errorf("FlashErs: RA3 = 0x%02X, want 0x00", c.ReadBank3RA())
	}

	data := mem.cs0.Data()
	// Block 1 should be erased
	for i := 0x10000; i < 0x20000; i++ {
		if data[i] != 0xFF {
			t.Errorf("flash[0x%X] = 0x%02X after FlashErs, want 0xFF", i, data[i])
			break
		}
	}
	// Block 0 should be untouched
	if data[0x100] == 0xFF {
		t.Error("block 0 was erased, should be untouched")
	}
}

func TestGenerateBIOSRETIByte(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()

	if bios[hleRETIHandler] != 0x07 {
		t.Errorf("RETI handler offset 0x%04X = 0x%02X, want 0x07", hleRETIHandler, bios[hleRETIHandler])
	}
}

func TestGenerateBIOSExpandedVectorTable(t *testing.T) {
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory(nil): %v", err)
	}
	hle := newHLEBIOS(mem)
	bios := hle.generateBIOS()

	readLE24 := func(buf []byte) uint32 {
		return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16
	}

	// Verify entries 16-32 are populated (these were previously zeroed).
	for i := 16; i < hleIntCount; i++ {
		got := readLE24(bios[hleVectorTable+i*4:])
		want := uint32(hleBIOSBase + hleIntBase + i)
		if got != want {
			t.Errorf("vector[%d] = 0x%06X, want 0x%06X", i, got, want)
		}
	}
}

func TestInitVectorTable(t *testing.T) {
	hle, _ := newHLETestCPU(t)

	hle.initVectorTable(hle.mem)

	retiAddr := uint32(hleBIOSBase + hleRETIHandler)
	for i := 0; i < 18; i++ {
		addr := uint32(0x6FB8) + uint32(i)*4
		got := hle.mem.Read32(addr) & 0xFFFFFF
		if got != retiAddr {
			t.Errorf("RAM vector[%d] at $%04X = 0x%06X, want 0x%06X", i, addr, got, retiAddr)
		}
	}
}

func TestSWI1DispatchRETI(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Set up a stack frame as the CPU would push for SWI 1.
	wantSR := tlcs900h.SRInitMax | tlcs900h.SRInitSYSM
	wantPC := uint32(0x200100)
	regs := c.Registers()
	regs.XSP = 0x6C00
	c.SetState(regs)

	// Push SR+PC onto stack (SWI pushes SR at XSP, PC at XSP+2).
	hle.mem.Write16(0x6BFA, wantSR)
	hle.mem.Write32(0x6BFC, wantPC)

	regs = c.Registers()
	regs.XSP = 0x6BFA
	regs.PC = hleBIOSBase + hleSWI1Offset + 1
	regs.Bank[3].XBC = 0x03 // syscall index 3 (stub)
	c.SetState(regs)

	hle.hleSWI1Dispatch(c)

	regs = c.Registers()
	if regs.PC != wantPC {
		t.Errorf("PC after SWI1+RETI = 0x%06X, want 0x%06X", regs.PC, wantPC)
	}
	if regs.XSP != 0x6C00 {
		t.Errorf("XSP after SWI1+RETI = 0x%04X, want 0x6C00", regs.XSP)
	}
}

func TestIntDispatchUserHandler(t *testing.T) {
	hle, c := newHLETestCPU(t)
	hle.initVectorTable(hle.mem)

	// Install a user handler for VBlank (vector offset $2C -> RAM $6FCC).
	userHandler := uint32(0x200400)
	hle.mem.Write8(0x6FCC, uint8(userHandler))
	hle.mem.Write8(0x6FCD, uint8(userHandler>>8))
	hle.mem.Write8(0x6FCE, uint8(userHandler>>16))
	hle.mem.Write8(0x6FCF, 0)

	regs := c.Registers()
	regs.XSP = 0x6C00
	c.SetState(regs)

	hle.hleIntDispatch(c, 0x2C)

	regs = c.Registers()
	if regs.PC != userHandler {
		t.Errorf("PC after int dispatch = 0x%06X, want 0x%06X", regs.PC, userHandler)
	}
}

func TestIntDispatchDefaultRETI(t *testing.T) {
	hle, c := newHLETestCPU(t)
	hle.initVectorTable(hle.mem)

	// Default handler (no user handler installed) should RETI directly.
	// Set up stack frame.
	wantPC := uint32(0x200200)
	wantSR := tlcs900h.SRInitMax | tlcs900h.SRInitSYSM
	regs := c.Registers()
	regs.XSP = 0x6BFA
	c.SetState(regs)

	hle.mem.Write16(0x6BFA, wantSR)
	hle.mem.Write32(0x6BFC, wantPC)

	hle.hleIntDispatch(c, 0x2C)

	// Default RAM vector matches the RETI handler address, so
	// hleIntDispatch performs RETI directly, restoring PC and SR
	// from the stack frame.
	regs = c.Registers()
	if regs.PC != wantPC {
		t.Errorf("PC after default int dispatch = 0x%06X, want 0x%06X", regs.PC, wantPC)
	}
	if regs.XSP != 0x6C00 {
		t.Errorf("XSP after default int dispatch = 0x%04X, want 0x6C00", regs.XSP)
	}
}

func TestIntDispatchUnmappedVector(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Unmapped vector offset should RETI directly.
	wantPC := uint32(0x200300)
	wantSR := tlcs900h.SRInitMax | tlcs900h.SRInitSYSM
	regs := c.Registers()
	regs.XSP = 0x6BFA
	c.SetState(regs)

	hle.mem.Write16(0x6BFA, wantSR)
	hle.mem.Write32(0x6BFC, wantPC)

	hle.hleIntDispatch(c, 0x04) // vector offset $04, not in map

	regs = c.Registers()
	if regs.PC != wantPC {
		t.Errorf("PC after unmapped int dispatch = 0x%06X, want 0x%06X", regs.PC, wantPC)
	}
	if regs.XSP != 0x6C00 {
		t.Errorf("XSP after unmapped int dispatch = 0x%04X, want 0x6C00", regs.XSP)
	}
}

func TestHLESysFontSet(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Set RA3: font color=1 (bits 0-1), bg color=2 (bits 4-5)
	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000021 // bg=2 (bits 5-4=10), font=1 (bits 1-0=01)
	c.SetState(regs)

	hle.hleSysFontSet(c)

	// Verify output size: 4096 bytes at $A000-$AFFF
	// Character 0 ($00) is all zeros in the font, so all pixels should be bg color (2).
	// Each row of char 0: all clear bits -> all bg color.
	// Left 4 pixels: bg=2 -> hi = 0b10_10_10_10 = 0xAA
	// Right 4 pixels: bg=2 -> lo = 0b10_10_10_10 = 0xAA
	for row := 0; row < 8; row++ {
		addr := uint32(0xA000) + uint32(row*2)
		lo := hle.mem.Read8(addr)
		hi := hle.mem.Read8(addr + 1)
		if lo != 0xAA {
			t.Errorf("char 0 row %d lo = 0x%02X, want 0xAA", row, lo)
		}
		if hi != 0xAA {
			t.Errorf("char 0 row %d hi = 0x%02X, want 0xAA", row, hi)
		}
	}

	// Character 2 ($02) is all 0xFF in the font, so all pixels should be font color (1).
	// Left 4 pixels: font=1 -> hi = 0b01_01_01_01 = 0x55
	// Right 4 pixels: font=1 -> lo = 0b01_01_01_01 = 0x55
	for row := 0; row < 8; row++ {
		addr := uint32(0xA000) + 32 + uint32(row*2) // char 2 starts at offset 32
		lo := hle.mem.Read8(addr)
		hi := hle.mem.Read8(addr + 1)
		if lo != 0x55 {
			t.Errorf("char 2 row %d lo = 0x%02X, want 0x55", row, lo)
		}
		if hi != 0x55 {
			t.Errorf("char 2 row %d hi = 0x%02X, want 0x55", row, hi)
		}
	}
}

func TestHLESysFontSetDefaultColors(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Default: font=0, bg=0 -> all output bytes should be 0x00
	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000000
	c.SetState(regs)

	hle.hleSysFontSet(c)

	// All output should be zero since both colors are 0
	for i := uint32(0); i < 4096; i++ {
		val := hle.mem.Read8(0xA000 + i)
		if val != 0x00 {
			t.Errorf("$%04X = 0x%02X with zero colors, want 0x00", 0xA000+i, val)
			break
		}
	}
}

func TestHLESysFontSetMixedRow(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// font=3 (0b11), bg=0
	regs := c.Registers()
	regs.Bank[3].XWA = 0x00000003
	c.SetState(regs)

	hle.hleSysFontSet(c)

	// Character $20 (space) at offset 0x20*8=0x100 in font data.
	// Font data for $20 row 0: hleFontData[0x100] = 0x00 (all clear)
	// So output should be all bg (0).
	addr := uint32(0xA000) + uint32(0x20)*16
	lo := hle.mem.Read8(addr)
	hi := hle.mem.Read8(addr + 1)
	if lo != 0x00 || hi != 0x00 {
		t.Errorf("char $20 row 0: lo=0x%02X hi=0x%02X, want 0x00 0x00", lo, hi)
	}

	// Character $21 ('!') row 0: hleFontData[0x108] = 0x10 = 0b00010000
	// Left 4 pixels (bits 7-4): 0,0,0,1 -> 0b00_00_00_11 = 0x03
	// Right 4 pixels (bits 3-0): 0,0,0,0 -> 0b00_00_00_00 = 0x00
	addr21 := uint32(0xA000) + uint32(0x21)*16
	lo21 := hle.mem.Read8(addr21)
	hi21 := hle.mem.Read8(addr21 + 1)
	if hi21 != 0x03 {
		t.Errorf("char $21 row 0: hi=0x%02X, want 0x03", hi21)
	}
	if lo21 != 0x00 {
		t.Errorf("char $21 row 0: lo=0x%02X, want 0x00", lo21)
	}
}

func TestFontDataSize(t *testing.T) {
	if len(hleFontData) != 2048 {
		t.Errorf("hleFontData size = %d, want 2048", len(hleFontData))
	}
}

func TestHLESysReturnSuccess(t *testing.T) {
	hle, c := newHLETestCPU(t)

	// Test all syscalls that return RA3=0x00
	funcs := []struct {
		name string
		fn   func(*hleBIOS, *tlcs900h.CPU)
	}{
		{"AlarmSet", (*hleBIOS).hleSysAlarmSet},
		{"AlarmDownSet", (*hleBIOS).hleSysAlarmDownSet},
		{"FlashProtect", (*hleBIOS).hleSysFlashProtect},
		{"ComInit", (*hleBIOS).hleSysComInit},
		{"ComCreateData", (*hleBIOS).hleSysComCreateData},
		{"ComCreateBufData", (*hleBIOS).hleSysComCreateBufData},
	}

	for _, tt := range funcs {
		// Set RA3 to non-zero to verify it gets cleared
		regs := c.Registers()
		regs.Bank[3].XWA = 0x000000FF
		c.SetState(regs)

		tt.fn(hle, c)

		if c.ReadBank3RA() != 0x00 {
			t.Errorf("%s: RA3 = 0x%02X, want 0x00", tt.name, c.ReadBank3RA())
		}
	}
}

func TestInitRAMSFR(t *testing.T) {
	_, _, mem := newHLETestCPUWithCart(t, 256)
	mem.InitSystemState()

	checks := []struct {
		addr uint32
		want uint8
		name string
	}{
		// Port registers
		{0x06, 0xFF, "P5FC"},
		{0x09, 0xFF, "P7"},
		{0x0D, 0x09, "P8CR"},
		{0x10, 0x34, "PACR"},
		{0x11, 0x3C, "PAFC"},
		{0x13, 0xFF, "PBCR"},
		// Memory controller
		{0x15, 0x3F, "MSAR0"},
		{0x18, 0x3F, "MSAR2"},
		{0x1A, 0x2D, "MAMR2"},
		{0x1B, 0x01, "DTEFCR"},
		{0x1E, 0x0F, "BEXCS"},
		{0x1F, 0xB2, "BCS"},
		// Timers
		{0x20, 0x80, "TRUN"},
		{0x22, 0x01, "TREG0"},
		{0x23, 0x90, "TREG1"},
		{0x24, 0x03, "T01MOD"},
		{0x27, 0x62, "TREG3"},
		{0x28, 0x05, "T23MOD"},
		{0x2C, 0x0C, "Timer capture low"},
		{0x2D, 0x0C, "Timer capture high"},
		{0x2E, 0x4C, "TFFCR2 low"},
		{0x2F, 0x4C, "TFFCR2 high"},
		{0x38, 0x30, "T4MOD"},
		{0x3C, 0x20, "TREG8L"},
		{0x3D, 0xFF, "TREG8H"},
		{0x3E, 0x80, "TREG9L"},
		{0x3F, 0x7F, "TREG9H"},
		{0x48, 0x30, "T89MOD"},
		// Serial
		{0x51, 0x20, "SC0BUF"},
		{0x52, 0x69, "SC0MOD"},
		{0x53, 0x15, "BR0CR"},
		// I/O function control
		{0x5C, 0xFF, "IOFC"},
		// ADC mode
		{0x68, 0x17, "ADMOD1"},
		{0x69, 0x17, "ADMOD2"},
		{0x6A, 0x03, "ADREG0L"},
		{0x6B, 0x03, "ADREG0H"},
		{0x6C, 0x02, "ADREG1L"},
		// Watchdog
		{0x6E, 0xF0, "WDMOD"},
		// Interrupt priorities
		{0x70, 0x02, "INTE0AD"},
		{0x71, 0x54, "INTE45"},
		// DAC
		{0xA2, 0x80, "DACL"},
		{0xA3, 0x80, "DACR"},
		// Custom I/O
		{0xB2, 0x01, "RTS disabled"},
		{0xB3, 0x04, "NMI enable"},
		{0xB4, 0x0A, "Custom IO B4"},
		{0xB6, 0x05, "Power state"},
		{0xBC, 0xFE, "Custom IO BC"},
	}
	for _, tt := range checks {
		got := mem.readIOByte(tt.addr)
		if got != tt.want {
			t.Errorf("%s ($%02X) = 0x%02X, want 0x%02X", tt.name, tt.addr, got, tt.want)
		}
	}
}

func TestInitRAMK2GE(t *testing.T) {
	_, _, mem := newHLETestCPUWithCart(t, 256)
	mem.InitSystemState()

	checks := []struct {
		addr uint32
		want uint8
		name string
	}{
		{0x8000, 0xC0, "K2GE IRQ enable"},
		{0x8004, 0xFF, "Window H end"},
		{0x8005, 0xFF, "Window V end"},
		{0x8006, 0xC6, "Frame rate"},
		{0x8118, 0x80, "Background color on"},
		{0x8400, 0xFF, "LED on"},
		{0x87F4, 0x80, "K2GE config"},
		// Mono palette registers (memdump.bin values)
		{0x8101, 0x00, "SPPLT01"},
		{0x8102, 0x04, "SPPLT02"},
		{0x8103, 0x07, "SPPLT03"},
		{0x8105, 0x07, "SPPLT11"},
		{0x8106, 0x07, "SPPLT12"},
		{0x8107, 0x00, "SPPLT13"},
		{0x8109, 0x07, "SC1PLT01"},
		{0x810A, 0x07, "SC1PLT02"},
		{0x810B, 0x07, "SC1PLT03"},
		{0x810D, 0x00, "SC1PLT11"},
		{0x810E, 0x02, "SC1PLT12"},
		{0x810F, 0x06, "SC1PLT13"},
		{0x8111, 0x00, "SC2PLT01"},
		{0x8112, 0x00, "SC2PLT02"},
		{0x8113, 0x00, "SC2PLT03"},
		{0x8115, 0x07, "SC2PLT11"},
		{0x8116, 0x07, "SC2PLT12"},
		{0x8117, 0x07, "SC2PLT13"},
	}
	for _, tt := range checks {
		got := mem.Read8(tt.addr)
		if got != tt.want {
			t.Errorf("%s ($%04X) = 0x%02X, want 0x%02X", tt.name, tt.addr, got, tt.want)
		}
	}

	// Cart has system code $23 (color), so setGEMode sets K2GE color mode.
	// $87E2 = $00 (color), $6F95 = $10 (color flag)
	if got := mem.Read8(0x87E2); got != 0x00 {
		t.Errorf("$87E2 = 0x%02X, want 0x00 (K2GE color mode)", got)
	}
	if got := mem.Read8(0x6F95); got != 0x10 {
		t.Errorf("$6F95 = 0x%02X, want 0x10 (color flag)", got)
	}

	// Color cart: K1GE compat palette area ($8380-$83FF) should NOT be filled
	for i := uint32(0); i < 0x80; i++ {
		got := mem.Read8(0x8380 + i)
		if got != 0x00 {
			t.Errorf("palette $%04X = 0x%02X, want 0x00 (color game, should not fill)", 0x8380+i, got)
			break
		}
	}
}

func TestInitRAMCartHeader(t *testing.T) {
	cart := make([]byte, 256)
	// Entry point at $1C
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00
	// Software ID at $20
	cart[0x20] = 0xAB
	cart[0x21] = 0xCD
	// Sub-code at $22
	cart[0x22] = 0x07
	// System code at $23
	cart[0x23] = 0x10 // color
	// Title at $24
	copy(cart[0x24:0x30], []byte("TEST TITLE!!"))

	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	mem.InitSystemState()

	// Entry point at $6C00
	if mem.Read32(0x6C00)&0xFFFFFF != 0x200040 {
		t.Errorf("$6C00 entry point = 0x%06X, want 0x200040", mem.Read32(0x6C00)&0xFFFFFF)
	}
	// Software ID at $6C04 and $6E82
	if mem.Read16(0x6C04) != 0xCDAB {
		t.Errorf("$6C04 SW ID = 0x%04X, want 0xCDAB", mem.Read16(0x6C04))
	}
	if mem.Read16(0x6E82) != 0xCDAB {
		t.Errorf("$6E82 SW ID = 0x%04X, want 0xCDAB", mem.Read16(0x6E82))
	}
	// Sub-code at $6C06 and $6E84
	if mem.Read8(0x6C06) != 0x07 {
		t.Errorf("$6C06 sub-code = 0x%02X, want 0x07", mem.Read8(0x6C06))
	}
	if mem.Read8(0x6E84) != 0x07 {
		t.Errorf("$6E84 sub-code = 0x%02X, want 0x07", mem.Read8(0x6E84))
	}
	// System code at $6F91
	if mem.Read8(0x6F91) != 0x10 {
		t.Errorf("$6F91 sys code = 0x%02X, want 0x10", mem.Read8(0x6F91))
	}
	// $6F95 is set by setGEMode: $10 for color (system code >= $10)
	if mem.Read8(0x6F95) != 0x10 {
		t.Errorf("$6F95 = 0x%02X, want 0x10 (color mode)", mem.Read8(0x6F95))
	}
	// Software ID is written to $6C04 (verified above) and $6C69.
	// $6C69 write lands at the normal workRAM offset; the read path
	// mirrors it back through $6C04.
	if mem.Read8(0x6C04) != 0xAB {
		t.Errorf("$6C04 SW ID = 0x%02X, want 0xAB", mem.Read8(0x6C04))
	}
	// Title at $6C08
	for i := 0; i < 12; i++ {
		got := mem.Read8(uint32(0x6C08 + i))
		want := cart[0x24+i]
		if got != want {
			t.Errorf("$%04X title[%d] = 0x%02X, want 0x%02X", 0x6C08+i, i, got, want)
			break
		}
	}
}

func TestInitRAMBootState(t *testing.T) {
	_, _, mem := newHLETestCPUWithCart(t, 256)
	mem.InitSystemState()

	byteChecks := []struct {
		addr uint32
		want uint8
		name string
	}{
		{0x6C14, 0xDC, "checksum low"},
		{0x6C46, 0x01, "cart present"},
		{0x6C55, 0x01, "commercial game"},
		{0x6F83, 0x40, "boot flags"},
		{0x6F84, 0x40, "user boot"},
		{0x6F87, 0x01, "language English"},
	}
	for _, tt := range byteChecks {
		got := mem.Read8(tt.addr)
		if got != tt.want {
			t.Errorf("%s ($%04X) = 0x%02X, want 0x%02X", tt.name, tt.addr, got, tt.want)
		}
	}

	wordChecks := []struct {
		addr uint32
		want uint16
		name string
	}{
		{0x6C7A, 0xA5A5, "boot marker 1"},
		{0x6C7C, 0x5AA5, "boot marker 2"},
		{0x6F80, 0x03FF, "battery voltage"},
	}
	for _, tt := range wordChecks {
		got := mem.Read16(tt.addr)
		if got != tt.want {
			t.Errorf("%s ($%04X) = 0x%04X, want 0x%04X", tt.name, tt.addr, got, tt.want)
		}
	}
}

func TestVBlankHousekeeping(t *testing.T) {
	hle, c := newHLETestCPU(t)
	hle.initVectorTable(hle.mem)

	// Simulate button A pressed: $B0 active-high, bit 4=1 means pressed
	hle.mem.SetInput(0x10) // bit 4 set = A pressed

	// Set up stack frame for RETI
	wantPC := uint32(0x200100)
	wantSR := tlcs900h.SRInitMax | tlcs900h.SRInitSYSM
	regs := c.Registers()
	regs.XSP = 0x6BFA
	c.SetState(regs)
	hle.mem.Write16(0x6BFA, wantSR)
	hle.mem.Write32(0x6BFC, wantPC)

	// Dispatch VBlank (no user handler, default RETI)
	hle.hleIntDispatch(c, 0x2C)

	// Check input was scanned - $6F82 stores active-high (bits 0-6 from $B0)
	// $B0=0x10 (bit 4 set = A pressed) -> $6F82 = 0x10
	input := hle.mem.Read8(0x6F82)
	if input != 0x10 {
		t.Errorf("$6F82 = 0x%02X, want 0x10 (A pressed, active-high)", input)
	}

	// Check $6C5F stores raw value for edge detection
	prev := hle.mem.Read8(0x6C5F)
	if prev != 0x10 {
		t.Errorf("$6C5F = 0x%02X, want 0x10 (raw active-high)", prev)
	}

	// Check battery voltage was set
	batt := hle.mem.Read16(0x6F80)
	if batt != 0x03FF {
		t.Errorf("$6F80 battery = 0x%04X, want 0x03FF", batt)
	}
}

func TestHLEFlashType(t *testing.T) {
	tests := []struct {
		size int
		want uint8
	}{
		{0, 0},
		{0x80000, 1},  // 512 KB = 4Mbit
		{0x100000, 2}, // 1 MB = 8Mbit
		{0x200000, 3}, // 2 MB = 16Mbit
		{0x40000, 1},  // 256 KB = 4Mbit
	}
	for _, tt := range tests {
		got := flashType(tt.size)
		if got != tt.want {
			t.Errorf("flashType(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
}

func TestInitRAMFlashType(t *testing.T) {
	// 1 MB cart should get type 2 (8Mbit)
	_, _, mem := newHLETestCPUWithCart(t, 1024*1024)
	mem.InitSystemState()

	cs0 := mem.Read8(0x6C58)
	if cs0 != 2 {
		t.Errorf("$6C58 CS0 type = %d, want 2 (8Mbit)", cs0)
	}
	cs0copy := mem.Read8(0x6F92)
	if cs0copy != 2 {
		t.Errorf("$6F92 CS0 type copy = %d, want 2", cs0copy)
	}
}

func TestInitRAMSpriteTable(t *testing.T) {
	_, _, mem := newHLETestCPUWithCart(t, 256)
	mem.InitSystemState()

	// Check sprite table 1 at $9000 has $0020 words
	for i := uint32(0); i < 0x200; i += 2 {
		got := mem.Read16(0x9000 + i)
		if got != 0x0020 {
			t.Errorf("sprite1[$%03X] = 0x%04X, want 0x0020", i, got)
			break
		}
	}
	// Check sprite table 2 at $9800
	for i := uint32(0); i < 0x200; i += 2 {
		got := mem.Read16(0x9800 + i)
		if got != 0x0020 {
			t.Errorf("sprite2[$%03X] = 0x%04X, want 0x0020", i, got)
			break
		}
	}
}

func TestInitMonoModeSet(t *testing.T) {
	// Create a mono cart (system code $00 at offset $23)
	cart := make([]byte, 256)
	cart[0x23] = 0x00 // monochrome

	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	mem.InitSystemState()

	// $87E2 should be $80 (K1GE mode)
	mode := mem.Read8(0x87E2)
	if mode != 0x80 {
		t.Errorf("$87E2 = 0x%02X, want 0x80 (K1GE mode)", mode)
	}
	// $6F95 should be $00 (mono flag)
	flag := mem.Read8(0x6F95)
	if flag != 0x00 {
		t.Errorf("$6F95 = 0x%02X, want 0x00 (mono)", flag)
	}
}

func TestInitColorModeSet(t *testing.T) {
	// Create a color cart (system code $10 at offset $23)
	cart := make([]byte, 256)
	cart[0x23] = 0x10 // color

	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	mem.InitSystemState()

	// $87E2 should be $00 (K2GE color mode)
	mode := mem.Read8(0x87E2)
	if mode != 0x00 {
		t.Errorf("$87E2 = 0x%02X, want 0x00 (K2GE color)", mode)
	}
	// $6F95 should be $10 (color flag)
	flag := mem.Read8(0x6F95)
	if flag != 0x10 {
		t.Errorf("$6F95 = 0x%02X, want 0x10 (color)", flag)
	}
}

func TestInitK1GEPalettesMono(t *testing.T) {
	// Create a mono cart
	cart := make([]byte, 256)
	cart[0x23] = 0x00 // monochrome

	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	mem.InitSystemState()

	// Grayscale ramp: 8 shades per group, 16-bit LE
	wantShades := [8]uint16{
		0x0FFF, 0x0DDD, 0x0BBB, 0x0999,
		0x0777, 0x0444, 0x0333, 0x0000,
	}

	// Background color ($83E0) should be white
	bgColor := mem.Read16(0x83E0)
	if bgColor != 0x0FFF {
		t.Errorf("$83E0 BG color = 0x%04X, want 0x0FFF (white)", bgColor)
	}
	// Window color ($83F0) should be white
	winColor := mem.Read16(0x83F0)
	if winColor != 0x0FFF {
		t.Errorf("$83F0 window color = 0x%04X, want 0x0FFF (white)", winColor)
	}

	// Check all three K1GE compat palette areas
	bases := []struct {
		addr uint32
		name string
	}{
		{0x8380, "sprite"},
		{0x83A0, "plane1"},
		{0x83C0, "plane2"},
	}
	for _, base := range bases {
		for group := uint32(0); group < 2; group++ {
			for i, want := range wantShades {
				addr := base.addr + group*16 + uint32(i)*2
				got := mem.Read16(addr)
				if got != want {
					t.Errorf("%s group %d shade %d ($%04X) = 0x%04X, want 0x%04X",
						base.name, group, i, addr, got, want)
				}
			}
		}
	}
}

func TestInitK1GEPalettesColor(t *testing.T) {
	// Create a color cart
	cart := make([]byte, 256)
	cart[0x23] = 0x10 // color

	mem, err := NewMemory(cart, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	mem.InitSystemState()

	// K1GE compat palettes should NOT be filled for color games
	for i := uint32(0); i < 0x60; i++ {
		got := mem.Read8(0x8380 + i)
		if got != 0x00 {
			t.Errorf("palette $%04X = 0x%02X, want 0x00 (color game, should not fill)", 0x8380+i, got)
			break
		}
	}
}
