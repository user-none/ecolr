package tlcs900h

// Logic instructions: AND, OR, XOR

func init() {
	// --- AND ---

	// regOps 0xC0-0xC7: AND R,r
	for i := 0; i < 8; i++ {
		regOps[0xC0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := (a & b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, true)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}

	// regOps 0xCC: AND r,#
	regOps[0xCC] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := (a & b) & sz.Mask()
		c.writeOpReg(result)
		c.setFlagsLogic(sz, result, true)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}

	// srcMemOps 0xC0-0xC7: AND R,(mem) [src]
	for i := 0; i < 8; i++ {
		srcMemOps[0xC0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := (a & b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, true)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}

	// srcMemOps 0xC8-0xCF: AND (mem),R [src]
	for i := 0; i < 8; i++ {
		srcMemOps[0xC8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := (a & b) & sz.Mask()
			c.writeBus(sz, c.opAddr, result)
			c.setFlagsLogic(sz, result, true)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}

	// srcMemOps 0x3C: AND (mem),#
	srcMemOps[0x3C] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := (a & b) & sz.Mask()
		c.writeBus(sz, c.opAddr, result)
		c.setFlagsLogic(sz, result, true)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- XOR ---

	// regOps 0xD0-0xD7: XOR R,r
	for i := 0; i < 8; i++ {
		regOps[0xD0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := (a ^ b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}

	// regOps 0xCD: XOR r,#
	regOps[0xCD] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := (a ^ b) & sz.Mask()
		c.writeOpReg(result)
		c.setFlagsLogic(sz, result, false)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}

	// srcMemOps 0xD0-0xD7: XOR R,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0xD0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := (a ^ b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}

	// srcMemOps 0xD8-0xDF: XOR (mem),R
	for i := 0; i < 8; i++ {
		srcMemOps[0xD8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := (a ^ b) & sz.Mask()
			c.writeBus(sz, c.opAddr, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}

	// srcMemOps 0x3D: XOR (mem),#
	srcMemOps[0x3D] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := (a ^ b) & sz.Mask()
		c.writeBus(sz, c.opAddr, result)
		c.setFlagsLogic(sz, result, false)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}

	// --- OR ---

	// regOps 0xE0-0xE7: OR R,r
	for i := 0; i < 8; i++ {
		regOps[0xE0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			result := (a | b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 2, 2, 2)
		}
	}

	// regOps 0xCE: OR r,#
	regOps[0xCE] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readOpReg()
		b := c.fetchImm(sz)
		result := (a | b) & sz.Mask()
		c.writeOpReg(result)
		c.setFlagsLogic(sz, result, false)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}

	// srcMemOps 0xE0-0xE7: OR R,(mem)
	for i := 0; i < 8; i++ {
		srcMemOps[0xE0+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readBus(sz, c.opAddr)
			result := (a | b) & sz.Mask()
			c.writeReg(sz, op, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}

	// srcMemOps 0xE8-0xEF: OR (mem),R
	for i := 0; i < 8; i++ {
		srcMemOps[0xE8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readBus(sz, c.opAddr)
			b := c.readReg(sz, op)
			result := (a | b) & sz.Mask()
			c.writeBus(sz, c.opAddr, result)
			c.setFlagsLogic(sz, result, false)
			c.cycles += cyclesBWL(sz, 6, 6, 10)
		}
	}

	// srcMemOps 0x3E: OR (mem),#
	srcMemOps[0x3E] = func(c *CPU, op uint8) {
		sz := c.opSize
		a := c.readBus(sz, c.opAddr)
		b := c.fetchImm(sz)
		result := (a | b) & sz.Mask()
		c.writeBus(sz, c.opAddr, result)
		c.setFlagsLogic(sz, result, false)
		c.cycles += cyclesBWL(sz, 7, 8, 0)
	}
}
