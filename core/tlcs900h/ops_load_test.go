package tlcs900h

import "testing"

// --- Standalone baseOps ---

func TestPUSH_POP_SR(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// Set SR to a known value
	c.setSR(0xF987) // IFF=7, MAX, RFP=1, flags=0x87
	origSR := c.reg.SR

	// PUSH SR (opcode 0x02)
	c.bus.Write8(0x1000, 0x02)
	c.Step()

	// POP SR (opcode 0x03) into different SR
	c.setSR(0x0800) // clear everything except MAX
	c.bus.Write8(c.reg.PC, 0x03)
	c.Step()

	if c.reg.SR != origSR {
		t.Errorf("POP SR = 0x%04X, want 0x%04X", c.reg.SR, origSR)
	}
}

func TestPUSH_SR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x02)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("PUSH SR cycles = %d, want 3", cycles)
	}
}

func TestPOP_SR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// Push something first
	c.push(Word, uint32(c.reg.SR))
	c.bus.Write8(0x1000, 0x03)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("POP SR cycles = %d, want 4", cycles)
	}
}

func TestPUSH_POP_A(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.WriteReg8(r8From3bit[1], 0xAB) // A = 0xAB

	// PUSH A (0x14)
	c.bus.Write8(0x1000, 0x14)
	c.Step()

	// Clear A
	c.reg.WriteReg8(r8From3bit[1], 0x00)

	// POP A (0x15)
	c.bus.Write8(c.reg.PC, 0x15)
	c.Step()

	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAB)
}

func TestPUSH_A_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x14)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("PUSH A cycles = %d, want 3", cycles)
	}
}

func TestPOP_A_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.push(Byte, 0x42)
	c.bus.Write8(0x1000, 0x15)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("POP A cycles = %d, want 4", cycles)
	}
}

func TestPUSH_POP_F(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.setFlags(0xC5) // S=1,Z=1,V=1,C=1

	// PUSH F (0x18)
	c.bus.Write8(0x1000, 0x18)
	c.Step()

	// Clear flags
	c.setFlags(0x00)

	// POP F (0x19)
	c.bus.Write8(c.reg.PC, 0x19)
	c.Step()

	if c.flags() != 0xC5 {
		t.Errorf("POP F flags = 0x%02X, want 0xC5", c.flags())
	}
}

func TestPUSH_F_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x18)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("PUSH F cycles = %d, want 3", cycles)
	}
}

func TestPOP_F_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.push(Byte, 0x00)
	c.bus.Write8(0x1000, 0x19)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("POP F cycles = %d, want 4", cycles)
	}
}

func TestEX_FF(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.setFlags(0xC5) // F = 0xC5
	c.reg.FP = 0x12  // F' = 0x12

	// EX F,F' (0x16)
	c.bus.Write8(0x1000, 0x16)
	c.Step()

	if c.flags() != 0x12 {
		t.Errorf("F after EX = 0x%02X, want 0x12", c.flags())
	}
	if c.reg.FP != 0xC5 {
		t.Errorf("F' after EX = 0x%02X, want 0xC5", c.reg.FP)
	}
}

func TestEX_FF_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x16)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("EX F,F' cycles = %d, want 2", cycles)
	}
}

func TestINCF(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// Set RFP to 0
	c.reg.SR = (c.reg.SR &^ srRFPMask) | (0 << srRFPShift)
	c.bus.Write8(0x1000, 0x0C)
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 1 {
		t.Errorf("INCF: RFP = %d, want 1", rfp)
	}
}

func TestINCF_Wrap(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.SR = (c.reg.SR &^ srRFPMask) | (3 << srRFPShift)
	c.bus.Write8(0x1000, 0x0C)
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 0 {
		t.Errorf("INCF wrap: RFP = %d, want 0", rfp)
	}
}

func TestINCF_BankSwitch(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// Set bank 0 XWA
	c.reg.SR = (c.reg.SR &^ srRFPMask) | (0 << srRFPShift)
	c.reg.Bank[0].XWA = 0x1111
	c.reg.Bank[1].XWA = 0x2222

	// INCF switches to bank 1
	c.bus.Write8(0x1000, 0x0C)
	c.Step()

	// Now reading XWA should give bank 1's value
	got := c.reg.ReadReg32(0)
	if got != 0x2222 {
		t.Errorf("after INCF, XWA = 0x%08X, want 0x2222", got)
	}
}

func TestINCF_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x0C)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("INCF cycles = %d, want 2", cycles)
	}
}

func TestDECF(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.SR = (c.reg.SR &^ srRFPMask) | (2 << srRFPShift)
	c.bus.Write8(0x1000, 0x0D)
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 1 {
		t.Errorf("DECF: RFP = %d, want 1", rfp)
	}
}

func TestDECF_Wrap(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.SR = (c.reg.SR &^ srRFPMask) | (0 << srRFPShift)
	c.bus.Write8(0x1000, 0x0D)
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 3 {
		t.Errorf("DECF wrap: RFP = %d, want 3", rfp)
	}
}

func TestDECF_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x0D)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("DECF cycles = %d, want 2", cycles)
	}
}

func TestLDF(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x17)
	c.bus.Write8(0x1001, 0x02) // set RFP to 2
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 2 {
		t.Errorf("LDF: RFP = %d, want 2", rfp)
	}
}

func TestLDF_MasksTo2Bits(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x17)
	c.bus.Write8(0x1001, 0xFF) // only low 2 bits used = 3
	c.Step()
	rfp := int((c.reg.SR & srRFPMask) >> srRFPShift)
	if rfp != 3 {
		t.Errorf("LDF mask: RFP = %d, want 3", rfp)
	}
}

func TestLDF_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x17)
	c.bus.Write8(0x1001, 0x00)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("LDF cycles = %d, want 2", cycles)
	}
}

// --- LD R,# / PUSH R / POP R (standalone baseOps) ---

func TestLD_R_Imm_Byte(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// LD R1,# byte: opcode 0x21, imm8
	c.bus.Write8(0x1000, 0x21)
	c.bus.Write8(0x1001, 0xAB)
	c.Step()
	// R1 in byte = A (r8From3bit[1] = 0x00)
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAB)
}

