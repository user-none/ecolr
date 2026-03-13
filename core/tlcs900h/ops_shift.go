package tlcs900h

// Shift and rotate instructions.
// RLC, RRC, RL, RR, SLA, SRA, SLL, SRL, RLD, RRD

// rlcOp rotates val left count times without carry.
// The MSB wraps to LSB. Returns the result and the carry (last MSB shifted out).
func rlcOp(sz Size, val uint32, count uint8) (uint32, bool) {
	mask := sz.Mask()
	bits := sz.Bits()
	v := val & mask
	shift := uint32(count) % bits
	if shift == 0 {
		return v, v&1 != 0
	}
	result := ((v << shift) | (v >> (bits - shift))) & mask
	return result, result&1 != 0
}

// rrcOp rotates val right count times without carry.
// The LSB wraps to MSB. Returns the result and the carry (last LSB shifted out).
func rrcOp(sz Size, val uint32, count uint8) (uint32, bool) {
	mask := sz.Mask()
	msb := sz.MSB()
	bits := sz.Bits()
	v := val & mask
	shift := uint32(count) % bits
	if shift == 0 {
		return v, v&msb != 0
	}
	result := ((v >> shift) | (v << (bits - shift))) & mask
	return result, result&msb != 0
}

// rlOp rotates val left count times through carry.
// Carry goes to LSB, MSB goes to carry.
func rlOp(sz Size, val uint32, count uint8, carryIn bool) (uint32, bool) {
	mask := sz.Mask()
	bits := sz.Bits()
	width := bits + 1
	v := uint64(val & mask)
	if carryIn {
		v |= 1 << bits
	}
	shift := uint64(count) % uint64(width)
	if shift == 0 {
		return uint32(v) & mask, v>>bits&1 != 0
	}
	rotated := ((v << shift) | (v >> (uint64(width) - shift))) & (uint64(1<<width) - 1)
	return uint32(rotated) & mask, rotated>>bits&1 != 0
}

// rrOp rotates val right count times through carry.
// Carry goes to MSB, LSB goes to carry.
func rrOp(sz Size, val uint32, count uint8, carryIn bool) (uint32, bool) {
	mask := sz.Mask()
	bits := sz.Bits()
	width := bits + 1
	v := uint64(val & mask)
	if carryIn {
		v |= 1 << bits
	}
	shift := uint64(count) % uint64(width)
	if shift == 0 {
		return uint32(v) & mask, v>>bits&1 != 0
	}
	rotated := ((v >> shift) | (v << (uint64(width) - shift))) & (uint64(1<<width) - 1)
	return uint32(rotated) & mask, rotated>>bits&1 != 0
}

// slaOp shifts val left count times (arithmetic / logical).
// LSB = 0, carry = last MSB shifted out. Also used for SLL.
func slaOp(sz Size, val uint32, count uint8) (uint32, bool) {
	mask := sz.Mask()
	msb := sz.MSB()
	v := val & mask
	carry := (v<<(count-1))&mask&msb != 0
	return (v << count) & mask, carry
}

// sraOp shifts val right count times (arithmetic).
// MSB is preserved (sign extend), carry = last LSB shifted out.
func sraOp(sz Size, val uint32, count uint8) (uint32, bool) {
	mask := sz.Mask()
	v := val & mask
	sv := int32(v)
	if v&sz.MSB() != 0 {
		sv = int32(v | ^mask)
	}
	carry := (sv>>(count-1))&1 != 0
	return uint32(sv>>count) & mask, carry
}

// srlOp shifts val right count times (logical).
// MSB = 0, carry = last LSB shifted out.
func srlOp(sz Size, val uint32, count uint8) (uint32, bool) {
	v := val & sz.Mask()
	carry := (v>>(count-1))&1 != 0
	return v >> count, carry
}

// shiftOp is a function type for shift/rotate operations that do not
// use the carry flag as input.
type shiftOp func(sz Size, val uint32, count uint8) (uint32, bool)

// shiftCarryOp is a function type for RL/RR that need carry input.
type shiftCarryOp func(sz Size, val uint32, count uint8, carryIn bool) (uint32, bool)

