package tlcs900h

// opFunc is the signature for an instruction handler.
// The op parameter is the opcode byte that dispatched to this handler.
type opFunc func(c *CPU, op uint8)

// Dispatch tables.
// baseOps handles first-byte dispatch (0x00-0xFF).
// Prefix bytes resolve an operand and then dispatch through secondary tables.
var baseOps [256]opFunc

// Secondary dispatch tables for the second opcode byte after a
// prefix byte has resolved the first operand.
var regOps [256]opFunc    // Register-source operations
var srcMemOps [256]opFunc // Source-memory operations
var dstMemOps [256]opFunc // Destination-memory operations

// unhandledOp is the default handler for unimplemented opcodes.
// It consumes 1 cycle (NOP equivalent).
func unhandledOp(c *CPU, _ uint8) { c.cycles++ }

// execute decodes and executes the instruction at PC.
func (c *CPU) execute() {
	op := c.fetchPC()
	if h := c.customOps[op]; h != nil {
		h(c)
		return
	}
	baseOps[op](c, op)
}

func init() {
	// Register prefixes: C8-CF (byte), D8-DF (word), E8-EF (long)
	for i := uint8(0); i < 8; i++ {
		regB := 0xC8 + i
		regW := 0xD8 + i
		regL := 0xE8 + i
		baseOps[regB] = prefixReg3bit(Byte)
		baseOps[regW] = prefixReg3bit(Word)
		baseOps[regL] = prefixReg3bit(Long)
	}

	// Extended register prefixes: C7 (byte), D7 (word), E7 (long)
	baseOps[0xC7] = prefixRegExtended(Byte)
	baseOps[0xD7] = prefixRegExtended(Word)
	baseOps[0xE7] = prefixRegExtended(Long)

	// Source memory prefixes: (R) indirect
	// 80-87 (byte), 90-97 (word), A0-A7 (long)
	for i := uint8(0); i < 8; i++ {
		baseOps[0x80+i] = prefixMemIndirect(Byte)
		baseOps[0x90+i] = prefixMemIndirect(Word)
		baseOps[0xA0+i] = prefixMemIndirect(Long)
	}

	// Source memory prefixes: (R+d8) indirect
	// 88-8F (byte), 98-9F (word), A8-AF (long)
	for i := uint8(0); i < 8; i++ {
		baseOps[0x88+i] = prefixMemDisp8(Byte)
		baseOps[0x98+i] = prefixMemDisp8(Word)
		baseOps[0xA8+i] = prefixMemDisp8(Long)
	}

	// Source memory prefixes: C0-C5 (byte), D0-D5 (word), E0-E5 (long)
	// x0=(n), x1=(nn), x2=(nnn), x3=reg indirect, x4=predec, x5=postinc
	for _, info := range []struct {
		base uint8
		sz   Size
	}{
		{0xC0, Byte},
		{0xD0, Word},
		{0xE0, Long},
	} {
		baseOps[info.base+0] = prefixMemImm8(info.sz)
		baseOps[info.base+1] = prefixMemImm16(info.sz)
		baseOps[info.base+2] = prefixMemImm24(info.sz)
		baseOps[info.base+3] = prefixMemRegIndirect(info.sz)
		baseOps[info.base+4] = prefixMemPreDec(info.sz)
		baseOps[info.base+5] = prefixMemPostInc(info.sz)
	}

	// Destination memory prefixes: B0-B7, B8-BF, F0-F5
	// F0=(n), F1=(nn), F2=(nnn), F3=reg indirect, F4=predec, F5=postinc
	for i := uint8(0); i < 8; i++ {
		baseOps[0xB0+i] = prefixDstIndirect
		baseOps[0xB8+i] = prefixDstDisp8
	}
	baseOps[0xF0] = prefixDstImm8
	baseOps[0xF1] = prefixDstImm16
	baseOps[0xF2] = prefixDstImm24
	baseOps[0xF3] = prefixDstRegIndirect
	baseOps[0xF4] = prefixDstPreDec
	baseOps[0xF5] = prefixDstPostInc

	// Fill nil entries with default handler to avoid nil checks on dispatch.
	for i := range baseOps {
		if baseOps[i] == nil {
			baseOps[i] = unhandledOp
		}
	}
	for i := range regOps {
		if regOps[i] == nil {
			regOps[i] = unhandledOp
		}
	}
	for i := range srcMemOps {
		if srcMemOps[i] == nil {
			srcMemOps[i] = unhandledOp
		}
	}
	for i := range dstMemOps {
		if dstMemOps[i] == nil {
			dstMemOps[i] = unhandledOp
		}
	}
}

// prefixReg3bit returns a handler for 3-bit register prefix opcodes.
func prefixReg3bit(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		c.opReg = op & 0x07
		c.opRegEx = false
		c.dispatchReg(c.fetchPC())
	}
}

