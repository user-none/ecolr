package tlcs900h

// Arithmetic instructions.
// ADD, ADC, SUB, SBC, CP, INC, DEC, NEG, MUL, MULS, MULA,
// DIV, DIVS, DAA, EXTZ, EXTS, PAA, MINC1/2/4, MDEC1/2/4

// halfBit returns the half-carry test bit for the given size.
func halfBit(sz Size) uint32 {
	switch sz {
	case Byte:
		return 0x10
	case Word:
		return 0x100
	case Long:
		return 0x10000
	}
	return 0
}

// cyclesBWL selects a cycle count based on operand size.
func cyclesBWL(sz Size, b, w, l uint64) uint64 {
	switch sz {
	case Byte:
		return b
	case Word:
		return w
	case Long:
		return l
	}
	return b
}

// addOp computes a + b, sets arithmetic flags, returns masked result.
func (c *CPU) addOp(sz Size, a, b uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	full := uint64(a&mask) + uint64(b&mask)
	result := uint32(full) & mask
	carry := full>>sz.Bits() != 0
	hb := halfBit(sz)
	halfCarry := ((a ^ b ^ result) & hb) != 0
	overflow := ((a ^ result) & (b ^ result) & msb) != 0
	c.setFlagsArith(sz, result, carry, overflow, halfCarry, false)
	return result
}

// adcOp computes a + b + carry, sets arithmetic flags.
func (c *CPU) adcOp(sz Size, a, b uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	cf := uint64(0)
	if c.getFlag(flagC) {
		cf = 1
	}
	full := uint64(a&mask) + uint64(b&mask) + cf
	result := uint32(full) & mask
	carry := full>>sz.Bits() != 0
	hb := halfBit(sz)
	halfCarry := ((a ^ b ^ result) & hb) != 0
	overflow := ((a ^ result) & (b ^ result) & msb) != 0
	c.setFlagsArith(sz, result, carry, overflow, halfCarry, false)
	return result
}

// subOp computes a - b, sets arithmetic flags.
func (c *CPU) subOp(sz Size, a, b uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	full := uint64(a&mask) - uint64(b&mask)
	result := uint32(full) & mask
	carry := full>>sz.Bits() != 0
	hb := halfBit(sz)
	halfCarry := ((a ^ b ^ result) & hb) != 0
	overflow := ((a ^ b) & (a ^ result) & msb) != 0
	c.setFlagsArith(sz, result, carry, overflow, halfCarry, true)
	return result
}

// sbcOp computes a - b - carry, sets arithmetic flags.
func (c *CPU) sbcOp(sz Size, a, b uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	cf := uint64(0)
	if c.getFlag(flagC) {
		cf = 1
	}
	full := uint64(a&mask) - uint64(b&mask) - cf
	result := uint32(full) & mask
	carry := full>>sz.Bits() != 0
	hb := halfBit(sz)
	halfCarry := ((a ^ b ^ result) & hb) != 0
	overflow := ((a ^ b) & (a ^ result) & msb) != 0
	c.setFlagsArith(sz, result, carry, overflow, halfCarry, true)
	return result
}

// incOp computes a + n with carry preserved.
func (c *CPU) incOp(sz Size, a, n uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	oldC := c.getFlag(flagC)
	full := (a & mask) + (n & mask)
	result := full & mask
	hb := halfBit(sz)
	halfCarry := ((a ^ n ^ result) & hb) != 0
	overflow := ((a ^ result) & (n ^ result) & msb) != 0
	c.setFlagsArith(sz, result, false, overflow, halfCarry, false)
	c.setFlag(flagC, oldC) // preserve carry
	return result
}