func TestLD_R_Imm_Byte_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x20)
	c.bus.Write8(0x1001, 0x00)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("LD R,# byte cycles = %d, want 2", cycles)
	}
}

func TestLD_RR_Imm_Word(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// LD RR0,## word: opcode 0x30, imm16
	c.bus.Write8(0x1000, 0x30)
	c.bus.Write16(0x1001, 0x1234)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1234)
}

func TestLD_RR_Imm_Word_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x30)
	c.bus.Write16(0x1001, 0x0000)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("LD RR,## word cycles = %d, want 3", cycles)
	}
}

func TestLD_XRR_Imm_Long(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	// LD XRR1,#### long: opcode 0x41, imm32
	c.bus.Write8(0x1000, 0x41)
	c.bus.Write32(0x1001, 0xDEADBEEF)
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0xDEADBEEF)
}

func TestLD_XRR_Imm_Long_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x40)
	c.bus.Write32(0x1001, 0x00000000)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("LD XRR,#### long cycles = %d, want 5", cycles)
	}
}

func TestPUSH_POP_RR_Word(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.WriteReg16(1, 0xBEEF) // BC

	// PUSH RR1 (0x29)
	c.bus.Write8(0x1000, 0x29)
	c.Step()

	c.reg.WriteReg16(1, 0x0000) // clear BC

	// POP RR1 (0x49)
	c.bus.Write8(c.reg.PC, 0x49)
	c.Step()

	checkReg16(t, "BC", c.reg.ReadReg16(1), 0xBEEF)
}

func TestPUSH_RR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x28)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("PUSH RR cycles = %d, want 3", cycles)
	}
}

func TestPOP_RR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.push(Word, 0x1234)
	c.bus.Write8(0x1000, 0x48)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("POP RR cycles = %d, want 4", cycles)
	}
}

func TestPUSH_POP_XRR_Long(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.reg.WriteReg32(2, 0xCAFEBABE) // XDE

	// PUSH XRR2 (0x3A)
	c.bus.Write8(0x1000, 0x3A)
	c.Step()

	c.reg.WriteReg32(2, 0x00000000)

	// POP XRR2 (0x5A)
	c.bus.Write8(c.reg.PC, 0x5A)
	c.Step()

	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0xCAFEBABE)
}

func TestPUSH_XRR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.bus.Write8(0x1000, 0x38)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("PUSH XRR cycles = %d, want 5", cycles)
	}
}

func TestPOP_XRR_Cycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.push(Long, 0x12345678)
	c.bus.Write8(0x1000, 0x58)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("POP XRR cycles = %d, want 6", cycles)
	}
}

// --- LD<B>(#8),# and PUSH<B># ---

func TestLD_B_Imm8Addr(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	// LD<B> (#8),# : opcode 0x08, addr8, imm8
	bus.write8(0x1000, 0x08)
	bus.write8(0x1001, 0x50) // addr = 0x50
	bus.write8(0x1002, 0xAA) // val = 0xAA
	c.Step()
	got := bus.Read8(0x50)
	if got != 0xAA {
		t.Errorf("LD<B>(#8),# = 0x%02X, want 0xAA", got)
	}
}

func TestLD_B_Imm8Addr_Cycles(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x08)
	bus.write8(0x1001, 0x00)
	bus.write8(0x1002, 0x00)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("LD<B>(#8),# cycles = %d, want 5", cycles)
	}
}

func TestPUSH_B_Imm(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	spBefore := c.reg.XSP
	bus.write8(0x1000, 0x09)
	bus.write8(0x1001, 0x42)
	c.Step()
	if c.reg.XSP != spBefore-1 {
		t.Errorf("SP after PUSH<B># = 0x%04X, want 0x%04X", c.reg.XSP, spBefore-1)
	}
	got := bus.Read8(c.reg.XSP)
	if got != 0x42 {
		t.Errorf("pushed value = 0x%02X, want 0x42", got)
	}
}

func TestPUSH_B_Imm_Cycles(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x09)
	bus.write8(0x1001, 0x00)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("PUSH<B># cycles = %d, want 4", cycles)
	}
}

func TestLD_W_Imm8Addr(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	// LD<W> (#8),## : opcode 0x0A, addr8, imm16
	bus.write8(0x1000, 0x0A)
	bus.write8(0x1001, 0x50) // addr = 0x50
	bus.write16LE(0x1002, 0x1234)
	c.Step()
	got := bus.Read16(0x50)
	if got != 0x1234 {
		t.Errorf("LD<W>(#8),## = 0x%04X, want 0x1234", got)
	}
}

func TestLD_W_Imm8Addr_Cycles(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x0A)
	bus.write8(0x1001, 0x00)
	bus.write16LE(0x1002, 0x0000)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("LD<W>(#8),## cycles = %d, want 6", cycles)
	}
}

func TestPUSH_W_Imm(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	spBefore := c.reg.XSP
	bus.write8(0x1000, 0x0B)
	bus.write16LE(0x1001, 0xBEEF)
	c.Step()
	if c.reg.XSP != spBefore-2 {
		t.Errorf("SP after PUSH<W>## = 0x%04X, want 0x%04X", c.reg.XSP, spBefore-2)
	}
	got := bus.Read16(c.reg.XSP)
	if got != 0xBEEF {
		t.Errorf("pushed value = 0x%04X, want 0xBEEF", got)
	}
}

func TestPUSH_W_Imm_Cycles(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x0B)
	bus.write16LE(0x1001, 0x0000)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("PUSH<W>## cycles = %d, want 5", cycles)
	}
}

// --- regOps: LD R,r / LD r,R / LD r,#3 / LD r,# ---

func TestLD_Rr_RegReg_Byte(t *testing.T) {
	// LD R,r: prefix byte reg1(A), op2=0x88 (R0=W)
	// W = W <- A (prefix)
	c, _ := setupRegOp(t, 0xC9, 0x88) // prefix=byte A, LD R0(W),r(A)
	c.reg.WriteReg8(r8From3bit[1], 0xAB)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xAB)
}

