package core

import "testing"

func TestZ80BusRAMReadWrite(t *testing.T) {
	m := makeTestMemory(t)

	bus := m.z80bus

	// Write via Z80 bus, read back
	bus.Write(0x0000, 0xAB)
	if got := bus.Read(0x0000); got != 0xAB {
		t.Errorf("Z80 bus read $0000 = 0x%02X, want 0xAB", got)
	}

	bus.Write(0x0FFF, 0xCD)
	if got := bus.Read(0x0FFF); got != 0xCD {
		t.Errorf("Z80 bus read $0FFF = 0x%02X, want 0xCD", got)
	}

	// Verify shared with Memory.z80RAM (TLCS-900H $7000-$7FFF)
	if m.z80RAM[0x0000] != 0xAB {
		t.Errorf("z80RAM[0] = 0x%02X, want 0xAB", m.z80RAM[0x0000])
	}
	if m.z80RAM[0x0FFF] != 0xCD {
		t.Errorf("z80RAM[$FFF] = 0x%02X, want 0xCD", m.z80RAM[0x0FFF])
	}

	// Write via TLCS-900H side, read from Z80 bus
	m.z80RAM[0x0100] = 0xEF
	if got := bus.Read(0x0100); got != 0xEF {
		t.Errorf("Z80 bus read after TLCS write = 0x%02X, want 0xEF", got)
	}

	// Fetch delegates to Read
	if got := bus.Fetch(0x0000); got != 0xAB {
		t.Errorf("Z80 bus fetch $0000 = 0x%02X, want 0xAB", got)
	}
}

func TestZ80BusPSGWrite(t *testing.T) {
	psg := NewT6W28(3072000, 48000, 2048)
	m, err := NewMemory(nil, psg)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}

	bus := m.z80bus

	// Write to PSG ports should not panic
	bus.Write(0x4000, 0x9F) // right channel
	bus.Write(0x4001, 0x9F) // left channel
}

func TestZ80BusCommByte(t *testing.T) {
	m := makeTestMemory(t)
	bus := m.z80bus

	// Write via Z80 bus at $8000
	bus.Write(0x8000, 0x42)
	if got := bus.Read(0x8000); got != 0x42 {
		t.Errorf("Z80 comm byte read = 0x%02X, want 0x42", got)
	}

	// Verify shared with TLCS-900H $BC (customIO offset $3C)
	if m.customIO[0x3C] != 0x42 {
		t.Errorf("customIO[$3C] = 0x%02X, want 0x42", m.customIO[0x3C])
	}

	// Write from TLCS-900H side, read from Z80
	m.customIO[0x3C] = 0x99
	if got := bus.Read(0x8000); got != 0x99 {
		t.Errorf("Z80 comm byte after TLCS write = 0x%02X, want 0x99", got)
	}
}

func TestZ80BusUnmapped(t *testing.T) {
	m := makeTestMemory(t)
	bus := m.z80bus

	addrs := []uint16{0x1000, 0x2000, 0x3FFF, 0x4002, 0x5000, 0x7FFF, 0x8001, 0xBFFF, 0xC001, 0xFFFF}
	for _, addr := range addrs {
		if got := bus.Read(addr); got != 0 {
			t.Errorf("Z80 bus read unmapped $%04X = 0x%02X, want 0x00", addr, got)
		}
	}
}

func TestZ80BusPSGReadZero(t *testing.T) {
	m := makeTestMemory(t)
	bus := m.z80bus

	if got := bus.Read(0x4000); got != 0 {
		t.Errorf("Z80 bus read $4000 = 0x%02X, want 0x00", got)
	}
	if got := bus.Read(0x4001); got != 0 {
		t.Errorf("Z80 bus read $4001 = 0x%02X, want 0x00", got)
	}
}

func TestZ80BusIONoOp(t *testing.T) {
	m := makeTestMemory(t)
	bus := m.z80bus

	if got := bus.In(0x00); got != 0 {
		t.Errorf("Z80 bus In(0) = 0x%02X, want 0x00", got)
	}
	if got := bus.In(0xFF); got != 0 {
		t.Errorf("Z80 bus In($FF) = 0x%02X, want 0x00", got)
	}

	// Out should not panic
	bus.Out(0x00, 0xFF)
	bus.Out(0xFF, 0x00)
}

func TestZ80BusINT5Trigger(t *testing.T) {
	m := makeTestMemory(t)
	bus := m.z80bus

	// Configure INT5 with priority level 3 in INTE45 (reg index 1, high source)
	// High source priority is bits 6-4, pending is bit 7
	m.intc.WriteReg(1, 0x30) // priority 3, no pending

	// Z80 write to $C000 should set INT5 pending
	bus.Write(0xC000, 0x00)

	// Check that pending bit 7 of reg 1 is set
	reg := m.intc.ReadReg(1)
	if reg&0x80 == 0 {
		t.Errorf("INT5 pending not set after Z80 $C000 write: reg=$%02X", reg)
	}
}
