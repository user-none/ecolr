package tlcs900h

// Load, store, and exchange instructions.
// LD, LDA, PUSH, POP, EX, LDI, LDD, LDIR, LDDR, LDC, LINK, UNLK,
// INCF, DECF, LDF

func init() {
	// === Standalone baseOps (single-byte, no prefix) ===

	// 0x02: PUSH SR
	baseOps[0x02] = func(c *CPU, op uint8) {
		c.push(Word, uint32(c.reg.SR))
		c.cycles += 3
	}

	// 0x03: POP SR
	baseOps[0x03] = func(c *CPU, op uint8) {
		c.setSR(uint16(c.pop(Word)))
		c.cycles += 4
	}

	// 0x08: LD<B> (#8),#
	baseOps[0x08] = func(c *CPU, op uint8) {
		addr := uint32(c.fetchPC())
		val := uint32(c.fetchPC())
		c.writeBus(Byte, addr, val)
		c.cycles += 5
	}

	// 0x09: PUSH<B> #
	baseOps[0x09] = func(c *CPU, op uint8) {
		val := uint32(c.fetchPC())
		c.push(Byte, val)
		c.cycles += 4
	}

	// 0x0A: LD<W> (#8),##
	baseOps[0x0A] = func(c *CPU, op uint8) {
		addr := uint32(c.fetchPC())
		val := uint32(c.fetchPC16())
		c.writeBus(Word, addr, val)
		c.cycles += 6
	}

	// 0x0B: PUSH<W> ##
	baseOps[0x0B] = func(c *CPU, op uint8) {
		val := uint32(c.fetchPC16())
		c.push(Word, val)
		c.cycles += 5
	}

	// 0x0C: INCF
	baseOps[0x0C] = func(c *CPU, op uint8) {
		rfp := (c.reg.SR & srRFPMask) >> srRFPShift
		rfp = (rfp + 1) & 0x03
		c.reg.SR = (c.reg.SR &^ srRFPMask) | (rfp << srRFPShift)
		c.cycles += 2
	}

	// 0x0D: DECF
	baseOps[0x0D] = func(c *CPU, op uint8) {
		rfp := (c.reg.SR & srRFPMask) >> srRFPShift
		rfp = (rfp - 1) & 0x03
		c.reg.SR = (c.reg.SR &^ srRFPMask) | (rfp << srRFPShift)
		c.cycles += 2
	}

	// 0x14: PUSH A
	baseOps[0x14] = func(c *CPU, op uint8) {
		a := uint32(c.reg.ReadReg8(r8From3bit[1]))
		c.push(Byte, a)
		c.cycles += 3
	}

	// 0x15: POP A
	baseOps[0x15] = func(c *CPU, op uint8) {
		val := c.pop(Byte)
		c.reg.WriteReg8(r8From3bit[1], uint8(val))
		c.cycles += 4
	}

	// 0x16: EX F,F'
	baseOps[0x16] = func(c *CPU, op uint8) {
		c.swapFlags()
		c.cycles += 2
	}

	// 0x17: LDF #3
	baseOps[0x17] = func(c *CPU, op uint8) {
		imm := c.fetchPC()
		rfp := uint16(imm & 0x03)
		c.reg.SR = (c.reg.SR &^ srRFPMask) | (rfp << srRFPShift)
		c.cycles += 2
	}

	// 0x18: PUSH F
	baseOps[0x18] = func(c *CPU, op uint8) {
		c.push(Byte, uint32(c.flags()))
		c.cycles += 3
	}

	// 0x19: POP F
	baseOps[0x19] = func(c *CPU, op uint8) {
		val := c.pop(Byte)
		c.setFlags(uint8(val))
		c.cycles += 4
	}

	// 0x20-0x27: LD R,# (byte)
	for i := 0; i < 8; i++ {
		baseOps[0x20+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.fetchPC()
			c.reg.WriteReg8(r8From3bit[r], val)
			c.cycles += 2
		}
	}

	// 0x28-0x2F: PUSH RR (word)
	for i := 0; i < 8; i++ {
		baseOps[0x28+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := uint32(c.reg.ReadReg16(r))
			c.push(Word, val)
			c.cycles += 3
		}
	}

	// 0x30-0x37: LD RR,## (word)
	for i := 0; i < 8; i++ {
		baseOps[0x30+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.fetchPC16()
			c.reg.WriteReg16(r, val)
			c.cycles += 3
		}
	}

	// 0x38-0x3F: PUSH XRR (long)
	for i := 0; i < 8; i++ {
		baseOps[0x38+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.reg.ReadReg32(r)
			c.push(Long, val)
			c.cycles += 5
		}
	}

	// 0x40-0x47: LD XRR,#### (long)
	for i := 0; i < 8; i++ {
		baseOps[0x40+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.fetchPC32()
			c.reg.WriteReg32(r, val)
			c.cycles += 5
		}
	}

	// 0x48-0x4F: POP RR (word)
	for i := 0; i < 8; i++ {
		baseOps[0x48+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.pop(Word)
			c.reg.WriteReg16(r, uint16(val))
			c.cycles += 4
		}
	}

	// 0x58-0x5F: POP XRR (long)
	for i := 0; i < 8; i++ {
		baseOps[0x58+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.pop(Long)
			c.reg.WriteReg32(r, val)
			c.cycles += 6
		}
	}

	// === regOps (after register prefix) ===

	// 0x03: LD r,#
	regOps[0x03] = func(c *CPU, op uint8) {
		sz := c.opSize
		val := c.fetchImm(sz)
		c.writeOpReg(val)
		c.cycles += cyclesBWL(sz, 3, 4, 6)
	}

	// 0x04: PUSH r
	regOps[0x04] = func(c *CPU, op uint8) {
		sz := c.opSize
		val := c.readOpReg()
		c.push(sz, val)
		c.cycles += cyclesBWL(sz, 4, 4, 6)
	}

	// 0x05: POP r
	regOps[0x05] = func(c *CPU, op uint8) {
		sz := c.opSize
		val := c.pop(sz)
		c.writeOpReg(val)
		c.cycles += cyclesBWL(sz, 5, 5, 7)
	}

	// 0x0C: LINK r,d16 (long only)
	regOps[0x0C] = func(c *CPU, op uint8) {
		d16 := int16(c.fetchPC16())
		val := c.readOpReg()
		c.push(Long, val)
		c.writeOpReg(c.reg.XSP)
		c.reg.XSP += uint32(int32(d16))
		c.cycles += 8
	}

	// 0x0D: UNLK r (long only)
	regOps[0x0D] = func(c *CPU, op uint8) {
		c.reg.XSP = c.readOpReg()
		val := c.pop(Long)
		c.writeOpReg(val)
		c.cycles += 7
	}

	// 0x88-0x8F: LD R,r
	for i := 0; i < 8; i++ {
		regOps[0x88+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			val := c.readOpReg()
			c.writeReg(sz, op, val)
			c.cycles += 2
		}
	}

	// 0x98-0x9F: LD r,R
	for i := 0; i < 8; i++ {
		regOps[0x98+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			val := c.readReg(sz, op)
			c.writeOpReg(val)
			c.cycles += 2
		}
	}

	// 0xA8-0xAF: LD r,#3 (quick immediate 0-7)
	for i := 0; i < 8; i++ {
		regOps[0xA8+i] = func(c *CPU, op uint8) {
			val := uint32(op & 0x07)
			c.writeOpReg(val)
			c.cycles += 2
		}
	}

	// 0xB8-0xBF: EX R,r (byte/word only)
	for i := 0; i < 8; i++ {
		regOps[0xB8+i] = func(c *CPU, op uint8) {
			sz := c.opSize
			a := c.readReg(sz, op)
			b := c.readOpReg()
			c.writeReg(sz, op, b)
			c.writeOpReg(a)
			c.cycles += 3
		}
	}

	// 0x2E: LDC cr,r
	regOps[0x2E] = func(c *CPU, op uint8) {
		cr := c.fetchPC()
		val := c.readOpReg()
		c.writeControlReg(cr, val)
		c.cycles += 3
	}

	// 0x2F: LDC r,cr
	regOps[0x2F] = func(c *CPU, op uint8) {
		cr := c.fetchPC()
		val := c.readControlReg(cr)
		c.writeOpReg(val)
		c.cycles += 3
	}

	// === srcMemOps (source memory prefix) ===

	// 0x04: PUSH<W>(mem) [src]
	srcMemOps[0x04] = func(c *CPU, op uint8) {
		sz := c.opSize
		val := c.readBus(sz, c.opAddr)
		c.push(sz, val)
		c.cycles += 6
	}

	// 0x10: LDI<W>
	srcMemOps[0x10] = func(c *CPU, op uint8) {
		c.ldiLdd(1)
	}

	// 0x11: LDIR<W>
	srcMemOps[0x11] = func(c *CPU, op uint8) {
		c.ldirLddr(1)
	}

	// 0x12: LDD<W>
	srcMemOps[0x12] = func(c *CPU, op uint8) {
		c.ldiLdd(-1)
	}

	// 0x13: LDDR<W>
	srcMemOps[0x13] = func(c *CPU, op uint8) {
		c.ldirLddr(-1)
	}

	// 0x19: LD<W> (#16),(mem) [src prefix only]
	srcMemOps[0x19] = func(c *CPU, op uint8) {
		sz := c.opSize
		addr := uint32(c.fetchPC16())
		val := c.readBus(sz, c.opAddr)
		c.writeBus(sz, addr, val)
		c.cycles += 8
	}

	// 0x20-0x27: LD R,(mem) [src]
	for i := 0; i < 8; i++ {
		srcMemOps[0x20+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			sz := c.opSize
			val := c.readBus(sz, c.opAddr)
			c.writeReg(sz, r, val)
			c.cycles += cyclesBWL(sz, 4, 4, 6)
		}
	}

	// 0x30-0x37: EX (mem),R [src]
	for i := 0; i < 8; i++ {
		srcMemOps[0x30+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			sz := c.opSize
			memVal := c.readBus(sz, c.opAddr)
			regVal := c.readReg(sz, r)
			c.writeBus(sz, c.opAddr, regVal)
			c.writeReg(sz, r, memVal)
			c.cycles += 6
		}
	}

	// 0xF7: LDX (#8),# - Load Extract (8-bit bus mode block transfer)
	// Encoding: F7:00:addr:00:val:00 (6 bytes total)
	baseOps[0xF7] = func(c *CPU, op uint8) {
		c.fetchPC() // skip 0x00
		addr := uint32(c.fetchPC())
		c.fetchPC() // skip 0x00
		val := uint32(c.fetchPC())
		c.fetchPC() // skip 0x00
		c.writeBus(Byte, addr, val)
		c.cycles += 8
	}

	// === dstMemOps (destination memory prefix) ===

	// 0x00: LD<B> (mem),#
	dstMemOps[0x00] = func(c *CPU, op uint8) {
		val := uint32(c.fetchPC())
		c.writeBus(Byte, c.opAddr, val)
		c.cycles += 5
	}

	// 0x02: LD<W> (mem),#
	dstMemOps[0x02] = func(c *CPU, op uint8) {
		val := uint32(c.fetchPC16())
		c.writeBus(Word, c.opAddr, val)
		c.cycles += 6
	}

	// 0x04: POP<B>(mem) [dst]
	dstMemOps[0x04] = func(c *CPU, op uint8) {
		val := c.pop(Byte)
		c.writeBus(Byte, c.opAddr, val)
		c.cycles += 7
	}

	// 0x06: POP<W>(mem) [dst]
	dstMemOps[0x06] = func(c *CPU, op uint8) {
		val := c.pop(Word)
		c.writeBus(Word, c.opAddr, val)
		c.cycles += 7
	}

	// 0x14: LD<B> (mem),(#16) [dst]
	dstMemOps[0x14] = func(c *CPU, op uint8) {
		addr := uint32(c.fetchPC16())
		val := c.readBus(Byte, addr)
		c.writeBus(Byte, c.opAddr, val)
		c.cycles += 8
	}

	// 0x16: LD<W> (mem),(#16) [dst]
	dstMemOps[0x16] = func(c *CPU, op uint8) {
		addr := uint32(c.fetchPC16())
		val := c.readBus(Word, addr)
		c.writeBus(Word, c.opAddr, val)
		c.cycles += 8
	}

	// 0x20-0x27: LDA W R,mem [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x20+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			c.writeReg(Word, r, c.opAddr)
			c.cycles += 4
		}
	}

	// 0x30-0x37: LDA L R,mem [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x30+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			c.writeReg(Long, r, c.opAddr)
			c.cycles += 4
		}
	}

	// 0x40-0x47: LD<B> (mem),R [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x40+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.readReg(Byte, r)
			c.writeBus(Byte, c.opAddr, val)
			c.cycles += 4
		}
	}

	// 0x50-0x57: LD<W> (mem),R [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x50+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.readReg(Word, r)
			c.writeBus(Word, c.opAddr, val)
			c.cycles += 4
		}
	}

	// 0x60-0x67: LD<L> (mem),R [dst]
	for i := 0; i < 8; i++ {
		dstMemOps[0x60+i] = func(c *CPU, op uint8) {
			r := op & 0x07
			val := c.readReg(Long, r)
			c.writeBus(Long, c.opAddr, val)
			c.cycles += 6
		}
	}
}