func TestLD_Rr_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x88)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("LD R,r byte cycles = %d, want 2", cycles)
	}
}

func TestLD_rR_RegReg_Byte(t *testing.T) {
	// LD r,R: prefix byte reg1(A), op2=0x98 (R0=W)
	// A = A <- W
	c, _ := setupRegOp(t, 0xC9, 0x98) // prefix=byte A, LD r(A),R0(W)
	c.reg.WriteReg8(r8From3bit[0], 0xCD)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xCD)
}

func TestLD_rR_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x98)
	c.reg.WriteReg8(r8From3bit[0], 0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("LD r,R byte cycles = %d, want 2", cycles)
	}
}

func TestLD_Rr_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x89) // prefix=word reg0(WA), LD R1(BC),r(WA)
	c.reg.WriteReg16(0, 0x1234)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x1234)
}

func TestLD_Rr_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x89) // prefix=long reg0(XWA), LD R1(XBC),r(XWA)
	c.reg.WriteReg32(0, 0xDEADBEEF)
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0xDEADBEEF)
}

func TestLD_rQuick_Byte(t *testing.T) {
	// LD r,#3: prefix byte reg1(A), op2=0xAB (#3=3)
	c, _ := setupRegOp(t, 0xC9, 0xAB) // LD A,3
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 3)
}

func TestLD_rQuick_Zero(t *testing.T) {
	// LD r,#3 with #3=0
	c, _ := setupRegOp(t, 0xC9, 0xA8)
	c.reg.WriteReg8(r8From3bit[1], 0xFF) // pre-set to non-zero
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0)
}

func TestLD_rQuick_Seven(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xAF) // LD A,7
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 7)
}

func TestLD_rQuick_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xAD) // LD WA,5
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 5)
}

func TestLD_rQuick_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xAD) // LD XWA,5
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 5)
}

func TestLD_rQuick_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xA8)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("LD r,#3 cycles = %d, want 2", cycles)
	}
}

func TestLD_rImm_Byte(t *testing.T) {
	// LD r,#: prefix byte reg1(A), op2=0x03, imm8
	c, _ := setupRegOp(t, 0xC9, 0x03, 0x42)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x42)
}

func TestLD_rImm_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x03, 0x34, 0x12) // LD WA,0x1234
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1234)
}

func TestLD_rImm_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x03, 0xEF, 0xBE, 0xAD, 0xDE) // LD XWA,0xDEADBEEF
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0xDEADBEEF)
}

func TestLD_rImm_Byte_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x03, 0x00)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("LD r,# byte cycles = %d, want 3", cycles)
	}
}

func TestLD_rImm_Word_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x03, 0x00, 0x00)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("LD r,# word cycles = %d, want 4", cycles)
	}
}

func TestLD_rImm_Long_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x03, 0x00, 0x00, 0x00, 0x00)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("LD r,# long cycles = %d, want 6", cycles)
	}
}

// --- regOps: PUSH r / POP r ---

func TestPUSH_POP_r_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x04) // PUSH A (byte prefix reg1)
	c.reg.WriteReg8(r8From3bit[1], 0x55)
	c.Step()

	// Now POP it back
	c, _ = setupRegOp(t, 0xC9, 0x05) // POP A
	c.reg.WriteReg8(r8From3bit[1], 0x55)
	c.push(Byte, 0x55)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x55)
}

func TestPUSH_r_Byte_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x04)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("PUSH r byte cycles = %d, want 4", cycles)
	}
}

func TestPOP_r_Byte_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x05)
	c.push(Byte, 0)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("POP r byte cycles = %d, want 5", cycles)
	}
}

func TestPUSH_r_Word_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x04)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("PUSH r word cycles = %d, want 4", cycles)
	}
}

func TestPOP_r_Long_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x05)
	c.push(Long, 0)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("POP r long cycles = %d, want 7", cycles)
	}
}

// --- regOps: EX R,r ---

func TestEX_Rr_Byte(t *testing.T) {
	// EX R,r: prefix byte reg1(A), op2=0xB8 (R0=W)
	c, _ := setupRegOp(t, 0xC9, 0xB8)
	c.reg.WriteReg8(r8From3bit[0], 0x11) // W
	c.reg.WriteReg8(r8From3bit[1], 0x22) // A (prefix)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x22)
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x11)
}

func TestEX_Rr_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xB9) // prefix=word reg0(WA), EX R1(BC),r(WA)
	c.reg.WriteReg16(0, 0x1111)
	c.reg.WriteReg16(1, 0x2222)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x2222)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x1111)
}

func TestEX_Rr_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xB8)
	c.reg.WriteReg8(r8From3bit[0], 0)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("EX R,r byte cycles = %d, want 3", cycles)
	}
}

// --- regOps: LDC ---

func TestLDC_RoundTrip(t *testing.T) {
	// LDC doesn't actually store anything on NGPC (no DMA)
	// but we verify the instruction executes and cycles are correct
	c, _ := setupRegOp(t, 0xD8, 0x2E, 0x10) // LDC cr(0x10),r(WA)
	c.reg.WriteReg16(0, 0x1234)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("LDC cr,r cycles = %d, want 3", cycles)
	}
}

func TestLDC_ReadCR(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x2F, 0x10) // LDC r(WA),cr(0x10)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("LDC r,cr cycles = %d, want 3", cycles)
	}
	// Should read 0 since NGPC has no DMA
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0000)
}

func TestLDC_WriteINTNEST(t *testing.T) {
	// LDC cr(0x3C),r(WA) - write INTNEST
	c, _ := setupRegOp(t, 0xD8, 0x2E, 0x3C)
	c.reg.WriteReg16(0, 0x0003)
	c.Step()
	if c.intNest != 3 {
		t.Errorf("intNest = %d, want 3", c.intNest)
	}
}

func TestLDC_ReadINTNEST(t *testing.T) {
	// LDC r(WA),cr(0x3C) - read INTNEST
	c, _ := setupRegOp(t, 0xD8, 0x2F, 0x3C)
	c.intNest = 5
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 5)
}