// decOp computes a - n with carry preserved.
func (c *CPU) decOp(sz Size, a, n uint32) uint32 {
	mask := sz.Mask()
	msb := sz.MSB()
	oldC := c.getFlag(flagC)
	full := (a & mask) - (n & mask)
	result := full & mask
	hb := halfBit(sz)
	halfCarry := ((a ^ n ^ result) & hb) != 0
	overflow := ((a ^ n) & (a ^ result) & msb) != 0
	c.setFlagsArith(sz, result, false, overflow, halfCarry, true)
	c.setFlag(flagC, oldC) // preserve carry
	return result
}

func init() {
	// --- ADD ---

	// regOps 0x80-0x87: ADD R,r
	for i := 0; i < 8; i++ {
		regOps[0x80+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := c.addOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}
	// regOps 0xC8: ADD r,#
	regOps[0xC8] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := c.addOp(sz, a, b)
		c.writeOpReg(result)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}
	// srcMemOps 0x80-0x87: ADD R,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x80+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := c.addOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	// srcMemOps 0x88-0x8F: ADD (mem),R
	for i := 0; i < 8; i++ {
		srcMemOps[0x88+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := c.addOp(sz, a, b)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}
	// srcMemOps 0x38: ADD<W> (mem),#
	srcMemOps[0x38] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := c.addOp(sz, a, b)
		c.writeBus(sz, c.opAddr, result)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- ADC ---

	for i := 0; i < 8; i++ {
		regOps[0x90+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := c.adcOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}
	regOps[0xC9] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := c.adcOp(sz, a, b)
		c.writeOpReg(result)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0x90+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := c.adcOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0x98+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := c.adcOp(sz, a, b)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}
	srcMemOps[0x39] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := c.adcOp(sz, a, b)
		c.writeBus(sz, c.opAddr, result)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- SUB ---

	for i := 0; i < 8; i++ {
		regOps[0xA0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := c.subOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}
	regOps[0xCA] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := c.subOp(sz, a, b)
		c.writeOpReg(result)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0xA0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := c.subOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0xA8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := c.subOp(sz, a, b)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}
	srcMemOps[0x3A] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := c.subOp(sz, a, b)
		c.writeBus(sz, c.opAddr, result)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- SBC ---

	for i := 0; i < 8; i++ {
		regOps[0xB0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := c.sbcOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}
	regOps[0xCB] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := c.sbcOp(sz, a, b)
		c.writeOpReg(result)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0xB0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := c.sbcOp(sz, a, b)
			c.writeReg(sz, op, result)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	for i := 0; i < 8; i++ {
		srcMemOps[0xB8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := c.sbcOp(sz, a, b)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}
	srcMemOps[0x3B] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := c.sbcOp(sz, a, b)
		c.writeBus(sz, c.opAddr, result)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- CP ---

	// regOps 0xF0-0xF7: CP R,r
	for i := 0; i < 8; i++ {
		regOps[0xF0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			c.subOp(sz, a, b) // discard result
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}
	// regOps 0xD8-0xDF: CP r,#3
	for i := 0; i < 8; i++ {
		regOps[0xD8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readOpReg()
			b := uint32(op & 0x07)
			c.subOp(sz, a, b)
			c.cycles += cyclesBWL(sz, 2, 2, 0)
		}
	}
	// regOps 0xCF: CP r,#
	regOps[0xCF] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		c.subOp(sz, a, b)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}
	// srcMemOps 0xF0-0xF7: CP R,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0xF0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			c.subOp(sz, a, b)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	// srcMemOps 0xF8-0xFF: CP (mem),R
	for i := 0; i < 8; i++ {
		srcMemOps[0xF8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			c.subOp(sz, a, b)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}
	// srcMemOps 0x3F: CP<W> (mem),#
	srcMemOps[0x3F] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		c.subOp(sz, a, b)
		c.cycles += cyclesBWL(sz, 5, 6, 0)
	}

	// --- INC ---

	// regOps 0x60-0x67: INC #3,r
	// Note: For word/long operands, no flags change (Toshiba doc CPU900H-83).
	for i := 0; i < 8; i++ {
		regOps[0x60+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			n := uint32(op & 0x07)
			if n == 0 {
				n = 8
			}
			a := c.readOpReg()
			if sz == Byte {
				result := c.incOp(sz, a, n)
				c.writeOpReg(result)
			} else {
				result := (a + n) & sz.Mask()
				c.writeOpReg(result)
			}
			c.cycles += 2
		}
	}
	// srcMemOps 0x60-0x67: INC<W> #3,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x60+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			n := uint32(op & 0x07)
			if n == 0 {
				n = 8
			}
			a := c.readBus(sz, c.opAddr)
			result := c.incOp(sz, a, n)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += 6
		}
	}

	// --- DEC ---

	// regOps 0x68-0x6F: DEC #3,r
	// Note: For word/long operands, no flags change (Toshiba doc CPU900H-70).
	for i := 0; i < 8; i++ {
		regOps[0x68+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			n := uint32(op & 0x07)
			if n == 0 {
				n = 8
			}
			a := c.readOpReg()
			if sz == Byte {
				result := c.decOp(sz, a, n)
				c.writeOpReg(result)
			} else {
				result := (a - n) & sz.Mask()
				c.writeOpReg(result)
			}
			c.cycles += 2
		}
	}
	// srcMemOps 0x68-0x6F: DEC<W> #3,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x68+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			n := uint32(op & 0x07)
			if n == 0 {
				n = 8
			}
			a := c.readBus(sz, c.opAddr)
			result := c.decOp(sz, a, n)
			c.writeBus(sz, c.opAddr, result)
			c.cycles += 6
		}
	}

	// --- NEG ---

	// regOps 0x07: NEG r (byte/word only)
	regOps[0x07] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		result := c.subOp(sz, 0, a)
		c.writeOpReg(result)
		c.cycles += 2
	}

	// --- MUL ---

	// regOps 0x40-0x47: MUL RR,r
	for i := 0; i < 8; i++ {
		regOps[0x40+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readOpReg()
			b := c.readReg(sz, op)
			switch sz {
			case Byte:
				result := uint32(uint8(a)) * uint32(uint8(b))
				// Byte 3-bit code >> 1 gives parent word register code
				c.writeReg(Word, (op&0x07)>>1, result)
				c.cycles += 11
			case Word:
				result := uint32(uint16(a)) * uint32(uint16(b))
				c.writeReg(Long, op, result)
				c.cycles += 14
			}
		}
	}
	// regOps 0x08: MUL rr,#
	regOps[0x08] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		switch sz {
		case Byte:
			result := uint32(uint8(a)) * uint32(uint8(b))
			if c.opRegEx {
				// Clear bit 0 to get parent word register code
				c.writeRegEx(Word, c.opReg&0xFE, result)
			} else {
				c.writeReg(Word, c.opReg>>1, result)
			}
			c.cycles += 12
		case Word:
			result := uint32(uint16(a)) * uint32(uint16(b))
			if c.opRegEx {
				// regPtrByFullCode32 ignores bits 1-0 for long
				c.writeRegEx(Long, c.opReg, result)
			} else {
				c.writeReg(Long, c.opReg, result)
			}
			c.cycles += 15
		}
	}
	// srcMemOps 0x40-0x47: MUL RR,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x40+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			b := c.readBus(sz, c.opAddr)
			r := op & 0x07
			switch sz {
			case Byte:
				a := uint32(c.reg.ReadReg8(r8From3bit[r]))
				result := uint32(uint8(a)) * uint32(uint8(b))
				c.reg.WriteReg16(r>>1, uint16(result))
				c.cycles += 13
			case Word:
				a := uint32(c.reg.ReadReg16(r))
				result := uint32(uint16(a)) * uint32(uint16(b))
				c.reg.WriteReg32(r, result)
				c.cycles += 16
			}
		}
	}

	// --- MULS ---

	// regOps 0x48-0x4F: MULS RR,r
	for i := 0; i < 8; i++ {
		regOps[0x48+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readOpReg()
			b := c.readReg(sz, op)
			switch sz {
			case Byte:
				result := uint32(uint16(int16(int8(a)) * int16(int8(b))))
				c.writeReg(Word, (op&0x07)>>1, result)
				c.cycles += 9
			case Word:
				result := uint32(int32(int16(a)) * int32(int16(b)))
				c.writeReg(Long, op, result)
				c.cycles += 12
			}
		}
	}
	// regOps 0x09: MULS rr,#
	regOps[0x09] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		switch sz {
		case Byte:
			result := uint32(uint16(int16(int8(a)) * int16(int8(b))))
			if c.opRegEx {
				c.writeRegEx(Word, c.opReg&0xFE, result)
			} else {
				c.writeReg(Word, c.opReg>>1, result)
			}
			c.cycles += 10
		case Word:
			result := uint32(int32(int16(a)) * int32(int16(b)))
			if c.opRegEx {
				c.writeRegEx(Long, c.opReg, result)
			} else {
				c.writeReg(Long, c.opReg, result)
			}
			c.cycles += 13
		}
	}
	// srcMemOps 0x48-0x4F: MULS RR,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x48+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			b := c.readBus(sz, c.opAddr)
			r := op & 0x07
			switch sz {
			case Byte:
				a := uint32(c.reg.ReadReg8(r8From3bit[r]))
				result := uint32(uint16(int16(int8(a)) * int16(int8(b))))
				c.reg.WriteReg16(r>>1, uint16(result))
				c.cycles += 11
			case Word:
				a := uint32(c.reg.ReadReg16(r))
				result := uint32(int32(int16(a)) * int32(int16(b)))
				c.reg.WriteReg32(r, result)
				c.cycles += 14
			}
		}
	}

	// --- MULA ---

	// regOps 0x19: MULA rr (word only, D8+r prefix)
	regOps[0x19] = func(c *CPU, op uint8) {
		// rr = rr + (XDE) * (XHL), signed word multiply, 32-bit accumulate
		// (XDE) and (XHL) are memory reads at addresses in XDE/XHL
		// XHL decremented by 2 after
		// MULA accumulates into a 32-bit register pair; read at Long
		// regardless of the Word-sized opcode prefix.
		var acc uint32
		if c.opRegEx {
			acc = c.readRegEx(Long, c.opReg)
		} else {
			acc = c.readReg(Long, c.opReg)
		}
		deAddr := c.reg.ReadReg32(2)
		hlAddr := c.reg.ReadReg32(3)
		de := int32(int16(uint16(c.readBus(Word, deAddr))))
		hl := int32(int16(uint16(c.readBus(Word, hlAddr))))
		product := uint32(de * hl)
		result := acc + product

		// Write result as 32-bit to the register pair
		if c.opRegEx {
			c.writeRegEx(Long, c.opReg, result)
		} else {
			c.writeReg(Long, c.opReg, result)
		}

		// Decrement XHL by 2
		p := c.reg.regPtr32(3)
		*p -= 2

		// Flags: S:* Z:* H:- V:V N:- C:-
		// H, N, C are preserved
		msb := Long.MSB()
		old := c.flags()
		var f uint8
		if result&msb != 0 {
			f |= flagS
		}
		if result == 0 {
			f |= flagZ
		}
		// Overflow if sign changed unexpectedly
		if (acc^product)&msb == 0 && (acc^result)&msb != 0 {
			f |= flagV
		}
		f |= old & (flagH | flagN | flagC)
		c.setFlags(f)
		c.cycles += 19
	}

	// --- DIV ---

	// regOps 0x50-0x57: DIV RR,r
	for i := 0; i < 8; i++ {
		regOps[0x50+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			r := op & 0x07
			divisor := c.readOpReg()
			switch sz {
			case Byte:
				wr := r >> 1
				dividend := uint32(c.reg.ReadReg16(wr))
				c.divUnsigned8(wr, uint16(dividend), uint8(divisor))
				c.cycles += 15
			case Word:
				dividend := c.reg.ReadReg32(r)
				c.divUnsigned16(r, dividend, uint16(divisor))
				c.cycles += 23
			}
		}
	}
	// regOps 0x0A: DIV rr,#
	regOps[0x0A] = func(c *CPU, op uint8) {
		sz := c.opSize
		divisor := c.fetchImm(sz)
		if c.opRegEx {
			switch sz {
			case Byte:
				wrCode := c.opReg & 0xFE
				dividend := uint16(c.reg.ReadReg16ByFullCode(wrCode))
				if uint8(divisor) == 0 {
					c.setFlag(flagV, true)
				} else {
					q := dividend / uint16(uint8(divisor))
					r := dividend % uint16(uint8(divisor))
					if q > 0xFF {
						c.setFlag(flagV, true)
					} else {
						c.reg.WriteReg16ByFullCode(wrCode, uint16(r)<<8|uint16(uint8(q)))
						c.setFlag(flagV, false)
					}
				}
				c.cycles += 15
			case Word:
				dividend := c.reg.ReadReg32ByFullCode(c.opReg)
				if uint16(divisor) == 0 {
					c.setFlag(flagV, true)
				} else {
					q := dividend / uint32(uint16(divisor))
					r := dividend % uint32(uint16(divisor))
					if q > 0xFFFF {
						c.setFlag(flagV, true)
					} else {
						c.reg.WriteReg32ByFullCode(c.opReg, uint32(r)<<16|uint32(uint16(q)))
						c.setFlag(flagV, false)
					}
				}
				c.cycles += 23
			}
		} else {
			code := c.opReg & 0x07
			switch sz {
			case Byte:
				wr := code >> 1
				dividend := uint32(c.reg.ReadReg16(wr))
				c.divUnsigned8(wr, uint16(dividend), uint8(divisor))
				c.cycles += 15
			case Word:
				dividend := c.reg.ReadReg32(code)
				c.divUnsigned16(code, dividend, uint16(divisor))
				c.cycles += 23
			}
		}
	}
	// srcMemOps 0x50-0x57: DIV RR,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x50+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			r := op & 0x07
			divisor := c.readBus(sz, c.opAddr)
			switch sz {
			case Byte:
				wr := r >> 1
				dividend := uint32(c.reg.ReadReg16(wr))
				c.divUnsigned8(wr, uint16(dividend), uint8(divisor))
				c.cycles += 16
			case Word:
				dividend := c.reg.ReadReg32(r)
				c.divUnsigned16(r, dividend, uint16(divisor))
				c.cycles += 24
			}
		}
	}

	// --- DIVS ---

	// regOps 0x58-0x5F: DIVS RR,r
	for i := 0; i < 8; i++ {
		regOps[0x58+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			r := op & 0x07
			divisor := c.readOpReg()
			switch sz {
			case Byte:
				wr := r >> 1
				dividend := int16(c.reg.ReadReg16(wr))
				c.divSigned8(wr, dividend, int8(divisor))
				c.cycles += 18
			case Word:
				dividend := int32(c.reg.ReadReg32(r))
				c.divSigned16(r, dividend, int16(divisor))
				c.cycles += 26
			}
		}
	}
	// regOps 0x0B: DIVS rr,#
	regOps[0x0B] = func(c *CPU, op uint8) {
		sz := c.opSize
		imm := c.fetchImm(sz)
		if c.opRegEx {
			switch sz {
			case Byte:
				wrCode := c.opReg & 0xFE
				dividend := int16(c.reg.ReadReg16ByFullCode(wrCode))
				d := int8(imm)
				if d == 0 {
					c.setFlag(flagV, true)
				} else {
					q := dividend / int16(d)
					r := dividend % int16(d)
					if q > 127 || q < -128 {
						c.setFlag(flagV, true)
					} else {
						c.reg.WriteReg16ByFullCode(wrCode, uint16(uint8(r))<<8|uint16(uint8(q)))
						c.setFlag(flagV, false)
					}
				}
				c.cycles += 18
			case Word:
				dividend := int32(c.reg.ReadReg32ByFullCode(c.opReg))
				d := int16(imm)
				if d == 0 {
					c.setFlag(flagV, true)
				} else {
					q := dividend / int32(d)
					r := dividend % int32(d)
					if q > 32767 || q < -32768 {
						c.setFlag(flagV, true)
					} else {
						c.reg.WriteReg32ByFullCode(c.opReg, uint32(uint16(r))<<16|uint32(uint16(q)))
						c.setFlag(flagV, false)
					}
				}
				c.cycles += 26
			}
		} else {
			code := c.opReg & 0x07
			switch sz {
			case Byte:
				wr := code >> 1
				dividend := int16(c.reg.ReadReg16(wr))
				c.divSigned8(wr, dividend, int8(imm))
				c.cycles += 18
			case Word:
				dividend := int32(c.reg.ReadReg32(code))
				c.divSigned16(code, dividend, int16(imm))
				c.cycles += 26
			}
		}
	}
	// srcMemOps 0x58-0x5F: DIVS RR,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0x58+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			r := op & 0x07
			divisor := c.readBus(sz, c.opAddr)
			switch sz {
			case Byte:
				wr := r >> 1
				dividend := int16(c.reg.ReadReg16(wr))
				c.divSigned8(wr, dividend, int8(divisor))
				c.cycles += 19
			case Word:
				dividend := int32(c.reg.ReadReg32(r))
				c.divSigned16(r, dividend, int16(divisor))
				c.cycles += 27
			}
		}
	}

	// --- DAA ---

	// regOps 0x10: DAA r (byte only)
	regOps[0x10] = func(c *CPU, op uint8) {
		a := c.readOpReg()
		c.daaOp(uint8(a))
		c.cycles += 4
	}

	// --- EXTZ ---

	// regOps 0x12: EXTZ r (word/long only)
	regOps[0x12] = func(c *CPU, op uint8) {
		sz := c.opSize
		switch sz {
		case Word:
			// Clear high byte: keep only low byte
			val := c.readOpReg() & 0xFF
			c.writeOpReg(val)
		case Long:
			// Clear high word: keep only low word
			val := c.readOpReg() & 0xFFFF
			c.writeOpReg(val)
		}
		c.cycles += 3
	}

	// --- EXTS ---

	// regOps 0x13: EXTS r (word/long only)
	regOps[0x13] = func(c *CPU, op uint8) {
		sz := c.opSize
		switch sz {
		case Word:
			val := c.readOpReg()
			extended := uint32(uint16(int16(int8(val))))
			c.writeOpReg(extended)
		case Long:
			val := c.readOpReg()
			extended := uint32(int32(int16(val)))
			c.writeOpReg(extended)
		}
		c.cycles += 3
	}

	// --- PAA ---

	// regOps 0x14: PAA r (word/long only)
	regOps[0x14] = func(c *CPU, op uint8) {
		val := c.readOpReg()
		if val&1 != 0 {
			val++
		}
		c.writeOpReg(val & c.opSize.Mask())
		c.cycles += 4
	}

	// --- MINC1/2/4 ---

	// regOps 0x38: MINC1 #,r (word only)
	regOps[0x38] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val + 1) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 5
	}
	// regOps 0x39: MINC2 #,r
	regOps[0x39] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val + 2) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 5
	}
	// regOps 0x3A: MINC4 #,r
	regOps[0x3A] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val + 4) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 5
	}

	// --- MDEC1/2/4 ---

	// regOps 0x3C: MDEC1 #,r
	regOps[0x3C] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val - 1) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 4
	}
	// regOps 0x3D: MDEC2 #,r
	regOps[0x3D] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val - 2) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 4
	}
	// regOps 0x3E: MDEC4 #,r
	regOps[0x3E] = func(c *CPU, op uint8) {
		mask := uint32(c.fetchPC16())
		val := c.readOpReg()
		result := (val & ^mask) | ((val - 4) & mask)
		c.writeOpReg(result & c.opSize.Mask())
		c.cycles += 4
	}
}

