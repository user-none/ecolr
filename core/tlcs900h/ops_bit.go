package tlcs900h

import "math/bits"

// Bit manipulation and carry flag instructions.
// BIT, SET, RES, CHG, TSET, ANDCF, ORCF, XORCF, LDCF, STCF,
// BS1F, BS1B, CPL, RCF, SCF, CCF, ZCF

// getBit returns whether the specified bit is set in val.
// Returns false if the bit number exceeds the operand size.
func getBit(val uint32, bit uint8, sz Size) bool {
	if bit >= uint8(sz.Bits()) {
		return false
	}
	return val&(1<<bit) != 0
}

// setBitVal returns val with the specified bit set to 1.
func setBitVal(val uint32, bit uint8, sz Size) uint32 {
	if bit >= uint8(sz.Bits()) {
		return val
	}
	return val | (1 << bit)
}

// clearBitVal returns val with the specified bit cleared to 0.
func clearBitVal(val uint32, bit uint8, sz Size) uint32 {
	if bit >= uint8(sz.Bits()) {
		return val
	}
	return val &^ (1 << bit)
}

// toggleBitVal returns val with the specified bit toggled.
func toggleBitVal(val uint32, bit uint8, sz Size) uint32 {
	if bit >= uint8(sz.Bits()) {
		return val
	}
	return val ^ (1 << bit)
}

// setFlagsBIT sets flags for BIT and TSET instructions.
// S is set to the tested bit if it is the sign bit, Z is set if the bit is 0.
// H=1, N=0.
func (c *CPU) setFlagsBIT(val uint32, bit uint8, sz Size) {
	f := c.flags()
	f &^= flagS | flagZ | flagH | flagV | flagN
	f |= flagH // H always set

	bitSet := getBit(val, bit, sz)
	if !bitSet {
		f |= flagZ
	}
	// S mirrors the tested bit when it is the MSB position
	msb := uint8(sz.Bits() - 1)
	if bit == msb && bitSet {
		f |= flagS
	}
	// V is undefined per docs, set same as Z inverted (matches typical hw)
	if bitSet {
		f |= flagV
	}
	c.setFlags(f)
}