// --- regOps: LINK / UNLK ---

func TestLINK_UNLK_RoundTrip(t *testing.T) {
	// LINK r,d16: push r, r=SP, SP=SP+d16
	// Use long prefix for LINK
	c, _ := setupRegOp(t, 0xE8, 0x0C, 0xF0, 0xFF) // LINK XWA, d16=-16
	c.reg.WriteReg32(0, 0x12345678)               // XWA = original value
	spBefore := c.reg.XSP
	c.Step()

	// After LINK: old XWA pushed, XWA = old SP-4, SP = old SP-4 + (-16)
	wantSP := spBefore - 4 - 16
	if c.reg.XSP != wantSP {
		t.Errorf("SP after LINK = 0x%08X, want 0x%08X", c.reg.XSP, wantSP)
	}

	// UNLK: SP = r, r = pop
	// Set up for UNLK
	pc2 := c.reg.PC
	c.bus.Write8(pc2, 0xE8)   // long prefix reg 0
	c.bus.Write8(pc2+1, 0x0D) // UNLK
	c.Step()

	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0x12345678)
	if c.reg.XSP != spBefore {
		t.Errorf("SP after UNLK = 0x%08X, want 0x%08X", c.reg.XSP, spBefore)
	}
}

func TestLINK_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x0C, 0x00, 0x00) // LINK XWA, d16=0
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("LINK cycles = %d, want 8", cycles)
	}
}

func TestUNLK_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x0D) // UNLK XWA
	// Set XWA to point to valid stack area with data
	c.reg.WriteReg32(0, c.reg.XSP)
	c.push(Long, 0) // put something on the stack
	c.reg.WriteReg32(0, c.reg.XSP)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("UNLK cycles = %d, want 7", cycles)
	}
}

// --- memOps: LD R,(mem) ---

func TestLD_R_Mem_Byte(t *testing.T) {
	// Prefix 0x80 = byte (R0) indirect, op2 = 0x21 = LD R1(A),(mem)
	c, bus := setupMemOp(t, 0x80, 0x21)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xAB)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAB)
}

func TestLD_R_Mem_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x21) // word (R0) indirect
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1234)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x1234)
}

func TestLD_R_Mem_Long(t *testing.T) {
	c, bus := setupMemOp(t, 0xA0, 0x21) // long (R0) indirect
	c.reg.WriteReg32(0, 0x2000)
	bus.Write32(0x2000, 0xDEADBEEF)
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0xDEADBEEF)
}

func TestLD_R_Mem_Byte_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x21)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("LD R,(mem) byte cycles = %d, want 4", cycles)
	}
}

func TestLD_R_Mem_Long_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0xA0, 0x21)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write32(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("LD R,(mem) long cycles = %d, want 6", cycles)
	}
}

// --- memOps: LDA ---

func TestLDA_Word(t *testing.T) {
	// Dst prefix 0xB0 = (R0) indirect, op2 = 0x21 = LDA W R1,mem
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)    // dst indirect R0
	bus.write8(0x1001, 0x21)    // LDA W R1,mem
	c.reg.WriteReg32(0, 0x5000) // R0 = 0x5000 (effective address)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x5000)
}

func TestLDA_Long(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)    // dst indirect R0
	bus.write8(0x1001, 0x31)    // LDA L R1,mem
	c.reg.WriteReg32(0, 0x5000) // effective address
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x5000)
}

func TestLDA_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x20)
	c.reg.WriteReg32(0, 0x5000)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("LDA W cycles = %d, want 4", cycles)
	}
}

// --- memOps: LD (mem),R (dst prefix) ---

func TestLD_Mem_R_Byte(t *testing.T) {
	// Dst prefix 0xB2 = (R2=XDE), op2 = 0x41 = LD<B> (mem),R1
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB2)             // dst indirect R2 (XDE)
	bus.write8(0x1001, 0x41)             // LD<B> (mem),R1
	c.reg.WriteReg32(2, 0x2000)          // XDE = pointer
	c.reg.WriteReg8(r8From3bit[1], 0xAB) // A (R1 byte)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xAB {
		t.Errorf("LD<B>(mem),R = 0x%02X, want 0xAB", got)
	}
}

func TestLD_Mem_R_Word(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x51) // LD<W> (mem),R1
	c.reg.WriteReg32(0, 0x2000)
	c.reg.WriteReg16(1, 0xBEEF)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0xBEEF {
		t.Errorf("LD<W>(mem),R = 0x%04X, want 0xBEEF", got)
	}
}

func TestLD_Mem_R_Long(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x61) // LD<L> (mem),R1
	c.reg.WriteReg32(0, 0x2000)
	c.reg.WriteReg32(1, 0xCAFEBABE)
	c.Step()
	got := bus.Read32(0x2000)
	if got != 0xCAFEBABE {
		t.Errorf("LD<L>(mem),R = 0x%08X, want 0xCAFEBABE", got)
	}
}

func TestLD_Mem_R_Byte_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB2) // dst indirect R2 (XDE)
	bus.write8(0x1001, 0x40) // LD<B> (mem),R0
	c.reg.WriteReg32(2, 0x2000)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("LD<B>(mem),R cycles = %d, want 4", cycles)
	}
}

func TestLD_Mem_R_Long_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x60)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("LD<L>(mem),R cycles = %d, want 6", cycles)
	}
}

// --- src/dst table separation ---

func TestWrapDstMem_MUL_StillWorks(t *testing.T) {
	// MUL RR,(mem) via source prefix should still work
	// Prefix 0x81 = byte (R1) indirect, op2 = 0x40 = MUL RR,(mem) R0
	c, bus := setupMemOp(t, 0x81, 0x40)
	c.reg.WriteReg32(1, 0x2000)       // XBC = pointer
	bus.write8(0x2000, 7)             // mem = 7
	c.reg.WriteReg8(r8From3bit[0], 6) // W = 6
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 42)
}

