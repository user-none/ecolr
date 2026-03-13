package tlcs900h

// Control instructions: NOP, HALT, EI, DI, SWI, MIRR.
// PUSH SR, POP SR, LDF, LINK, UNLK, INCF, DECF are in ops_load.go.

func init() {
	// NOP (0x00) - 2 cycles
	baseOps[0x00] = func(c *CPU, op uint8) {
		c.cycles += 2
	}

	// HALT (0x05) - 6 cycles
	baseOps[0x05] = func(c *CPU, op uint8) {
		c.halted = true
		c.cycles += 6
	}

	// EI/DI (0x06) - EI #n sets IFF to n (0-6, 3 cycles), DI sets IFF to 7 (4 cycles)
	baseOps[0x06] = func(c *CPU, op uint8) {
		n := c.fetchPC() & 0x07
		sr := c.reg.SR
		sr &^= srIFFMask
		sr |= uint16(n) << srIFFShift
		c.setSR(sr)
		if n == 7 {
			c.cycles += 4
		} else {
			c.cycles += 3
		}
	}

	// SWI #n (0xF8-0xFF) - Software interrupt, 19 cycles
	// Push SR (word) and PC (long), load PC from vector table.
	// Stack layout per ISA: (XSP) = SR, (XSP+2) = 32-bit PC.
	for i := 0; i < 8; i++ {
		baseOps[0xF8+i] = func(c *CPU, op uint8) {
			num := uint32(op & 0x07)
			c.reg.XSP -= 6
			c.writeBus(Word, c.reg.XSP, uint32(c.reg.SR))
			c.writeBus(Long, c.reg.XSP+2, c.reg.PC)
			vecAddr := uint32(0xFFFF00) + num*4
			c.reg.PC = c.readBus(Long, vecAddr) & addrMask
			c.cycles += 19
		}
	}

	// MIRR r (D8+r : 0x16) - Bit reversal, word only, 3 cycles
	regOps[0x16] = func(c *CPU, op uint8) {
		val := uint16(c.readOpReg())
		val = (val&0xFF00)>>8 | (val&0x00FF)<<8
		val = (val&0xF0F0)>>4 | (val&0x0F0F)<<4
		val = (val&0xCCCC)>>2 | (val&0x3333)<<2
		val = (val&0xAAAA)>>1 | (val&0x5555)<<1
		c.writeOpReg(uint32(val))
		c.cycles += 3
	}
}
