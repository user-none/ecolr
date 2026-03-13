package tlcs900h

const addrMask = 0x00FFFFFF // 24-bit address space

// CPU implements the Toshiba TLCS-900/H processor core.
type CPU struct {
	reg     Registers
	bus     Bus
	cycles  uint64
	halted  bool
	stopped bool
	deficit int

	// Interrupt state
	pendingLevel uint8
	pendingVec   uint8
	hasPending   bool
	intNest      uint16 // INTNEST: interrupt nesting counter (cr 0x3C)

	// Decoded operand state set by prefix handlers, consumed by regOps/srcMemOps/dstMemOps.
	// Transient within a single instruction - not serialized.
	opSize   Size   // operand size from prefix
	opAddr   uint32 // resolved memory address (srcMemOps/dstMemOps path)
	opReg    uint8  // register code (regOps path)
	opRegEx  bool   // true if opReg is an extended (4-bit) code
	opMemReg uint8  // register code from source memory prefix (for LDI/LDD)

	// Custom opcode handlers registered by external code (e.g. HLE BIOS).
	customOps [256]func(c *CPU)
}

// New creates a CPU connected to the given bus and initializes state.
// The bus may not have valid data yet (e.g. BIOS not loaded), so the
// reset vector is not read. Call Reset() or LoadResetVector() once the
// bus is ready.
func New(bus Bus) *CPU {
	c := &CPU{bus: bus}
	c.initState()
	return c
}

// RegisterOp registers a custom handler for a primary opcode byte.
// When registered, the handler takes precedence over the static baseOps table.
func (c *CPU) RegisterOp(opcode uint8, handler func(c *CPU)) {
	c.customOps[opcode] = handler
}

// initState zeroes registers and sets power-on defaults without
// reading from the bus.
func (c *CPU) initState() {
	c.reg = Registers{}
	c.reg.SR = srMAX | srSYSM | srIFFMask // max mode, system mode, IFF=7 (all masked)
	c.reg.XSP = 0x6C00
	c.halted = false
	c.stopped = false
	c.deficit = 0
	c.pendingLevel = 0
	c.pendingVec = 0
	c.hasPending = false
	c.intNest = 0

	c.bus.Reset()
}

// Reset performs a full hardware reset: initializes CPU state and
// loads the reset vector from the bus.
func (c *CPU) Reset() {
	c.initState()
	c.LoadResetVector()
}

// LoadResetVector reads the reset vector from 0xFFFF00 and sets PC.
// Call after the bus has valid data (e.g. BIOS loaded).
func (c *CPU) LoadResetVector() {
	c.reg.PC = c.readBus(Long, 0xFFFF00) & addrMask
}

// SetPC sets the program counter.
func (c *CPU) SetPC(pc uint32) {
	c.reg.PC = pc
}

// Step executes one instruction and returns the number of cycles consumed.
func (c *CPU) Step() int {
	before := c.cycles

	if c.checkInterrupt() {
		return int(c.cycles - before)
	}

	if c.halted {
		c.cycles += 4
		return int(c.cycles - before)
	}

	c.execute()

	return int(c.cycles - before)
}

// StepCycles executes instructions within a cycle budget.
// Returns the number of cycles actually consumed.
func (c *CPU) StepCycles(budget int) int {
	if c.deficit > 0 {
		if budget >= c.deficit {
			n := c.deficit
			c.deficit = 0
			return n
		}
		c.deficit -= budget
		return budget
	}

	cost := c.Step()
	if cost <= budget {
		return cost
	}

	c.deficit = cost - budget
	return budget
}

// Deficit returns the remaining cycle debt from a previous StepCycles call.
func (c *CPU) Deficit() int {
	return c.deficit
}

// Cycles returns the total number of cycles executed.
func (c *CPU) Cycles() uint64 {
	return c.cycles
}

// AddCycles adds external hold cycles to the counter.
func (c *CPU) AddCycles(n uint64) {
	c.cycles += n
}

// Halted returns true if the CPU is halted.
func (c *CPU) Halted() bool {
	return c.halted
}

