package tlcs900h

// Bus provides memory access for the CPU.
// All I/O on the TLCS-900/H is memory-mapped; there is no separate I/O space.
// Addresses are 24-bit (masked internally by the CPU before calling Bus methods).
type Bus interface {
	Read(op Size, addr uint32) uint32
	Write(op Size, addr uint32, val uint32)
	Reset()
}