func init() {
	// === Standalone baseOps ===

	// 0x10: RCF - Reset Carry Flag
	baseOps[0x10] = func(c *CPU, op uint8) {
		f := c.flags()
		f &^= flagH | flagN | flagC
		c.setFlags(f)
		c.cycles += 2
	}

	// 0x11: SCF - Set Carry Flag
	baseOps[0x11] = func(c *CPU, op uint8) {
		f := c.flags()
		f &^= flagH | flagN
		f |= flagC
		c.setFlags(f)
		c.cycles += 2
	}

	// 0x12: CCF - Complement Carry Flag
	baseOps[0x12] = func(c *CPU, op uint8) {
		f := c.flags()
		// H gets old C value
		oldC := f & flagC
		f &^= flagH | flagN | flagC
		if oldC != 0 {
			f |= flagH
		} else {
			f |= flagC
		}
		c.setFlags(f)
		c.cycles += 2
	}

	// 0x13: ZCF - Zero Carry Flag (C = !Z)
	baseOps[0x13] = func(c *CPU, op uint8) {
		f := c.flags()
		oldZ := f & flagZ
		f &^= flagH | flagN | flagC
		if oldZ == 0 {
			f |= flagC
		}
		c.setFlags(f)
		c.cycles += 2
	}

	// === regOps ===

	// 0x06: CPL r (byte/word only)
	regOps[0x06] = func(c *CPU, op uint8) {
		sz := c.opSize
		val := c.readOpReg()
		result := ^val & sz.Mask()
		c.writeOpReg(result)
		f := c.flags()
		f |= flagH | flagN
		c.setFlags(f)
		c.cycles += cyclesBWL(sz, 2, 2, 0)
	}

	// 0x0E: BS1F A,r (word only) - Bit Search 1 Forward (LSB to MSB)
	regOps[0x0E] = func(c *CPU, op uint8) {
		val16 := uint16(c.readOpReg())
		if val16 == 0 {
			c.setFlag(flagV, true)
		} else {
			c.reg.WriteReg8(r8From3bit[1], uint8(bits.TrailingZeros16(val16))) // A
			c.setFlag(flagV, false)
		}
		c.cycles += 3
	}

	// 0x0F: BS1B A,r (word only) - Bit Search 1 Backward (MSB to LSB)
	regOps[0x0F] = func(c *CPU, op uint8) {
		val16 := uint16(c.readOpReg())
		if val16 == 0 {
			c.setFlag(flagV, true)
		} else {
			c.reg.WriteReg8(r8From3bit[1], uint8(bits.Len16(val16)-1)) // A
			c.setFlag(flagV, false)
		}
		c.cycles += 3
	}

	// 0x20: ANDCF #4,r
	regOps[0x20] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, c.getFlag(flagC) && getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x21: ORCF #4,r
	regOps[0x21] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, c.getFlag(flagC) || getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x22: XORCF #4,r
	regOps[0x22] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		bitVal := getBit(val, bit, sz)
		c.setFlag(flagC, c.getFlag(flagC) != bitVal)
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x23: LDCF #4,r - Load Carry from bit
	regOps[0x23] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x24: STCF #4,r - Store Carry to bit
	regOps[0x24] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		if c.getFlag(flagC) {
			val = setBitVal(val, bit, sz)
		} else {
			val = clearBitVal(val, bit, sz)
		}
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x28: ANDCF A,r
	regOps[0x28] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, c.getFlag(flagC) && getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x29: ORCF A,r
	regOps[0x29] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, c.getFlag(flagC) || getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x2A: XORCF A,r
	regOps[0x2A] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readOpReg()
		bitVal := getBit(val, bit, sz)
		c.setFlag(flagC, c.getFlag(flagC) != bitVal)
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x2B: LDCF A,r
	regOps[0x2B] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readOpReg()
		c.setFlag(flagC, getBit(val, bit, sz))
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x2C: STCF A,r
	regOps[0x2C] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readOpReg()
		if c.getFlag(flagC) {
			val = setBitVal(val, bit, sz)
		} else {
			val = clearBitVal(val, bit, sz)
		}
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x30: RES #4,r
	regOps[0x30] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		val = clearBitVal(val, bit, sz)
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x31: SET #4,r
	regOps[0x31] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		val = setBitVal(val, bit, sz)
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x32: CHG #4,r
	regOps[0x32] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		val = toggleBitVal(val, bit, sz)
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x33: BIT #4,r
	regOps[0x33] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		c.setFlagsBIT(val, bit, sz)
		c.cycles += cyclesBWL(sz, 3, 3, 0)
	}

	// 0x34: TSET #4,r - Test and Set
	regOps[0x34] = func(c *CPU, op uint8) {
		sz := c.opSize
		bit := c.fetchPC() & 0x0F
		val := c.readOpReg()
		c.setFlagsBIT(val, bit, sz)
		val = setBitVal(val, bit, sz)
		c.writeOpReg(val & sz.Mask())
		c.cycles += cyclesBWL(sz, 4, 4, 0)
	}

	// === dstMemOps (destination prefix, byte only) ===

	// 0x28: ANDCF A,(mem)
	dstMemOps[0x28] = func(c *CPU, op uint8) {
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readBus(Byte, c.opAddr)
		c.setFlag(flagC, c.getFlag(flagC) && getBit(val, bit, Byte))
		c.cycles += 6
	}

	// 0x29: ORCF A,(mem)
	dstMemOps[0x29] = func(c *CPU, op uint8) {
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readBus(Byte, c.opAddr)
		c.setFlag(flagC, c.getFlag(flagC) || getBit(val, bit, Byte))
		c.cycles += 6
	}

	// 0x2A: XORCF A,(mem)
	dstMemOps[0x2A] = func(c *CPU, op uint8) {
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readBus(Byte, c.opAddr)
		c.setFlag(flagC, c.getFlag(flagC) != getBit(val, bit, Byte))
		c.cycles += 6
	}

	// 0x2B: LDCF A,(mem)
	dstMemOps[0x2B] = func(c *CPU, op uint8) {
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readBus(Byte, c.opAddr)
		c.setFlag(flagC, getBit(val, bit, Byte))
		c.cycles += 6
	}

	// 0x2C: STCF A,(mem)
	dstMemOps[0x2C] = func(c *CPU, op uint8) {
		bit := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
		val := c.readBus(Byte, c.opAddr)
		if c.getFlag(flagC) {
			val = setBitVal(val, bit, Byte)
		} else {
			val = clearBitVal(val, bit, Byte)
		}
		c.writeBus(Byte, c.opAddr, val)
		c.cycles += 7
	}

	// 0x80-0x87: ANDCF #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x80+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlag(flagC, c.getFlag(flagC) && getBit(val, bit, Byte))
			c.cycles += 6
		}
	}

	// 0x88-0x8F: ORCF #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x88+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlag(flagC, c.getFlag(flagC) || getBit(val, bit, Byte))
			c.cycles += 6
		}
	}

	// 0x90-0x97: XORCF #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x90+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlag(flagC, c.getFlag(flagC) != getBit(val, bit, Byte))
			c.cycles += 6
		}
	}

	// 0x98-0x9F: LDCF #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x98+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlag(flagC, getBit(val, bit, Byte))
			c.cycles += 6
		}
	}

	// 0xA0-0xA7: STCF #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xA0+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			if c.getFlag(flagC) {
				val = setBitVal(val, bit, Byte)
			} else {
				val = clearBitVal(val, bit, Byte)
			}
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 7
		}
	}

	// 0xA8-0xAF: TSET #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xA8+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlagsBIT(val, bit, Byte)
			val = setBitVal(val, bit, Byte)
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 7
		}
	}

	// 0xB0-0xB7: RES #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xB0+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			val = clearBitVal(val, bit, Byte)
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 7
		}
	}

	// 0xB8-0xBF: SET #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xB8+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			val = setBitVal(val, bit, Byte)
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 7
		}
	}

	// 0xC0-0xC7: CHG #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xC0+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			val = toggleBitVal(val, bit, Byte)
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 7
		}
	}

	// 0xC8-0xCF: BIT #3,(mem) [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0xC8+i] = func(c *CPU, op uint8) {
			bit := op & 0x07
			val := c.readBus(Byte, c.opAddr)
			c.setFlagsBIT(val, bit, Byte)
			c.cycles += 6
		}
	}
}
