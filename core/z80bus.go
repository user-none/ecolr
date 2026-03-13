package core

// Z80Bus implements z80.Bus for the NGPC sound CPU memory map.
//
// Address map (Z80 16-bit view):
//
//	$0000-$0FFF  Shared RAM (maps to TLCS-900H $7000-$7FFF)
//	$4000        T6W28 right channel write port (write-only)
//	$4001        T6W28 left channel write port (write-only)
//	$8000        Communication byte (shared with TLCS-900H $BC)
//	$C000        Write triggers TLCS-900H INT5
//
// No mirroring. Exact 16-bit decode. Unmapped reads return 0.
type Z80Bus struct {
	ram  *[z80RAMSize]byte
	psg  *T6W28
	comm *[customIOSize]byte
	intc *IntC
}

// commOffset is the offset of the communication byte ($BC) within customIO.
const commOffset = 0xBC - 0x80

func (b *Z80Bus) Fetch(addr uint16) uint8 {
	return b.Read(addr)
}

func (b *Z80Bus) Read(addr uint16) uint8 {
	switch {
	case addr <= 0x0FFF:
		return b.ram[addr]
	case addr == 0x8000:
		return b.comm[commOffset]
	}
	return 0
}

func (b *Z80Bus) Write(addr uint16, val uint8) {
	switch {
	case addr <= 0x0FFF:
		b.ram[addr] = val
	case addr == 0x4000:
		if b.psg != nil {
			b.psg.WriteRight(val)
		}
	case addr == 0x4001:
		if b.psg != nil {
			b.psg.WriteLeft(val)
		}
	case addr == 0x8000:
		b.comm[commOffset] = val
	case addr == 0xC000:
		b.intc.SetPending(1, true)
	}
}

func (b *Z80Bus) In(port uint16) uint8 {
	return 0
}

func (b *Z80Bus) Out(port uint16, val uint8) {
}