// prefixRegExtended returns a handler for extended register prefix opcodes.
// Extended register mode adds +1 extra state per the databook.
func prefixRegExtended(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		c.opReg = c.fetchPC()
		c.opRegEx = true
		c.cycles++ // +1 extra state for extended register mode
		c.dispatchReg(c.fetchPC())
	}
}

// prefixMemIndirect returns a handler for (R) indirect memory prefix.
func prefixMemIndirect(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		r := op & 0x07
		c.opMemReg = r
		c.opAddr = c.reg.ReadReg32(r)
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemDisp8 returns a handler for (R+d8) indirect memory prefix.
// Adds +1 extra state for displacement computation.
func prefixMemDisp8(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		r := op & 0x07
		base := c.reg.ReadReg32(r)
		d8 := int8(c.fetchPC())
		c.opAddr = (base + uint32(int32(d8))) & addrMask
		c.cycles++ // +1 extra state for (R+d8) mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemImm8 returns a handler for (#8) direct memory prefix.
// Adds +1 extra state.
func prefixMemImm8(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		c.opAddr = uint32(c.fetchPC())
		c.cycles++ // +1 extra state for (#8) mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemImm16 returns a handler for (#16) direct memory prefix.
// Adds +2 extra states.
func prefixMemImm16(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		c.opAddr = uint32(c.fetchPC16())
		c.cycles += 2 // +2 extra states for (#16) mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemImm24 returns a handler for (#24) direct memory prefix.
// Adds +3 extra states.
func prefixMemImm24(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		c.opAddr = c.fetchPC24()
		c.cycles += 3 // +3 extra states for (#24) mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemRegIndirect returns a handler for register indirect memory prefix
// at position x3. Reads a sub-mode byte to determine the addressing mode:
//
//	(code & 0x03) == 0x00: (R) register indirect
//	(code & 0x03) == 0x01: (R+d16) register + 16-bit displacement
//	code == 0x03: (R+r8) register + 8-bit register index
//	code == 0x07: (R+r16) register + 16-bit register index
//	code == 0x13: (PC+d16) program counter relative
func prefixMemRegIndirect(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		code := c.fetchPC()
		switch code & 0x03 {
		case 0x00: // (R) register indirect
			c.opAddr = *c.reg.regPtrByFullCode32(code) & addrMask
			c.cycles++ // +1 extra state for (R) x3 mode
		case 0x01: // (R+d16)
			base := *c.reg.regPtrByFullCode32(code)
			d16 := int16(c.fetchPC16())
			c.opAddr = (base + uint32(int32(d16))) & addrMask
			c.cycles += 3 // +3 extra states for (R+d16) x3 mode
		case 0x03: // sub-sub modes
			switch code {
			case 0x03: // (R+r8)
				regCode := c.fetchPC()
				base := *c.reg.regPtrByFullCode32(regCode)
				idxCode := c.fetchPC()
				idx := int8(c.reg.ReadReg8ByFullCode(idxCode))
				c.opAddr = (base + uint32(int32(idx))) & addrMask
				c.cycles += 3 // +3 extra states for (R+r8) x3 mode
			case 0x07: // (R+r16)
				regCode := c.fetchPC()
				base := *c.reg.regPtrByFullCode32(regCode)
				idxCode := c.fetchPC()
				idx := int16(c.reg.ReadReg16ByFullCode(idxCode))
				c.opAddr = (base + uint32(int32(idx))) & addrMask
				c.cycles += 3 // +3 extra states for (R+r16) x3 mode
			case 0x13: // (PC+d16)
				d16 := int16(c.fetchPC16())
				c.opAddr = (c.reg.PC + uint32(int32(d16))) & addrMask
				c.cycles += 3 // +3 extra states for (PC+d16) x3 mode
			default:
				c.cycles++
				return
			}
		default: // 0x02: illegal
			c.cycles++
			return
		}
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemPreDec returns a handler for pre-decrement memory prefix.
// The register code byte encodes the register in bits 7-2 and step size
// in bits 1-0 (0=1, 1=2, 2=4). Adds +1 extra state.
func prefixMemPreDec(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		code := c.fetchPC()
		p := c.reg.regPtrByFullCode32(code)
		step := uint32(1) << (code & 0x03)
		*p -= step
		c.opAddr = *p & addrMask
		c.cycles++ // +1 extra state for pre-decrement mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// prefixMemPostInc returns a handler for post-increment memory prefix.
// The register code byte encodes the register in bits 7-2 and step size
// in bits 1-0 (0=1, 1=2, 2=4). Adds +1 extra state.
func prefixMemPostInc(sz Size) opFunc {
	return func(c *CPU, op uint8) {
		c.opSize = sz
		code := c.fetchPC()
		p := c.reg.regPtrByFullCode32(code)
		c.opAddr = *p & addrMask
		step := uint32(1) << (code & 0x03)
		*p += step
		c.cycles++ // +1 extra state for post-increment mode
		c.dispatchSrcMem(c.fetchPC())
	}
}

// dispatchReg dispatches from a register prefix.
func (c *CPU) dispatchReg(op2 uint8) {
	regOps[op2](c, op2)
}

// dispatchSrcMem dispatches from a source memory prefix.
func (c *CPU) dispatchSrcMem(op2 uint8) {
	srcMemOps[op2](c, op2)
}

// dispatchDstMem dispatches from a destination memory prefix.
func (c *CPU) dispatchDstMem(op2 uint8) {
	dstMemOps[op2](c, op2)
}

// Destination memory prefixes - these do not encode size in the prefix.
// Size comes from the second byte's encoding.

func prefixDstIndirect(c *CPU, op uint8) {
	c.opSize = 0 // determined by second byte
	r := op & 0x07
	c.opAddr = c.reg.ReadReg32(r)
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

func prefixDstDisp8(c *CPU, op uint8) {
	c.opSize = 0
	r := op & 0x07
	base := c.reg.ReadReg32(r)
	d8 := int8(c.fetchPC())
	c.opAddr = (base + uint32(int32(d8))) & addrMask
	c.cycles++ // +1 extra state for (R+d8) mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

func prefixDstImm8(c *CPU, op uint8) {
	c.opSize = 0
	c.opAddr = uint32(c.fetchPC())
	c.cycles++ // +1 extra state for (#8) mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

func prefixDstImm16(c *CPU, op uint8) {
	c.opSize = 0
	c.opAddr = uint32(c.fetchPC16())
	c.cycles += 2 // +2 extra states for (#16) mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

func prefixDstImm24(c *CPU, op uint8) {
	c.opSize = 0
	c.opAddr = c.fetchPC24()
	c.cycles += 3 // +3 extra states for (#24) mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

// prefixDstRegIndirect handles register indirect sub-modes for destination prefix.
func prefixDstRegIndirect(c *CPU, op uint8) {
	c.opSize = 0
	code := c.fetchPC()
	switch code & 0x03 {
	case 0x00: // (R) register indirect
		c.opAddr = *c.reg.regPtrByFullCode32(code) & addrMask
		c.cycles++ // +1 extra state for (R) x3 mode
	case 0x01: // (R+d16)
		base := *c.reg.regPtrByFullCode32(code)
		d16 := int16(c.fetchPC16())
		c.opAddr = (base + uint32(int32(d16))) & addrMask
		c.cycles += 3 // +3 extra states for (R+d16) x3 mode
	case 0x03: // sub-sub modes
		switch code {
		case 0x03: // (R+r8)
			regCode := c.fetchPC()
			base := *c.reg.regPtrByFullCode32(regCode)
			idxCode := c.fetchPC()
			idx := int8(c.reg.ReadReg8ByFullCode(idxCode))
			c.opAddr = (base + uint32(int32(idx))) & addrMask
			c.cycles += 3 // +3 extra states for (R+r8) x3 mode
		case 0x07: // (R+r16)
			regCode := c.fetchPC()
			base := *c.reg.regPtrByFullCode32(regCode)
			idxCode := c.fetchPC()
			idx := int16(c.reg.ReadReg16ByFullCode(idxCode))
			c.opAddr = (base + uint32(int32(idx))) & addrMask
			c.cycles += 3 // +3 extra states for (R+r16) x3 mode
		case 0x13: // (PC+d16)
			d16 := int16(c.fetchPC16())
			c.opAddr = (c.reg.PC + uint32(int32(d16))) & addrMask
			c.cycles += 3 // +3 extra states for (PC+d16) x3 mode
		default:
			c.cycles++
			return
		}
	default: // 0x02: illegal
		c.cycles++
		return
	}
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

// prefixDstPreDec handles pre-decrement for destination prefix.
// Register code byte: bits 7-2 = register, bits 1-0 = step size.
// Adds +1 extra state.
func prefixDstPreDec(c *CPU, op uint8) {
	c.opSize = 0
	code := c.fetchPC()
	p := c.reg.regPtrByFullCode32(code)
	step := uint32(1) << (code & 0x03)
	*p -= step
	c.opAddr = *p & addrMask
	c.cycles++ // +1 extra state for pre-decrement mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}

// prefixDstPostInc handles post-increment for destination prefix.
// Register code byte: bits 7-2 = register, bits 1-0 = step size.
// Adds +1 extra state.
func prefixDstPostInc(c *CPU, op uint8) {
	c.opSize = 0
	code := c.fetchPC()
	p := c.reg.regPtrByFullCode32(code)
	c.opAddr = *p & addrMask
	step := uint32(1) << (code & 0x03)
	*p += step
	c.cycles++ // +1 extra state for post-increment mode
	op2 := c.fetchPC()
	c.dispatchDstMem(op2)
}
