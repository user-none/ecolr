package tlcs900h

import "testing"

// testBus provides a simple memory-mapped bus for testing.
// 16 MB address space matching the 24-bit TLCS-900/H addressing.
type testBus struct {
	mem [16 * 1024 * 1024]byte
}

func (b *testBus) Read(sz Size, addr uint32) uint32 {
	addr &= 0xFFFFFF
	switch sz {
	case Byte:
		return uint32(b.mem[addr])
	case Word:
		return uint32(b.mem[addr]) | uint32(b.mem[addr+1])<<8
	case Long:
		return uint32(b.mem[addr]) | uint32(b.mem[addr+1])<<8 |
			uint32(b.mem[addr+2])<<16 | uint32(b.mem[addr+3])<<24
	}
	return 0
}

func (b *testBus) Write(sz Size, addr uint32, val uint32) {
	addr &= 0xFFFFFF
	switch sz {
	case Byte:
		b.mem[addr] = uint8(val)
	case Word:
		b.mem[addr] = uint8(val)
		b.mem[addr+1] = uint8(val >> 8)
	case Long:
		b.mem[addr] = uint8(val)
		b.mem[addr+1] = uint8(val >> 8)
		b.mem[addr+2] = uint8(val >> 16)
		b.mem[addr+3] = uint8(val >> 24)
	}
}

func (b *testBus) Reset() {}

// write8 writes a byte to the test bus memory.
func (b *testBus) write8(addr uint32, val uint8) {
	b.mem[addr&0xFFFFFF] = val
}

// write16LE writes a 16-bit little-endian value.
func (b *testBus) write16LE(addr uint32, val uint16) {
	addr &= 0xFFFFFF
	b.mem[addr] = uint8(val)
	b.mem[addr+1] = uint8(val >> 8)
}

// write32LE writes a 32-bit little-endian value.
func (b *testBus) write32LE(addr uint32, val uint32) {
	addr &= 0xFFFFFF
	b.mem[addr] = uint8(val)
	b.mem[addr+1] = uint8(val >> 8)
	b.mem[addr+2] = uint8(val >> 16)
	b.mem[addr+3] = uint8(val >> 24)
}

// newTestCPU creates a CPU with a test bus, sets the reset vector,
// and performs a full reset to load it.
func newTestCPU(t *testing.T, pc uint32) (*CPU, *testBus) {
	t.Helper()
	bus := &testBus{}
	// Write reset vector at 0xFFFF00
	bus.write32LE(0xFFFF00, pc)
	cpu := New(bus)
	cpu.LoadResetVector()
	return cpu, bus
}

// checkReg32 verifies a 32-bit register value.
func checkReg32(t *testing.T, name string, got, want uint32) {
	t.Helper()
	if got != want {
		t.Errorf("%s = 0x%08X, want 0x%08X", name, got, want)
	}
}

// checkReg16 verifies a 16-bit register value.
func checkReg16(t *testing.T, name string, got, want uint16) {
	t.Helper()
	if got != want {
		t.Errorf("%s = 0x%04X, want 0x%04X", name, got, want)
	}
}

// checkReg8 verifies an 8-bit register value.
func checkReg8(t *testing.T, name string, got, want uint8) {
	t.Helper()
	if got != want {
		t.Errorf("%s = 0x%02X, want 0x%02X", name, got, want)
	}
}