// Registers returns a snapshot of the current register state.
func (c *CPU) Registers() Registers {
	return c.reg
}

// ReadBank3W returns register W from Bank[3] (bits 15-8 of XWA).
// Used by the HLE BIOS to read the SWI 1 call number.
func (c *CPU) ReadBank3W() uint16 {
	return uint16((c.reg.Bank[3].XWA >> 8) & 0xFF)
}

// ReadBank3RA returns the low byte of Bank[3].XWA (register A).
func (c *CPU) ReadBank3RA() uint8 {
	return uint8(c.reg.Bank[3].XWA)
}

// ReadBank3RB returns bits 8-15 of Bank[3].XBC (register B).
func (c *CPU) ReadBank3RB() uint8 {
	return uint8(c.reg.Bank[3].XBC >> 8)
}

// ReadBank3RC returns the low byte of Bank[3].XBC (register C).
func (c *CPU) ReadBank3RC() uint8 {
	return uint8(c.reg.Bank[3].XBC)
}

// ReadBank3RD returns bits 8-15 of Bank[3].XDE (register D).
func (c *CPU) ReadBank3RD() uint8 {
	return uint8(c.reg.Bank[3].XDE >> 8)
}

// ReadBank3BC returns the low 16 bits of Bank[3].XBC.
func (c *CPU) ReadBank3BC() uint16 {
	return uint16(c.reg.Bank[3].XBC)
}

// ReadBank3XDE returns Bank[3].XDE.
func (c *CPU) ReadBank3XDE() uint32 {
	return c.reg.Bank[3].XDE
}

// ReadBank3XHL returns Bank[3].XHL.
func (c *CPU) ReadBank3XHL() uint32 {
	return c.reg.Bank[3].XHL
}

// WriteBank3RA writes the low byte of Bank[3].XWA, preserving upper bits.
func (c *CPU) WriteBank3RA(v uint8) {
	c.reg.Bank[3].XWA = (c.reg.Bank[3].XWA & 0xFFFFFF00) | uint32(v)
}

// WriteBank3RWA writes the low 16 bits of Bank[3].XWA, preserving upper bits.
func (c *CPU) WriteBank3RWA(v uint16) {
	c.reg.Bank[3].XWA = (c.reg.Bank[3].XWA & 0xFFFF0000) | uint32(v)
}

// Halt sets the CPU halted flag, stopping instruction execution until
// an interrupt or reset occurs.
func (c *CPU) Halt() {
	c.halted = true
}

