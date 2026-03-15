package tlcs900h

// Bus provides memory access for the CPU.
// All I/O on the TLCS-900/H is memory-mapped; there is no separate I/O space.
// Addresses are 24-bit (masked internally by the CPU before calling Bus methods).
type Bus interface {
	Read8(addr uint32) uint8
	Read16(addr uint32) uint16
	Read32(addr uint32) uint32
	Write8(addr uint32, val uint8)
	Write16(addr uint32, val uint16)
	Write32(addr uint32, val uint32)
	Reset()
}
