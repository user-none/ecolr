package tlcs900h

// Compare block instructions: CPI, CPIR, CPD, CPDR.

// cpiCpd performs a single compare-and-scan step.
// dir is +1 for CPI (increment) or -1 for CPD (decrement).
func (c *CPU) cpiCpd(dir int) {
	sz := c.opSize
	step := uint32(sz)

	// Read comparand: A for byte, WA for word.
	var comparand uint32
	if sz == Byte {
		comparand = uint32(c.reg.ReadReg8(r8From3bit[1])) // A
	} else {
		comparand = uint32(c.reg.ReadReg16(0)) // WA
	}

	// Read memory value at the pointer register address.
	memVal := c.readBus(sz, c.opAddr)

	// Save carry, perform subtraction for flags, restore carry.
	oldC := c.getFlag(flagC)
	c.subOp(sz, comparand, memVal)
	c.setFlag(flagC, oldC)

	// Update pointer register.
	p := c.reg.regPtr32(c.opMemReg)
	if dir > 0 {
		*p += step
	} else {
		*p -= step
	}

	// Decrement BC by 1.
	bc := c.reg.ReadReg16(1) - 1
	c.reg.WriteReg16(1, bc)

	// V = (BC != 0).
	c.setFlag(flagV, bc != 0)

	c.cycles += 6
}

// cpirCpdr performs repeated compare-and-scan until match or BC exhausted.
// dir is +1 for CPIR (increment) or -1 for CPDR (decrement).
func (c *CPU) cpirCpdr(dir int) {
	sz := c.opSize
	step := uint32(sz)

	// BC=0 means 65536 iterations (do-while: decrement wraps 0 to 0xFFFF).
	count := uint64(c.reg.ReadReg16(1))
	if count == 0 {
		count = 65536
	}

	// Read comparand: A for byte, WA for word.
	var comparand uint32
	if sz == Byte {
		comparand = uint32(c.reg.ReadReg8(r8From3bit[1]))
	} else {
		comparand = uint32(c.reg.ReadReg16(0))
	}

	oldC := c.getFlag(flagC)
	ptr := c.reg.ReadReg32(c.opMemReg)
	n := uint64(0)

	for count > 0 {
		memVal := c.readBus(sz, ptr)
		c.subOp(sz, comparand, memVal)
		if dir > 0 {
			ptr += step
		} else {
			ptr -= step
		}
		count--
		n++
		if c.getFlag(flagZ) {
			break // match found
		}
	}

	// Write back pointer and BC.
	c.reg.WriteReg32(c.opMemReg, ptr)
	c.reg.WriteReg16(1, uint16(count))

	// Restore carry, set V = (BC != 0).
	c.setFlag(flagC, oldC)
	c.setFlag(flagV, count != 0)

	c.cycles += 6*n + 1
}

func init() {
	// 0x14: CPI
	srcMemOps[0x14] = func(c *CPU, op uint8) { c.cpiCpd(1) }
	// 0x15: CPIR
	srcMemOps[0x15] = func(c *CPU, op uint8) { c.cpirCpdr(1) }
	// 0x16: CPD
	srcMemOps[0x16] = func(c *CPU, op uint8) { c.cpiCpd(-1) }
	// 0x17: CPDR
	srcMemOps[0x17] = func(c *CPU, op uint8) { c.cpirCpdr(-1) }
}