// readControlReg reads a control register by code.
// DMA registers (DMAS0-3 at 0x00-0x0C, DMAD0-3 at 0x10-0x1C,
// DMAC0-3/DMAM0-3 at 0x20-0x2C) are not implemented because NGPC
// software does not use the DMA controller. Reads return 0 and
// writes are discarded for unimplemented registers.
func (c *CPU) readControlReg(cr uint8) uint32 {
	switch cr {
	case 0x3C:
		return uint32(c.intNest)
	}
	return 0
}

// writeControlReg writes a control register by code.
// See readControlReg for details on unimplemented registers.
func (c *CPU) writeControlReg(cr uint8, val uint32) {
	switch cr {
	case 0x3C:
		c.intNest = uint16(val)
	}
}

// ldiLdd performs a single LDI or LDD transfer.
// dir is +1 for LDI, -1 for LDD.
func (c *CPU) ldiLdd(dir int) {
	sz := c.opSize
	step := uint32(sz)

	var srcReg, dstReg uint8
	if c.opMemReg == 5 {
		srcReg = 5 // XIY
		dstReg = 4 // XIX
	} else {
		srcReg = 3 // XHL
		dstReg = 2 // XDE
	}

	src := c.reg.ReadReg32(srcReg)
	dst := c.reg.ReadReg32(dstReg)

	val := c.readBus(sz, src)
	c.writeBus(sz, dst, val)

	if dir > 0 {
		c.reg.WriteReg32(srcReg, src+step)
		c.reg.WriteReg32(dstReg, dst+step)
	} else {
		c.reg.WriteReg32(srcReg, src-step)
		c.reg.WriteReg32(dstReg, dst-step)
	}

	bc := c.reg.ReadReg16(1) - 1
	c.reg.WriteReg16(1, bc)

	// Flags: H=0, N=0, V = (BC != 0)
	f := c.flags()
	f &^= flagH | flagN | flagV
	if bc != 0 {
		f |= flagV
	}
	c.setFlags(f)
	c.cycles += 8
}