func TestWrapDstMem_DIV_StillWorks(t *testing.T) {
	// DIV RR,(mem) via source prefix
	c, bus := setupMemOp(t, 0x81, 0x50) // byte (R1), DIV R0
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 5)    // divisor
	c.reg.WriteReg16(0, 100) // dividend in WA
	c.Step()
	got := c.reg.ReadReg16(0)
	quotient := uint8(got)
	remainder := uint8(got >> 8)
	if quotient != 20 {
		t.Errorf("DIV quotient = %d, want 20", quotient)
	}
	if remainder != 0 {
		t.Errorf("DIV remainder = %d, want 0", remainder)
	}
}

func TestWrapDstMem_INC_StillWorks(t *testing.T) {
	// INC #3,(mem) via source prefix
	c, bus := setupMemOp(t, 0x90, 0x62) // word (R0), INC 2
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1000)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0x1002 {
		t.Errorf("INC (mem) = 0x%04X, want 0x1002", got)
	}
}

// --- memOps: PUSH(mem) / POP(mem) ---

func TestPUSH_Mem_Byte(t *testing.T) {
	// Source prefix: PUSH<W>(mem) - op2=0x04
	c, bus := setupMemOp(t, 0x80, 0x04) // byte (R0) indirect
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xAB)
	spBefore := c.reg.XSP
	c.Step()
	if c.reg.XSP != spBefore-1 {
		t.Errorf("SP after PUSH = 0x%04X, want 0x%04X", c.reg.XSP, spBefore-1)
	}
	got := bus.Read8(c.reg.XSP)
	if got != 0xAB {
		t.Errorf("pushed value = 0x%02X, want 0xAB", got)
	}
}

func TestPUSH_Mem_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x04) // word (R0) indirect
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1234)
	spBefore := c.reg.XSP
	c.Step()
	if c.reg.XSP != spBefore-2 {
		t.Errorf("SP after PUSH = 0x%04X, want 0x%04X", c.reg.XSP, spBefore-2)
	}
	got := bus.Read16(c.reg.XSP)
	if got != 0x1234 {
		t.Errorf("pushed value = 0x%04X, want 0x1234", got)
	}
}

func TestPUSH_Mem_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x04)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("PUSH<W>(mem) cycles = %d, want 6", cycles)
	}
}

func TestPOP_Mem_Byte(t *testing.T) {
	// Dst prefix: POP<B>(mem) - op2=0x04 via dst prefix
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Byte, 0x42)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x04) // POP<B>(mem)
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x42 {
		t.Errorf("POP<B>(mem) = 0x%02X, want 0x42", got)
	}
}

func TestPOP_Mem_Word(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Word, 0xBEEF)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x06) // POP<W>(mem)
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0xBEEF {
		t.Errorf("POP<W>(mem) = 0x%04X, want 0xBEEF", got)
	}
}

func TestPOP_Mem_Byte_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Byte, 0)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x04)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("POP<B>(mem) cycles = %d, want 7", cycles)
	}
}

func TestPOP_Mem_Word_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Word, 0)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x06)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("POP<W>(mem) cycles = %d, want 7", cycles)
	}
}

// --- memOps: LD<W>(mem),# and LD<W>(mem),(#16) ---

func TestLD_Mem_Imm_Byte(t *testing.T) {
	// LD<B> (mem),# via dst prefix
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x00) // LD<B>(mem),#
	bus.write8(0x1002, 0xAB) // imm8
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xAB {
		t.Errorf("LD<B>(mem),# = 0x%02X, want 0xAB", got)
	}
}

func TestLD_Mem_Imm_Word(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)      // dst indirect R0
	bus.write8(0x1001, 0x02)      // LD<W>(mem),#
	bus.write16LE(0x1002, 0x1234) // imm16
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0x1234 {
		t.Errorf("LD<W>(mem),# = 0x%04X, want 0x1234", got)
	}
}

func TestLD_Mem_Imm_Byte_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x00)
	bus.write8(0x1002, 0x00)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("LD<B>(mem),# cycles = %d, want 5", cycles)
	}
}

func TestLD_Mem_Imm_Word_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x02)
	bus.write16LE(0x1002, 0x0000)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("LD<W>(mem),# cycles = %d, want 6", cycles)
	}
}

func TestLD_Mem_FromAddr_Byte(t *testing.T) {
	// LD<B> (mem),(#16) via dst prefix
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)      // dst indirect R0
	bus.write8(0x1001, 0x14)      // LD<B>(mem),(#16)
	bus.write16LE(0x1002, 0x3000) // source addr = 0x3000
	bus.write8(0x3000, 0x42)      // source value
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x42 {
		t.Errorf("LD<B>(mem),(#16) = 0x%02X, want 0x42", got)
	}
}

func TestLD_Mem_FromAddr_Word(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x16) // LD<W>(mem),(#16)
	bus.write16LE(0x1002, 0x3000)
	bus.Write16(0x3000, 0xBEEF)
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0xBEEF {
		t.Errorf("LD<W>(mem),(#16) = 0x%04X, want 0xBEEF", got)
	}
}

func TestLD_Mem_FromAddr_Cycles(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x14)
	bus.write16LE(0x1002, 0x3000)
	bus.write8(0x3000, 0)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("LD<B>(mem),(#16) cycles = %d, want 8", cycles)
	}
}

// --- memOps: LD<W>(#16),(mem) ---

func TestLD_Addr_FromMem_Byte(t *testing.T) {
	// LD<W> (#16),(mem) via source prefix
	c, bus := setupMemOp(t, 0x80, 0x19, 0x00, 0x30) // byte (R0), LD (#16),(mem), addr=0x3000
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x42)
	c.Step()
	got := bus.Read8(0x3000)
	if got != 0x42 {
		t.Errorf("LD<W>(#16),(mem) = 0x%02X, want 0x42", got)
	}
}

func TestLD_Addr_FromMem_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x19, 0x00, 0x30) // word (R0), addr=0x3000
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1234)
	c.Step()
	got := bus.Read16(0x3000)
	if got != 0x1234 {
		t.Errorf("LD<W>(#16),(mem) = 0x%04X, want 0x1234", got)
	}
}