// divUnsigned8 performs unsigned 16/8 division. Quotient in low byte,
// remainder in high byte of the 16-bit register pair.
func (c *CPU) divUnsigned8(regCode uint8, dividend uint16, divisor uint8) {
	if divisor == 0 {
		c.setFlag(flagV, true)
		return
	}
	quotient := dividend / uint16(divisor)
	remainder := dividend % uint16(divisor)
	if quotient > 0xFF {
		c.setFlag(flagV, true)
		return
	}
	result := uint16(remainder)<<8 | uint16(uint8(quotient))
	c.reg.WriteReg16(regCode, result)
	c.setFlag(flagV, false)
}

// divUnsigned16 performs unsigned 32/16 division.
func (c *CPU) divUnsigned16(regCode uint8, dividend uint32, divisor uint16) {
	if divisor == 0 {
		c.setFlag(flagV, true)
		return
	}
	quotient := dividend / uint32(divisor)
	remainder := dividend % uint32(divisor)
	if quotient > 0xFFFF {
		c.setFlag(flagV, true)
		return
	}
	result := uint32(remainder)<<16 | uint32(uint16(quotient))
	c.reg.WriteReg32(regCode, result)
	c.setFlag(flagV, false)
}

// divSigned8 performs signed 16/8 division.
func (c *CPU) divSigned8(regCode uint8, dividend int16, divisor int8) {
	if divisor == 0 {
		c.setFlag(flagV, true)
		return
	}
	quotient := dividend / int16(divisor)
	remainder := dividend % int16(divisor)
	if quotient > 127 || quotient < -128 {
		c.setFlag(flagV, true)
		return
	}
	result := uint16(uint8(remainder))<<8 | uint16(uint8(quotient))
	c.reg.WriteReg16(regCode, result)
	c.setFlag(flagV, false)
}

