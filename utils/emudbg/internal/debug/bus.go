package debug

import (
	"github.com/user-none/ecolr/core"
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

// overlayByte returns the pinned, mirrored, or original value for a single byte.
func (d *DebugBus) overlayByte(addr uint32, val uint8) uint8 {
	if pv, ok := d.pins[addr]; ok {
		return pv
	}
	if src, ok := d.mirrors[addr]; ok {
		return d.Memory.Read8(src)
	}
	return val
}

// Read8 implements tlcs900h.Bus.
func (d *DebugBus) Read8(addr uint32) uint8 {
	return d.overlayByte(addr, d.Memory.Read8(addr))
}

// Read16 implements tlcs900h.Bus.
func (d *DebugBus) Read16(addr uint32) uint16 {
	val := d.Memory.Read16(addr)
	lo := d.overlayByte(addr, uint8(val))
	hi := d.overlayByte(addr+1, uint8(val>>8))
	return uint16(lo) | uint16(hi)<<8
}

// Read32 implements tlcs900h.Bus.
func (d *DebugBus) Read32(addr uint32) uint32 {
	val := d.Memory.Read32(addr)
	b0 := d.overlayByte(addr, uint8(val))
	b1 := d.overlayByte(addr+1, uint8(val>>8))
	b2 := d.overlayByte(addr+2, uint8(val>>16))
	b3 := d.overlayByte(addr+3, uint8(val>>24))
	return uint32(b0) | uint32(b1)<<8 | uint32(b2)<<16 | uint32(b3)<<24
}
