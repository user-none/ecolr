package core

import (
	"encoding/binary"
	"testing"

	"github.com/user-none/eblitui/coreif"
	"github.com/user-none/ecolr/core/tlcs900h"
)

// makeSerializeEmulator creates an emulator suitable for serialization tests.
// It uses a real BIOS (minimal loop) and a small cart so GetROMCRC32 is non-zero.
func makeSerializeEmulator(t *testing.T) Emulator {
	t.Helper()
	bios := make([]byte, biosROMSize)
	// Reset vector at offset 0xFF00 -> address $FFFF00 points to $FF0010
	bios[0xFF00] = 0x10
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00
	// Place JR $-2 at $FF0010 (offset 0x0010) to loop forever.
	bios[0x0010] = 0xC8
	bios[0x0011] = 0xFE

	cart := make([]byte, 256)
	cart[0x1C] = 0x40
	cart[0x1D] = 0x00
	cart[0x1E] = 0x20
	cart[0x1F] = 0x00
	// Put some identifiable data in the cart
	for i := 0x40; i < len(cart); i++ {
		cart[i] = uint8(i)
	}

	emu, err := NewEmulator(cart, coreif.RegionNTSC)
	if err != nil {
		t.Fatalf("makeSerializeEmulator: %v", err)
	}
	emu.SetBIOS("system_bios", bios)
	return emu
}