func init() {
	// Table of shift/rotate operations indexed 0-7 matching opcode offsets.
	// Entries 2 and 3 (RL, RR) are nil here because they need carry input.
	simpleOps := [8]shiftOp{
		0: rlcOp, // RLC
		1: rrcOp, // RRC
		2: nil,   // RL  (uses carry)
		3: nil,   // RR  (uses carry)
		4: slaOp, // SLA
		5: sraOp, // SRA
		6: slaOp, // SLL (identical to SLA)
		7: srlOp, // SRL
	}

	carryOps := [8]shiftCarryOp{
		2: rlOp, // RL
		3: rrOp, // RR
	}

	for idx := 0; idx < 8; idx++ {
		idx := idx

		// regOps[0xE8+idx]: immediate count form
		regOps[0xE8+idx] = func(c *CPU, op uint8) {
			sz := c.opSize
			count := c.fetchPC() & 0x0F
			if count == 0 {
				count = 16
			}
			val := c.readOpReg()
			var result uint32
			var carry bool
			if carryOps[idx] != nil {
				result, carry = carryOps[idx](sz, val, count, c.getFlag(flagC))
			} else {
				result, carry = simpleOps[idx](sz, val, count)
			}
			c.writeOpReg(result)
			c.setFlagsShift(sz, result, carry)
			c.cycles += cyclesBWL(sz, 3+uint64(count), 3+uint64(count), 4+uint64(count))
		}

		// regOps[0xF8+idx]: A register count form
		regOps[0xF8+idx] = func(c *CPU, op uint8) {
			sz := c.opSize
			count := c.reg.ReadReg8(r8From3bit[1]) & 0x0F
			if count == 0 {
				count = 16
			}
			val := c.readOpReg()
			var result uint32
			var carry bool
			if carryOps[idx] != nil {
				result, carry = carryOps[idx](sz, val, count, c.getFlag(flagC))
			} else {
				result, carry = simpleOps[idx](sz, val, count)
			}
			c.writeOpReg(result)
			c.setFlagsShift(sz, result, carry)
			c.cycles += cyclesBWL(sz, 3+uint64(count), 3+uint64(count), 4+uint64(count))
		}

		// srcMemOps[0x78+idx]: memory form (count=1)
		srcMemOps[0x78+idx] = func(c *CPU, op uint8) {
			sz := c.opSize
			val := c.readBus(sz, c.opAddr)
			var result uint32
			var carry bool
			if carryOps[idx] != nil {
				result, carry = carryOps[idx](sz, val, 1, c.getFlag(flagC))
			} else {
				result, carry = simpleOps[idx](sz, val, 1)
			}
			c.writeBus(sz, c.opAddr, result)
			c.setFlagsShift(sz, result, carry)
			c.cycles += cyclesBWL(sz, 6, 6, 0)
		}
	}

	// RLD: srcMemOps[0x06]
	srcMemOps[0x06] = func(c *CPU, op uint8) {
		memVal := uint8(c.readBus(Byte, c.opAddr))
		aVal := c.reg.ReadReg8(r8From3bit[1])
		oldALo := aVal & 0x0F
		aVal = (aVal & 0xF0) | (memVal >> 4)
		memVal = (memVal << 4) | oldALo
		c.reg.WriteReg8(r8From3bit[1], aVal)
		c.writeBus(Byte, c.opAddr, uint32(memVal))
		c.setFlagsShift(Byte, uint32(aVal), c.getFlag(flagC))
		c.cycles += 14
	}

	// RRD: srcMemOps[0x07]
	srcMemOps[0x07] = func(c *CPU, op uint8) {
		memVal := uint8(c.readBus(Byte, c.opAddr))
		aVal := c.reg.ReadReg8(r8From3bit[1])
		oldALo := aVal & 0x0F
		aVal = (aVal & 0xF0) | (memVal & 0x0F)
		memVal = (oldALo << 4) | (memVal >> 4)
		c.reg.WriteReg8(r8From3bit[1], aVal)
		c.writeBus(Byte, c.opAddr, uint32(memVal))
		c.setFlagsShift(Byte, uint32(aVal), c.getFlag(flagC))
		c.cycles += 14
	}
}