// ldirLddr performs LDIR or LDDR (repeat until BC=0).
func (c *CPU) ldirLddr(dir int) {
	sz := c.opSize
	step := uint32(sz)

	var srcReg, dstReg uint8
	if c.opMemReg == 5 {
		srcReg = 5 // XIY
		dstReg = 4 // XIX
	} else {
		srcReg = 3 // XHL
		dstReg = 2 // XDE
	}

	// BC=0 means 65536 transfers (do-while: decrement wraps 0 to 0xFFFF)
	count := uint64(c.reg.ReadReg16(1))
	if count == 0 {
		count = 65536
	}
	src := c.reg.ReadReg32(srcReg)
	dst := c.reg.ReadReg32(dstReg)

	for i := uint64(0); i < count; i++ {
		val := c.readBus(sz, src)
		c.writeBus(sz, dst, val)
		if dir > 0 {
			src += step
			dst += step
		} else {
			src -= step
			dst -= step
		}
	}

	c.reg.WriteReg32(srcReg, src)
	c.reg.WriteReg32(dstReg, dst)
	c.reg.WriteReg16(1, 0)

	// Flags: H=0, N=0, V=0 (BC is always 0 at end)
	f := c.flags()
	f &^= flagH | flagN | flagV
	c.setFlags(f)
	c.cycles += count*7 + 1
}