// RETI pops SR+PC from the stack and decrements intNest.
// This mirrors the baseOps[0x07] instruction but is callable from HLE code.
func (c *CPU) RETI() {
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

// SetState directly sets the CPU register state. Intended for testing
// and state restore.
func (c *CPU) SetState(regs Registers) {
	c.reg = regs
}

// RequestInterrupt requests an interrupt at the given priority level
// with the specified vector number.
func (c *CPU) RequestInterrupt(level uint8, vector uint8) {
	if !c.hasPending || level > c.pendingLevel {
		c.pendingLevel = level
		c.pendingVec = vector
		c.hasPending = true
	}
}

// readBus reads from the bus with 24-bit address masking.
func (c *CPU) readBus(sz Size, addr uint32) uint32 {
	return c.bus.Read(sz, addr&addrMask)
}

// writeBus writes to the bus with 24-bit address masking.
func (c *CPU) writeBus(sz Size, addr uint32, val uint32) {
	c.bus.Write(sz, addr&addrMask, val)
}

// fetchPC reads a byte at PC and advances PC by 1.
func (c *CPU) fetchPC() uint8 {
	val := c.readBus(Byte, c.reg.PC)
	c.reg.PC = (c.reg.PC + 1) & addrMask
	return uint8(val)
}

// fetchPC16 reads a 16-bit little-endian value at PC and advances PC by 2.
func (c *CPU) fetchPC16() uint16 {
	lo := uint16(c.fetchPC())
	hi := uint16(c.fetchPC())
	return hi<<8 | lo
}

// fetchPC24 reads a 24-bit little-endian value at PC and advances PC by 3.
func (c *CPU) fetchPC24() uint32 {
	lo := uint32(c.fetchPC())
	mid := uint32(c.fetchPC())
	hi := uint32(c.fetchPC())
	return hi<<16 | mid<<8 | lo
}

// fetchPC32 reads a 32-bit little-endian value at PC and advances PC by 4.
func (c *CPU) fetchPC32() uint32 {
	lo := uint32(c.fetchPC16())
	hi := uint32(c.fetchPC16())
	return hi<<16 | lo
}

// push decrements SP and writes a value to the stack.
func (c *CPU) push(sz Size, val uint32) {
	c.reg.XSP -= uint32(sz)
	c.writeBus(sz, c.reg.XSP, val)
}

// pop reads a value from the stack and increments SP.
func (c *CPU) pop(sz Size) uint32 {
	val := c.readBus(sz, c.reg.XSP)
	c.reg.XSP += uint32(sz)
	return val
}

// readReg reads a register by 3-bit code at the given size.
func (c *CPU) readReg(sz Size, code uint8) uint32 {
	switch sz {
	case Byte:
		return uint32(c.reg.ReadReg8(r8From3bit[code&0x07]))
	case Word:
		return uint32(c.reg.ReadReg16(code & 0x07))
	case Long:
		return c.reg.ReadReg32(code & 0x07)
	}
	return 0
}

// writeReg writes a register by 3-bit code at the given size.
func (c *CPU) writeReg(sz Size, code uint8, val uint32) {
	switch sz {
	case Byte:
		c.reg.WriteReg8(r8From3bit[code&0x07], uint8(val))
	case Word:
		c.reg.WriteReg16(code&0x07, uint16(val))
	case Long:
		c.reg.WriteReg32(code&0x07, val)
	}
}

// readRegEx reads a register using extended encoding.
// The code is a full 8-bit register code (bank-aware).
// For byte: bits 1-0 select which byte of the 32-bit register.
// For word/long: bits 1-0 are ignored (same as regPtrByFullCode32).
func (c *CPU) readRegEx(sz Size, code uint8) uint32 {
	switch sz {
	case Byte:
		return uint32(c.reg.ReadReg8ByFullCode(code))
	case Word:
		return uint32(c.reg.ReadReg16ByFullCode(code))
	case Long:
		return c.reg.ReadReg32ByFullCode(code)
	}
	return 0
}

// writeRegEx writes a register using extended encoding.
// See readRegEx for encoding details.
func (c *CPU) writeRegEx(sz Size, code uint8, val uint32) {
	switch sz {
	case Byte:
		c.reg.WriteReg8ByFullCode(code, uint8(val))
	case Word:
		c.reg.WriteReg16ByFullCode(code, uint16(val))
	case Long:
		c.reg.WriteReg32ByFullCode(code, val)
	}
}

// fetchImm fetches an immediate value of the given size from PC.
func (c *CPU) fetchImm(sz Size) uint32 {
	switch sz {
	case Byte:
		return uint32(c.fetchPC())
	case Word:
		return uint32(c.fetchPC16())
	case Long:
		return c.fetchPC32()
	}
	return 0
}

// readOpReg reads the prefix register at the current operand size.
func (c *CPU) readOpReg() uint32 {
	if c.opRegEx {
		return c.readRegEx(c.opSize, c.opReg)
	}
	return c.readReg(c.opSize, c.opReg)
}

// writeOpReg writes the prefix register at the current operand size.
func (c *CPU) writeOpReg(val uint32) {
	if c.opRegEx {
		c.writeRegEx(c.opSize, c.opReg, val)
	} else {
		c.writeReg(c.opSize, c.opReg, val)
	}
}

// setSR sets the status register, handling bank switching when RFP changes.
func (c *CPU) setSR(sr uint16) {
	// Preserve MAX bit (always 1)
	sr |= srMAX
	c.reg.SR = sr
}