func TestLD_Addr_FromMem_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x19, 0x00, 0x30)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("LD<W>(#16),(mem) cycles = %d, want 8", cycles)
	}
}

// --- memOps: EX (mem),R ---

func TestEX_Mem_R_Byte(t *testing.T) {
	// Source prefix: EX (mem),R - op2=0x30+R
	// Use R2 (XDE) as pointer, R1 (A) as data register
	c, bus := setupMemOp(t, 0x82, 0x31) // byte (R2=XDE), EX (mem),R1(A)
	c.reg.WriteReg32(2, 0x2000)
	bus.write8(0x2000, 0x11)
	c.reg.WriteReg8(r8From3bit[1], 0x22) // A
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x11)
	got := bus.Read8(0x2000)
	if got != 0x22 {
		t.Errorf("(mem) = 0x%02X, want 0x22", got)
	}
}

func TestEX_Mem_R_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x31) // word (R0), EX (mem),R1
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1111)
	c.reg.WriteReg16(1, 0x2222)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x1111)
	got := bus.Read16(0x2000)
	if got != 0x2222 {
		t.Errorf("(mem) = 0x%04X, want 0x2222", got)
	}
}

func TestEX_Mem_R_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x82, 0x30) // byte (R2=XDE), EX (mem),R0
	c.reg.WriteReg32(2, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("EX (mem),R cycles = %d, want 6", cycles)
	}
}

// --- memOps: LDI / LDD / LDIR / LDDR ---

func TestLDI_Byte(t *testing.T) {
	// LDI via (XHL) prefix: 0x83 = byte (R3=XHL) indirect, op2=0x10
	c, bus := setupMemOp(t, 0x83, 0x10)
	c.reg.WriteReg32(3, 0x2000) // XHL = src
	c.reg.WriteReg32(2, 0x3000) // XDE = dst
	c.reg.WriteReg16(1, 3)      // BC = count
	bus.write8(0x2000, 0xAB)    // source data
	c.Step()

	// After LDI: XHL++, XDE++, BC--
	got := bus.Read8(0x3000)
	if got != 0xAB {
		t.Errorf("LDI dest = 0x%02X, want 0xAB", got)
	}
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2001)
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x3001)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)
	// V=1 because BC != 0
	checkFlags(t, c, -1, -1, 0, 1, 0, -1)
}

func TestLDI_BC_Reaches_Zero(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x10)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg32(2, 0x3000)
	c.reg.WriteReg16(1, 1) // BC = 1
	bus.write8(0x2000, 0x42)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
	// V=0 because BC == 0
	checkFlags(t, c, -1, -1, 0, 0, 0, -1)
}

func TestLDI_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x93, 0x10) // word prefix
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg32(2, 0x3000)
	c.reg.WriteReg16(1, 4) // BC = 4 (decremented by 1)
	bus.Write16(0x2000, 0x1234)
	c.Step()
	got := bus.Read16(0x3000)
	if got != 0x1234 {
		t.Errorf("LDI dest = 0x%04X, want 0x1234", got)
	}
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2002) // +2 for word
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x3002)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 3) // 4-1
}

func TestLDD_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x12) // LDD
	c.reg.WriteReg32(3, 0x2005)
	c.reg.WriteReg32(2, 0x3005)
	c.reg.WriteReg16(1, 3)
	bus.write8(0x2005, 0xCD)
	c.Step()
	got := bus.Read8(0x3005)
	if got != 0xCD {
		t.Errorf("LDD dest = 0x%02X, want 0xCD", got)
	}
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2004) // decremented
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x3004)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)
}

func TestLDI_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x10)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg32(2, 0x3000)
	c.reg.WriteReg16(1, 1)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("LDI cycles = %d, want 8", cycles)
	}
}

func TestLDIR_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x11) // LDIR
	c.reg.WriteReg32(3, 0x2000)         // XHL = src
	c.reg.WriteReg32(2, 0x3000)         // XDE = dst
	c.reg.WriteReg16(1, 3)              // BC = 3
	bus.write8(0x2000, 0x11)
	bus.write8(0x2001, 0x22)
	bus.write8(0x2002, 0x33)
	c.Step()

	// All 3 bytes should be copied
	if bus.Read8(0x3000) != 0x11 {
		t.Errorf("LDIR [0] = 0x%02X, want 0x11", bus.Read8(0x3000))
	}
	if bus.Read8(0x3001) != 0x22 {
		t.Errorf("LDIR [1] = 0x%02X, want 0x22", bus.Read8(0x3001))
	}
	if bus.Read8(0x3002) != 0x33 {
		t.Errorf("LDIR [2] = 0x%02X, want 0x33", bus.Read8(0x3002))
	}
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2003)
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x3003)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
	// V=0 at end
	checkFlags(t, c, -1, -1, 0, 0, 0, -1)
}

func TestLDIR_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x11)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg32(2, 0x3000)
	c.reg.WriteReg16(1, 4) // BC = 4
	bus.write8(0x2000, 0)
	cycles := c.Step()
	// 7*4 + 1 = 29
	if cycles != 29 {
		t.Errorf("LDIR cycles = %d, want 29", cycles)
	}
}

func TestLDDR_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x13) // LDDR
	c.reg.WriteReg32(3, 0x2002)         // XHL = src (end)
	c.reg.WriteReg32(2, 0x3002)         // XDE = dst (end)
	c.reg.WriteReg16(1, 3)              // BC = 3
	bus.write8(0x2002, 0x33)
	bus.write8(0x2001, 0x22)
	bus.write8(0x2000, 0x11)
	c.Step()

	if bus.Read8(0x3002) != 0x33 {
		t.Errorf("LDDR [2] = 0x%02X, want 0x33", bus.Read8(0x3002))
	}
	if bus.Read8(0x3001) != 0x22 {
		t.Errorf("LDDR [1] = 0x%02X, want 0x22", bus.Read8(0x3001))
	}
	if bus.Read8(0x3000) != 0x11 {
		t.Errorf("LDDR [0] = 0x%02X, want 0x11", bus.Read8(0x3000))
	}
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x1FFF) // decremented past start
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x2FFF)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
}