func TestSerializeSizeMatchesOutput(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	expected := SerializeSize()
	if len(data) != expected {
		t.Errorf("Serialize output length = %d, want %d", len(data), expected)
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Run a frame to get some state changes
	emu.RunFrame()

	// Set some distinctive memory values
	emu.mem.Write(tlcs900h.Byte, 0x4000, 0xAB) // work RAM
	emu.mem.Write(tlcs900h.Byte, 0x4001, 0xCD)
	emu.mem.Write(tlcs900h.Byte, 0xA2, 0xF0) // DAC left
	emu.mem.Write(tlcs900h.Byte, 0xA3, 0x0F) // DAC right

	// Capture state
	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Record key values
	cpuRegs := emu.cpu.Registers()
	dacL, dacR := emu.mem.DACValues()
	workByte0 := emu.mem.workRAM[0]
	workByte1 := emu.mem.workRAM[1]
	intcReg0 := emu.mem.intc.ReadReg(0)
	lastCycle := emu.mem.lastCycle

	// Modify state
	emu.RunFrame()
	emu.mem.Write(tlcs900h.Byte, 0x4000, 0x00)
	emu.mem.Write(tlcs900h.Byte, 0xA2, 0x80)

	// Restore state
	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	// Verify key values match
	gotRegs := emu.cpu.Registers()
	if gotRegs.PC != cpuRegs.PC {
		t.Errorf("PC = 0x%06X, want 0x%06X", gotRegs.PC, cpuRegs.PC)
	}
	if gotRegs.SR != cpuRegs.SR {
		t.Errorf("SR = 0x%04X, want 0x%04X", gotRegs.SR, cpuRegs.SR)
	}

	gotDacL, gotDacR := emu.mem.DACValues()
	if gotDacL != dacL {
		t.Errorf("dacL = 0x%02X, want 0x%02X", gotDacL, dacL)
	}
	if gotDacR != dacR {
		t.Errorf("dacR = 0x%02X, want 0x%02X", gotDacR, dacR)
	}

	if emu.mem.workRAM[0] != workByte0 {
		t.Errorf("workRAM[0] = 0x%02X, want 0x%02X", emu.mem.workRAM[0], workByte0)
	}
	if emu.mem.workRAM[1] != workByte1 {
		t.Errorf("workRAM[1] = 0x%02X, want 0x%02X", emu.mem.workRAM[1], workByte1)
	}

	if emu.mem.intc.ReadReg(0) != intcReg0 {
		t.Errorf("intc.regs[0] = 0x%02X, want 0x%02X", emu.mem.intc.ReadReg(0), intcReg0)
	}

	if emu.mem.lastCycle != lastCycle {
		t.Errorf("lastCycle = %d, want %d", emu.mem.lastCycle, lastCycle)
	}
}

func TestSerializeVerifyWrongMagic(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Corrupt magic
	data[0] = 'X'

	err = emu.VerifyState(data)
	if err == nil {
		t.Error("expected error for wrong magic")
	}
}

func TestSerializeVerifyWrongVersion(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Set version to future version
	binary.LittleEndian.PutUint16(data[12:14], stateVersion+1)

	err = emu.VerifyState(data)
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestSerializeVerifyWrongROMCRC(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Corrupt ROM CRC
	binary.LittleEndian.PutUint32(data[14:18], 0xDEADBEEF)

	err = emu.VerifyState(data)
	if err == nil {
		t.Error("expected error for wrong ROM CRC")
	}
}

func TestSerializeVerifyCorruptedData(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Corrupt data after header
	data[stateHeaderSize+10] ^= 0xFF

	err = emu.VerifyState(data)
	if err == nil {
		t.Error("expected error for corrupted data CRC")
	}
}

func TestSerializeVerifyTooShort(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	err = emu.VerifyState(data[:stateHeaderSize-1])
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

// ReadMemory tests

func TestReadMemoryWorkRAM(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Write known values to work RAM
	emu.mem.workRAM[0] = 0xAA
	emu.mem.workRAM[1] = 0xBB
	emu.mem.workRAM[workRAMSize-1] = 0xCC

	// Read first two bytes
	buf := make([]byte, 2)
	n := emu.ReadMemory(0x0000, buf)
	if n != 2 {
		t.Fatalf("ReadMemory returned %d, want 2", n)
	}
	if buf[0] != 0xAA || buf[1] != 0xBB {
		t.Errorf("buf = %v, want [0xAA, 0xBB]", buf)
	}

	// Read last byte of work RAM
	buf = make([]byte, 1)
	n = emu.ReadMemory(workRAMFlatEnd, buf)
	if n != 1 {
		t.Fatalf("ReadMemory returned %d, want 1", n)
	}
	if buf[0] != 0xCC {
		t.Errorf("buf[0] = 0x%02X, want 0xCC", buf[0])
	}
}

func TestReadMemoryZ80RAM(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	emu.mem.z80RAM[0] = 0x11
	emu.mem.z80RAM[z80RAMSize-1] = 0x22

	buf := make([]byte, 1)
	n := emu.ReadMemory(z80RAMFlatStart, buf)
	if n != 1 || buf[0] != 0x11 {
		t.Errorf("z80RAM[0]: n=%d, buf[0]=0x%02X, want 1/0x11", n, buf[0])
	}

	n = emu.ReadMemory(z80RAMFlatEnd, buf)
	if n != 1 || buf[0] != 0x22 {
		t.Errorf("z80RAM[end]: n=%d, buf[0]=0x%02X, want 1/0x22", n, buf[0])
	}
}

func TestReadMemoryK2GE(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	emu.mem.k2ge[0] = 0x33
	emu.mem.k2ge[k2geSize-1] = 0x44

	buf := make([]byte, 1)
	n := emu.ReadMemory(k2geFlatStart, buf)
	if n != 1 || buf[0] != 0x33 {
		t.Errorf("k2ge[0]: n=%d, buf[0]=0x%02X, want 1/0x33", n, buf[0])
	}

	n = emu.ReadMemory(k2geFlatEnd, buf)
	if n != 1 || buf[0] != 0x44 {
		t.Errorf("k2ge[end]: n=%d, buf[0]=0x%02X, want 1/0x44", n, buf[0])
	}
}

func TestReadMemoryCrossRegion(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Set boundary values
	emu.mem.workRAM[workRAMSize-1] = 0xAA
	emu.mem.z80RAM[0] = 0xBB

	// Read across work RAM -> Z80 RAM boundary
	buf := make([]byte, 2)
	n := emu.ReadMemory(workRAMFlatEnd, buf)
	if n != 2 {
		t.Fatalf("ReadMemory cross-region returned %d, want 2", n)
	}
	if buf[0] != 0xAA || buf[1] != 0xBB {
		t.Errorf("cross-region buf = [0x%02X, 0x%02X], want [0xAA, 0xBB]", buf[0], buf[1])
	}
}

func TestReadMemoryOutOfRange(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Read past end of K2GE
	buf := make([]byte, 2)
	n := emu.ReadMemory(k2geFlatEnd, buf)
	if n != 1 {
		t.Errorf("ReadMemory past k2ge returned %d, want 1", n)
	}

	// Read completely out of range
	n = emu.ReadMemory(k2geFlatEnd+1, buf)
	if n != 0 {
		t.Errorf("ReadMemory out of range returned %d, want 0", n)
	}
}

func TestSerializeTimerState(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Set some timer state
	emu.mem.timers.trun = 0x83
	emu.mem.timers.treg[0] = 0x42
	emu.mem.timers.prescaler = 12345

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Mutate
	emu.mem.timers.trun = 0
	emu.mem.timers.treg[0] = 0
	emu.mem.timers.prescaler = 0

	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if emu.mem.timers.trun != 0x83 {
		t.Errorf("trun = 0x%02X, want 0x83", emu.mem.timers.trun)
	}
	if emu.mem.timers.treg[0] != 0x42 {
		t.Errorf("treg[0] = 0x%02X, want 0x42", emu.mem.timers.treg[0])
	}
	if emu.mem.timers.prescaler != 12345 {
		t.Errorf("prescaler = %d, want 12345", emu.mem.timers.prescaler)
	}
}

func TestSerializeRTCState(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Set distinctive RTC state
	emu.mem.rtc.ctrl = 0x01
	emu.mem.rtc.regs[0] = 0x26 // year 26
	emu.mem.rtc.regs[5] = 0x59 // second 59
	emu.mem.rtc.cyclesLeft = 999

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Mutate
	emu.mem.rtc.ctrl = 0
	emu.mem.rtc.regs[0] = 0
	emu.mem.rtc.cyclesLeft = 0

	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if emu.mem.rtc.ctrl != 0x01 {
		t.Errorf("rtc.ctrl = 0x%02X, want 0x01", emu.mem.rtc.ctrl)
	}
	if emu.mem.rtc.regs[0] != 0x26 {
		t.Errorf("rtc.regs[0] = 0x%02X, want 0x26", emu.mem.rtc.regs[0])
	}
	if emu.mem.rtc.cyclesLeft != 999 {
		t.Errorf("rtc.cyclesLeft = %d, want 999", emu.mem.rtc.cyclesLeft)
	}
}

func TestSerializeADCState(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	emu.mem.adc.admod = 0x20
	emu.mem.adc.result[0] = 0x3FF
	emu.mem.adc.an[1] = 0x200
	emu.mem.adc.cyclesLeft = 160

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	emu.mem.adc.admod = 0
	emu.mem.adc.result[0] = 0
	emu.mem.adc.an[1] = 0
	emu.mem.adc.cyclesLeft = 0

	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if emu.mem.adc.admod != 0x20 {
		t.Errorf("admod = 0x%02X, want 0x20", emu.mem.adc.admod)
	}
	if emu.mem.adc.result[0] != 0x3FF {
		t.Errorf("result[0] = 0x%03X, want 0x3FF", emu.mem.adc.result[0])
	}
	if emu.mem.adc.an[1] != 0x200 {
		t.Errorf("an[1] = 0x%03X, want 0x200", emu.mem.adc.an[1])
	}
	if emu.mem.adc.cyclesLeft != 160 {
		t.Errorf("cyclesLeft = %d, want 160", emu.mem.adc.cyclesLeft)
	}
}

func TestSerializeFlashState(t *testing.T) {
	emu := makeSerializeEmulator(t)
	emu.Start()

	// Put flash in ID mode
	emu.mem.cs0.state = flashIDMode

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	emu.mem.cs0.state = flashRead

	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	if emu.mem.cs0.state != flashIDMode {
		t.Errorf("cs0.state = %d, want %d (flashIDMode)", emu.mem.cs0.state, flashIDMode)
	}
}

func TestSerializeIntCHasPending(t *testing.T) {
	emu := makeSerializeEmulator(t)

	// Set a pending interrupt so hasPending becomes true.
	emu.mem.intc.SetPending(3, false)
	if !emu.mem.intc.hasPending {
		t.Fatal("hasPending should be true after SetPending")
	}

	data, err := emu.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Clear the pending bit so hasPending is false before deserialize.
	emu.mem.intc.Reset()
	if emu.mem.intc.hasPending {
		t.Fatal("hasPending should be false after Reset")
	}

	err = emu.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	// After deserialize, hasPending must reflect the restored pending bits.
	if !emu.mem.intc.hasPending {
		t.Error("hasPending not restored after Deserialize")
	}

	// Verify the pending bit is actually present in the register.
	reg := emu.mem.intc.ReadReg(3)
	if reg&0x08 == 0 {
		t.Errorf("IntC reg 3 pending bit not restored: got %#02x", reg)
	}
}