// divSigned16 performs signed 32/16 division.
func (c *CPU) divSigned16(regCode uint8, dividend int32, divisor int16) {
	if divisor == 0 {
		c.setFlag(flagV, true)
		return
	}
	quotient := dividend / int32(divisor)
	remainder := dividend % int32(divisor)
	if quotient > 32767 || quotient < -32768 {
		c.setFlag(flagV, true)
		return
	}
	result := uint32(uint16(remainder))<<16 | uint32(uint16(quotient))
	c.reg.WriteReg32(regCode, result)
	c.setFlag(flagV, false)
}

// daaOp performs BCD decimal adjust on accumulator.
func (c *CPU) daaOp(val uint8) {
	f := c.flags()
	nFlag := f&flagN != 0
	cFlag := f&flagC != 0
	hFlag := f&flagH != 0

	correction := uint8(0)
	newC := cFlag

	oldVal := val
	if nFlag {
		// After subtraction
		if hFlag {
			correction |= 0x06
		}
		if cFlag {
			correction |= 0x60
		}
		val -= correction
	} else {
		// After addition
		if hFlag || (val&0x0F) > 9 {
			correction |= 0x06
		}
		if cFlag || val > 0x99 {
			correction |= 0x60
			newC = true
		}
		val += correction
	}

	var newF uint8
	if val&0x80 != 0 {
		newF |= flagS
	}
	if val == 0 {
		newF |= flagZ
	}
	// H: actual half-carry/borrow of the adjustment operation
	if (oldVal^correction^val)&0x10 != 0 {
		newF |= flagH
	}
	// V: parity
	newF |= parityTable[val]
	// N: preserved
	if nFlag {
		newF |= flagN
	}
	if newC {
		newF |= flagC
	}
	c.setFlags(newF)
	c.writeOpReg(uint32(val))
}
