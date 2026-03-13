package tlcs900h

// checkInterrupt checks for and processes a pending interrupt.
// Returns true if an interrupt was serviced.
func (c *CPU) checkInterrupt() bool {
	if !c.hasPending {
		return false
	}

	level := c.pendingLevel
	mask := c.iff()

	// Level 7 (NMI) is non-maskable. Others must be >= the current mask.
	if level < mask && level < 7 {
		return false
	}

	c.processInterrupt()
	return true
}

// processInterrupt services the pending interrupt.
// Pushes PC and SR, updates IFF to the serviced level, reads the
// vector address from the vector table, and jumps to the handler.
func (c *CPU) processInterrupt() {
	level := c.pendingLevel
	vec := c.pendingVec

	c.hasPending = false
	c.pendingLevel = 0
	c.pendingVec = 0

	// Save current state: SR at XSP, PC at XSP+2 (matches RETI layout)
	c.reg.XSP -= 6
	c.writeBus(Word, c.reg.XSP, uint32(c.reg.SR))
	c.writeBus(Long, c.reg.XSP+2, c.reg.PC)

	// Set IFF to level+1 to mask the current and lower priority interrupts.
	// Capped at 7 since IFF is a 3-bit field.
	newIFF := level + 1
	if newIFF > 7 {
		newIFF = 7
	}
	sr := c.reg.SR
	sr &^= srIFFMask
	sr |= uint16(newIFF) << srIFFShift
	c.setSR(sr)

	// Read handler address from vector table
	// Vector table at 0xFFFF00, each entry is 4 bytes
	vecAddr := uint32(0xFFFF00) + uint32(vec)*4
	c.reg.PC = c.readBus(Long, vecAddr) & addrMask

	c.intNest++

	c.halted = false
	c.stopped = false

	// Interrupt acknowledge costs cycles
	c.cycles += 18
}