func TestLDDR_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x13)
	c.reg.WriteReg32(3, 0x2002)
	c.reg.WriteReg32(2, 0x3002)
	c.reg.WriteReg16(1, 3)
	bus.write8(0x2002, 0)
	cycles := c.Step()
	// 7*3 + 1 = 22
	if cycles != 22 {
		t.Errorf("LDDR cycles = %d, want 22", cycles)
	}
}

func TestLDIR_BC_Zero(t *testing.T) {
	// BC=0 should mean 65536 transfers per databook do-while semantics.
	// We test with a small write to confirm at least one transfer happens
	// and BC wraps to 0 after 65536 decrements.
	c, bus := setupMemOp(t, 0x83, 0x11) // LDIR byte
	c.reg.WriteReg32(3, 0x2000)         // XHL = src
	c.reg.WriteReg32(2, 0x3000)         // XDE = dst
	c.reg.WriteReg16(1, 0)              // BC = 0 -> 65536 transfers
	bus.write8(0x2000, 0xAA)
	c.Step()

	// First byte should be copied
	if bus.Read8(0x3000) != 0xAA {
		t.Errorf("LDIR BC=0 [0] = 0x%02X, want 0xAA", bus.Read8(0x3000))
	}
	// BC should be 0 after 65536 decrements
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
	// XHL should advance by 65536
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2000+0x10000)
	checkReg32(t, "XDE", c.reg.ReadReg32(2), 0x3000+0x10000)
	// 7*65536 + 1 = 458753
	if c.cycles != 458753 {
		t.Errorf("LDIR BC=0 cycles = %d, want 458753", c.cycles)
	}
	checkFlags(t, c, -1, -1, 0, 0, 0, -1)
}

func TestLDDR_BC_Zero(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x13) // LDDR byte
	c.reg.WriteReg32(3, 0x2000)         // XHL = src
	c.reg.WriteReg32(2, 0x3000)         // XDE = dst
	c.reg.WriteReg16(1, 0)              // BC = 0 -> 65536 transfers
	bus.write8(0x2000, 0xBB)
	c.Step()

	if bus.Read8(0x3000) != 0xBB {
		t.Errorf("LDDR BC=0 [0] = 0x%02X, want 0xBB", bus.Read8(0x3000))
	}
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
	// 7*65536 + 1 = 458753
	if c.cycles != 458753 {
		t.Errorf("LDDR BC=0 cycles = %d, want 458753", c.cycles)
	}
	checkFlags(t, c, -1, -1, 0, 0, 0, -1)
}

// --- LDX ---

func TestLDX_Basic(t *testing.T) {
	// LDX (#8),# : F7:00:addr:00:val:00
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xF7)
	bus.write8(0x1001, 0x00)
	bus.write8(0x1002, 0x50) // addr = 0x50
	bus.write8(0x1003, 0x00)
	bus.write8(0x1004, 0xAB) // val = 0xAB
	bus.write8(0x1005, 0x00)
	c.Step()
	got := bus.Read8(0x50)
	if got != 0xAB {
		t.Errorf("LDX (#8),# = 0x%02X, want 0xAB", got)
	}
}

func TestLDX_Cycles(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xF7)
	bus.write8(0x1001, 0x00)
	bus.write8(0x1002, 0x50)
	bus.write8(0x1003, 0x00)
	bus.write8(0x1004, 0x00)
	bus.write8(0x1005, 0x00)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("LDX cycles = %d, want 8", cycles)
	}
}

func TestLDX_PC_Advances(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xF7)
	bus.write8(0x1001, 0x00)
	bus.write8(0x1002, 0x50)
	bus.write8(0x1003, 0x00)
	bus.write8(0x1004, 0x00)
	bus.write8(0x1005, 0x00)
	c.Step()
	if c.reg.PC != 0x1006 {
		t.Errorf("PC after LDX = 0x%04X, want 0x1006", c.reg.PC)
	}
}

func TestLDI_XIY_Variant(t *testing.T) {
	// Prefix 0x85 = byte (R5=XIY) indirect, op2=0x10 (LDI)
	c, bus := setupMemOp(t, 0x85, 0x10)
	c.reg.XIY = 0x2000     // src
	c.reg.XIX = 0x3000     // dst
	c.reg.WriteReg16(1, 2) // BC = 2
	bus.write8(0x2000, 0x77)
	c.Step()
	got := bus.Read8(0x3000)
	if got != 0x77 {
		t.Errorf("LDI XIY variant dest = 0x%02X, want 0x77", got)
	}
	if c.reg.XIY != 0x2001 {
		t.Errorf("XIY = 0x%08X, want 0x2001", c.reg.XIY)
	}
	if c.reg.XIX != 0x3001 {
		t.Errorf("XIX = 0x%08X, want 0x3001", c.reg.XIX)
	}
}

// TestDstRegIndirect_R_plus_r16 tests the (R+r16) addressing mode in the
// destination prefix (0xF3 07). This verifies that the index register is
// decoded using the full 8-bit register code (bank-aware), not the short
// 3-bit code.
//
// Encodes: LD.W (XIX+HL),IZ using dst prefix
//
//	F3 07 F0 EC 56
//	F3 = dst reg-indirect prefix
//	07 = (R+r16) sub-mode
//	F0 = base register code for XIX
//	EC = index register code for HL (current bank XHL low 16)
//	56 = LD.W store from IZ (dispatchDstMem opcode)
func TestDstRegIndirect_R_plus_r16(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Set XIX = 0x5000 (base address)
	c.reg.XIX = 0x00005000
	// Set HL = 0x0010 (index offset) in current bank XHL
	c.reg.Bank[0].XHL = 0x00000010
	// Set IZ = 0xBEEF (value to store)
	c.reg.XIZ = 0x0000BEEF

	// Write instruction: F3 07 F0 EC 56
	bus.write8(0x1000, 0xF3) // dst reg-indirect prefix
	bus.write8(0x1001, 0x07) // (R+r16) sub-mode
	bus.write8(0x1002, 0xF0) // base = XIX
	bus.write8(0x1003, 0xEC) // index = HL (current bank, code $EC)
	bus.write8(0x1004, 0x56) // LD.W store IZ

	c.Step()

	// Should store 0xBEEF at address 0x5000 + 0x0010 = 0x5010
	got := bus.Read16(0x5010)
	if got != 0xBEEF {
		t.Errorf("memory at $5010 = 0x%04X, want 0xBEEF", got)
	}
}

