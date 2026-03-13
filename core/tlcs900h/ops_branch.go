package tlcs900h

// Branch, call, and return instructions.
// JP, JR, JRL, CALL, CALR, RET, RETD, RETI, DJNZ, SCC

func init() {
	// RETI: baseOps[0x07]
	// Restore SR and PC from stack. SR at XSP (word), PC at XSP+2 (long).
	baseOps[0x07] = func(c *CPU, op uint8) {
		sr := uint16(c.readBus(Word, c.reg.XSP))
		pc := c.readBus(Long, c.reg.XSP+2) & addrMask
		c.reg.XSP += 6
		c.setSR(sr)
		c.reg.PC = pc
		if c.intNest > 0 {
			c.intNest--
		}
		c.cycles += 12
	}

	// RET: baseOps[0x0E]
	baseOps[0x0E] = func(c *CPU, op uint8) {
		c.reg.PC = c.pop(Long) & addrMask
		c.cycles += 9
	}

	// RETD: baseOps[0x0F]
	// Pop PC, then adjust XSP by signed d16.
	baseOps[0x0F] = func(c *CPU, op uint8) {
		d16 := int16(c.fetchPC16())
		c.reg.PC = c.pop(Long) & addrMask
		c.reg.XSP += uint32(int32(d16))
		c.cycles += 11
	}

	// JP #16: baseOps[0x1A]
	baseOps[0x1A] = func(c *CPU, op uint8) {
		c.reg.PC = uint32(c.fetchPC16()) & addrMask
		c.cycles += 5
	}

	// JP #24: baseOps[0x1B]
	baseOps[0x1B] = func(c *CPU, op uint8) {
		c.reg.PC = c.fetchPC24() & addrMask
		c.cycles += 6
	}

	// CALL #16: baseOps[0x1C]
	baseOps[0x1C] = func(c *CPU, op uint8) {
		target := uint32(c.fetchPC16()) & addrMask
		c.push(Long, c.reg.PC)
		c.reg.PC = target
		c.cycles += 9
	}

	// CALL #24: baseOps[0x1D]
	baseOps[0x1D] = func(c *CPU, op uint8) {
		target := c.fetchPC24() & addrMask
		c.push(Long, c.reg.PC)
		c.reg.PC = target
		c.cycles += 10
	}

	// CALR: baseOps[0x1E]
	// Unconditional relative call with 16-bit displacement.
	baseOps[0x1E] = func(c *CPU, op uint8) {
		d16 := int16(c.fetchPC16())
		c.push(Long, c.reg.PC)
		c.reg.PC = (c.reg.PC + uint32(int32(d16))) & addrMask
		c.cycles += 10
	}

	// JR cc: baseOps[0x60-0x6F]
	for i := 0; i < 16; i++ {
		baseOps[0x60+i] = func(c *CPU, op uint8) {
			d8 := int8(c.fetchPC())
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.reg.PC = (c.reg.PC + uint32(int32(d8))) & addrMask
				c.cycles += 5
			} else {
				c.cycles += 2
			}
		}
	}

	// JRL cc: baseOps[0x70-0x7F]
	for i := 0; i < 16; i++ {
		baseOps[0x70+i] = func(c *CPU, op uint8) {
			d16 := int16(c.fetchPC16())
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.reg.PC = (c.reg.PC + uint32(int32(d16))) & addrMask
				c.cycles += 5
			} else {
				c.cycles += 2
			}
		}
	}

	// DJNZ: regOps[0x1C]
	// Decrement register and branch if not zero.
	regOps[0x1C] = func(c *CPU, op uint8) {
		d8 := int8(c.fetchPC())
		val := c.readOpReg() - 1
		val &= c.opSize.Mask()
		c.writeOpReg(val)
		if val != 0 {
			c.reg.PC = (c.reg.PC + uint32(int32(d8))) & addrMask
			c.cycles += 6
		} else {
			c.cycles += 4
		}
	}

	// SCC: regOps[0x70-0x7F]
	// Set register to 1 if condition true, else 0.
	for i := 0; i < 16; i++ {
		regOps[0x70+i] = func(c *CPU, op uint8) {
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.writeOpReg(1)
			} else {
				c.writeOpReg(0)
			}
			c.cycles += 2
		}
	}

	// JP cc,mem: dstMemOps[0xD0-0xDF]
	for i := 0; i < 16; i++ {
		dstMemOps[0xD0+i] = func(c *CPU, op uint8) {
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.reg.PC = c.opAddr & addrMask
				c.cycles += 7
			} else {
				c.cycles += 4
			}
		}
	}

	// CALL cc,mem: dstMemOps[0xE0-0xEF]
	for i := 0; i < 16; i++ {
		dstMemOps[0xE0+i] = func(c *CPU, op uint8) {
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.push(Long, c.reg.PC)
				c.reg.PC = c.opAddr & addrMask
				c.cycles += 12
			} else {
				c.cycles += 4
			}
		}
	}

	// RET cc: dstMemOps[0xF0-0xFF]
	for i := 0; i < 16; i++ {
		dstMemOps[0xF0+i] = func(c *CPU, op uint8) {
			cc := op & 0x0F
			if c.testCondition(cc) {
				c.reg.PC = c.pop(Long) & addrMask
				c.cycles += 12
			} else {
				c.cycles += 4
			}
		}
	}
}
