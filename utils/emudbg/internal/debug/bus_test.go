package debug

import (
	"testing"

	"github.com/user-none/ecolr/core"
	"github.com/user-none/ecolr/core/tlcs900h"
)

func newTestBus(t *testing.T) (*DebugBus, *core.Memory) {
	t.Helper()
	mem, err := core.NewMemory(nil, nil)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	// Provide a minimal BIOS so Reset works.
	bios := make([]byte, 0x10000)
	// Set reset vector at $FFFF00 to point somewhere valid in BIOS range.
	// The CPU reads the initial PC from $FFFF00 (long).
	bios[0xFF00] = 0x00
	bios[0xFF01] = 0x00
	bios[0xFF02] = 0xFF
	bios[0xFF03] = 0x00
	mem.SetBIOS(bios)
	bus := NewDebugBus(mem)
	return bus, mem
}

func TestPinByteRead(t *testing.T) {
	bus, mem := newTestBus(t)

	// Write a known value to work RAM ($4000).
	mem.Write(tlcs900h.Byte, 0x4000, 0xAB)

	// Without pin, should read the actual value.
	got := bus.Read(tlcs900h.Byte, 0x4000)
	if got != 0xAB {
		t.Fatalf("expected 0xAB, got 0x%02X", got)
	}

	// Pin to a different value.
	bus.Pin(0x4000, 0x42)
	got = bus.Read(tlcs900h.Byte, 0x4000)
	if got != 0x42 {
		t.Fatalf("expected pinned 0x42, got 0x%02X", got)
	}

	// Unpin should restore original.
	bus.Unpin(0x4000)
	got = bus.Read(tlcs900h.Byte, 0x4000)
	if got != 0xAB {
		t.Fatalf("expected restored 0xAB, got 0x%02X", got)
	}
}

func TestPinWordRead(t *testing.T) {
	bus, mem := newTestBus(t)

	// Write $CDAB to work RAM ($4000-$4001) little-endian.
	mem.Write(tlcs900h.Word, 0x4000, 0xCDAB)

	// Pin only the high byte ($4001).
	bus.Pin(0x4001, 0xFF)
	got := bus.Read(tlcs900h.Word, 0x4000)
	if got != 0xFFAB {
		t.Fatalf("expected 0xFFAB, got 0x%04X", got)
	}
}

func TestMirrorByteRead(t *testing.T) {
	bus, mem := newTestBus(t)

	// Write source value.
	mem.Write(tlcs900h.Byte, 0x4010, 0x99)

	// Mirror $4020 -> $4010.
	bus.Mirror(0x4020, 0x4010)
	got := bus.Read(tlcs900h.Byte, 0x4020)
	if got != 0x99 {
		t.Fatalf("expected mirrored 0x99, got 0x%02X", got)
	}

	// Unmirror should read actual value at $4020.
	bus.Unmirror(0x4020)
	got = bus.Read(tlcs900h.Byte, 0x4020)
	if got != 0x00 {
		t.Fatalf("expected 0x00, got 0x%02X", got)
	}
}

func TestPinOverridesMirror(t *testing.T) {
	bus, mem := newTestBus(t)

	mem.Write(tlcs900h.Byte, 0x4010, 0x77)
	bus.Mirror(0x4020, 0x4010)
	bus.Pin(0x4020, 0x33)

	// Pin should take priority over mirror.
	got := bus.Read(tlcs900h.Byte, 0x4020)
	if got != 0x33 {
		t.Fatalf("expected pinned 0x33 over mirror, got 0x%02X", got)
	}
}

func TestMirrorWordRead(t *testing.T) {
	bus, mem := newTestBus(t)

	// Source bytes at $4010 and $4011.
	mem.Write(tlcs900h.Byte, 0x4010, 0x11)
	mem.Write(tlcs900h.Byte, 0x4011, 0x22)

	// Mirror two bytes: $4020->$4010, $4021->$4011.
	bus.Mirror(0x4020, 0x4010)
	bus.Mirror(0x4021, 0x4011)

	got := bus.Read(tlcs900h.Word, 0x4020)
	if got != 0x2211 {
		t.Fatalf("expected 0x2211, got 0x%04X", got)
	}
}

func TestNoOverlayPassthrough(t *testing.T) {
	bus, mem := newTestBus(t)

	mem.Write(tlcs900h.Long, 0x4000, 0xDEADBEEF)
	got := bus.Read(tlcs900h.Long, 0x4000)
	if got != 0xDEADBEEF {
		t.Fatalf("expected 0xDEADBEEF, got 0x%08X", got)
	}
}