// TestDstRegIndirect_R_plus_r8 tests the (R+r8) addressing mode in the
// destination prefix (0xF3 03). Verifies full 8-bit register code decoding.
//
// Register code 0xE1 under full 8-bit decoding = W (high byte of current
// bank XWA). Under the incorrect 4-bit decoding it would map to C (low
// byte of XBC) since 0xE1 & 0x07 = 1, bit 3 = 0.
//
// Encodes: LD.B (XIX+W),0x42 using dst prefix
//
//	F3 03 F0 E1 48 42
//	F3 = dst reg-indirect prefix
//	03 = (R+r8) sub-mode
//	F0 = base register code for XIX
//	E1 = index register code for W (current bank XWA high byte)
//	48 42 = LD.B (mem),imm8
func TestDstRegIndirect_R_plus_r8(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Set XIX = 0x5000 (base address)
	c.reg.XIX = 0x00005000
	// Set W = 0x10 (high byte of XWA), A = 0x20 (low byte)
	c.reg.Bank[0].XWA = 0x00001020
	// Set C = 0x30 (low byte of XBC) - this is what the buggy
	// code would use as the index instead of W
	c.reg.Bank[0].XBC = 0x00000030

	// Write instruction: F3 03 F0 E1 00 42
	bus.write8(0x1000, 0xF3) // dst reg-indirect prefix
	bus.write8(0x1001, 0x03) // (R+r8) sub-mode
	bus.write8(0x1002, 0xF0) // base = XIX
	bus.write8(0x1003, 0xE1) // index = W (current bank XWA high byte)
	bus.write8(0x1004, 0x00) // LD.B (mem),imm8
	bus.write8(0x1005, 0x42) // immediate value

	c.Step()

	// With correct decoding: W = 0x10, addr = 0x5000 + 0x10 = 0x5010
	// With buggy decoding: C = 0x30, addr = 0x5000 + 0x30 = 0x5030
	got := bus.Read8(0x5010)
	if got != 0x42 {
		t.Errorf("memory at $5010 = 0x%02X, want 0x42 (correct index W=0x10)", got)
	}
	// Verify buggy address was NOT written
	gotBuggy := bus.Read8(0x5030)
	if gotBuggy == 0x42 {
		t.Errorf("memory at $5030 = 0x42 - using wrong register (C instead of W)")
	}
}

// TestExtPrefixByte_WritesCorrectReg tests that the C7 extended byte prefix
// uses the full 8-bit register code for read/write, not a 4-bit code.
//
// Code $E7 under full 8-bit decoding = B (high byte of current bank XBC).
// Under the incorrect 4-bit decoding it maps to SPL (low byte of XSP)
// since $E7 & 0x07 = 7 = XSP, bit 3 = 0 = low byte.
//
// Encodes: LD.B B,B (self-load from 3-bit source B to extended dest B)
//
//	C7 E7 9A
//	C7 = byte extended register prefix
//	E7 = full register code for B (current bank XBC high byte)
//	9A = LD r,R where R = 3-bit code 2 = B
func TestExtPrefixByte_WritesCorrectReg(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Set B = 0x42 (high byte of XBC), SP = 0x6C00
	c.reg.Bank[0].XBC = 0x00004200
	c.reg.XSP = 0x00006C00

	// Write instruction: C7 E7 9A
	bus.write8(0x1000, 0xC7) // byte extended register prefix
	bus.write8(0x1001, 0xE7) // full code = B (current bank XBC high byte)
	bus.write8(0x1002, 0x9A) // LD r,R (source = 3-bit code 2 = B)

	c.Step()

	// B should still be 0x42 (self-load)
	b := c.reg.ReadReg8(0x09) // 4-bit code for B
	if b != 0x42 {
		t.Errorf("B = 0x%02X, want 0x42", b)
	}
	// SP must NOT be modified
	if c.reg.XSP != 0x00006C00 {
		t.Errorf("XSP = 0x%08X, want 0x00006C00 (SP was corrupted)", c.reg.XSP)
	}
}

// TestExtPrefixWord_WritesCorrectReg tests that the D7 extended word prefix
// uses the full 8-bit register code with bit 1 selecting low/high word.
// Code $E2: bit 1 set -> high word of current bank XWA.
// This must NOT modify the low word (WA) or any other register.
func TestExtPrefixWord_WritesCorrectReg(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Set XWA = 0x00001234 (low word = 0x1234, high word = 0x0000)
	c.reg.Bank[0].XWA = 0x00001234
	// Set XDE = 0x0000ABCD (low word = 0xABCD)
	c.reg.Bank[0].XDE = 0x0000ABCD

	// Write instruction: D7 E2 03 BE EF
	// D7 = word extended register prefix
	// E2 = full code for high word of XWA (bit 1 set)
	// 03 = LD r,# (immediate)
	// BE EF = immediate $EFBE (little-endian)
	bus.write8(0x1000, 0xD7)
	bus.write8(0x1001, 0xE2)
	bus.write8(0x1002, 0x03) // LD r,#
	bus.write8(0x1003, 0xBE)
	bus.write8(0x1004, 0xEF)

	c.Step()

	// High word of XWA should be $EFBE, low word preserved
	if c.reg.Bank[0].XWA != 0xEFBE1234 {
		t.Errorf("XWA = 0x%08X, want 0xEFBE1234", c.reg.Bank[0].XWA)
	}
	// DE must NOT be modified (old 3-bit mapping would target DE)
	if c.reg.Bank[0].XDE != 0x0000ABCD {
		t.Errorf("XDE = 0x%08X, want 0x0000ABCD (was corrupted)", c.reg.Bank[0].XDE)
	}
}
