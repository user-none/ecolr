package debug

import (
	"github.com/user-none/ecolr/core"
	"github.com/user-none/ecolr/core/tlcs900h"
)

// DebugBus wraps *core.Memory to intercept CPU reads with pin and mirror
// support. Pins force a specific byte value for reads at an address.
// Mirrors redirect reads of one address to read from another address.
// Pins take priority over mirrors. Writes are never intercepted.
type DebugBus struct {
	*core.Memory
	pins    map[uint32]uint8
	mirrors map[uint32]uint32
}

// NewDebugBus creates a DebugBus wrapping the given Memory.
func NewDebugBus(mem *core.Memory) *DebugBus {
	return &DebugBus{
		Memory:  mem,
		pins:    make(map[uint32]uint8),
		mirrors: make(map[uint32]uint32),
	}
}

// Pin forces reads of addr to return val (byte granularity).
func (d *DebugBus) Pin(addr uint32, val uint8) {
	d.pins[addr] = val
}

// Unpin removes a pinned address.
func (d *DebugBus) Unpin(addr uint32) {
	delete(d.pins, addr)
}

// Mirror makes reads of addr return the value at srcAddr instead.
func (d *DebugBus) Mirror(addr, srcAddr uint32) {
	d.mirrors[addr] = srcAddr
}

// Unmirror removes a mirror.
func (d *DebugBus) Unmirror(addr uint32) {
	delete(d.mirrors, addr)
}

// Read implements tlcs900h.Bus. It delegates to Memory.Read then overlays
// pinned or mirrored bytes.
func (d *DebugBus) Read(op tlcs900h.Size, addr uint32) uint32 {
	val := d.Memory.Read(op, addr)
	return d.overlay(op, addr, val)
}

// overlay replaces individual bytes in val that are pinned or mirrored.
func (d *DebugBus) overlay(op tlcs900h.Size, addr uint32, val uint32) uint32 {
	for i := tlcs900h.Size(0); i < op; i++ {
		a := addr + uint32(i)
		shift := i * 8
		if pv, ok := d.pins[a]; ok {
			val = (val &^ (0xFF << shift)) | (uint32(pv) << shift)
		} else if src, ok := d.mirrors[a]; ok {
			mv := d.Memory.Read(tlcs900h.Byte, src)
			val = (val &^ (0xFF << shift)) | (mv << shift)
		}
	}
	return val
}
