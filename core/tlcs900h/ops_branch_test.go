package tlcs900h

import "testing"

// --- JP tests ---

func TestJP16(t *testing.T) {
	// JP #16: opcode 0x1A followed by 16-bit LE address
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1A)
	bus.write16LE(0x1001, 0x8000) // target = 0x008000
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x008000)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJP24(t *testing.T) {
	// JP #24: opcode 0x1B followed by 24-bit LE address
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1B)
	bus.write8(0x1001, 0x56) // low
	bus.write8(0x1002, 0x34) // mid
	bus.write8(0x1003, 0x12) // high
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x123456)
	if cycles != 6 {
		t.Errorf("cycles = %d, want 6", cycles)
	}
}

func TestJP_CC_Mem_True(t *testing.T) {
	// JP T,mem - condition always true (cc=0x8)
	// Use (R0) indirect prefix: 0xB0 then 0xD8 (cc=T)
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0)      // dst indirect via R0
	bus.write8(0x1001, 0xD8)      // JP cc=T(0x8)
	c.reg.WriteReg32(0, 0x005000) // XWA = target address
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	if cycles != 7 {
		t.Errorf("cycles = %d, want 7", cycles)
	}
}

func TestJP_CC_Mem_False(t *testing.T) {
	// JP F,mem - condition always false (cc=0x0)
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0) // dst indirect via R0
	bus.write8(0x1001, 0xD0) // JP cc=F(0x0)
	c.reg.WriteReg32(0, 0x005000)
	cycles := c.Step()
	// PC should be past instruction (0x1002), not the target
	checkReg32(t, "PC", c.reg.PC, 0x1002)
	if cycles != 4 {
		t.Errorf("cycles = %d, want 4", cycles)
	}
}

// --- JR tests ---

func TestJR_True_Positive(t *testing.T) {
	// JR T,$+2+d8: opcode 0x68 (cc=T=0x8), d8=0x10
	// PC starts at 0x1000, after fetch of opcode+d8: PC=0x1002
	// Target = 0x1002 + 0x10 = 0x1012
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x68) // JR T
	bus.write8(0x1001, 0x10) // d8 = +16
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1012)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJR_True_Negative(t *testing.T) {
	// JR T with negative displacement
	// PC=0x1000, d8=-16 (0xF0), after fetch PC=0x1002
	// Target = 0x1002 + (-16) = 0x0FF2
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x68) // JR T
	bus.write8(0x1001, 0xF0) // d8 = -16
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x0FF2)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJR_False(t *testing.T) {
	// JR F (cc=0x0): never branches
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x60) // JR F
	bus.write8(0x1001, 0x10) // d8 = +16 (ignored)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1002)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

func TestJR_EQ_True(t *testing.T) {
	// JR EQ: branches when Z flag is set
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x66) // JR EQ (cc=0x6)
	bus.write8(0x1001, 0x20) // d8 = +32
	c.setFlag(flagZ, true)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1022)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJR_NE_True(t *testing.T) {
	// JR NE: branches when Z flag is clear
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x6E) // JR NE (cc=0xE)
	bus.write8(0x1001, 0x20) // d8 = +32
	// Z flag is clear by default
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1022)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

// --- JRL tests ---

func TestJRL_True(t *testing.T) {
	// JRL T,$+3+d16: opcode 0x78 (cc=T=0x8), d16=0x0100
	// After fetch: PC = 0x1003, target = 0x1003 + 0x0100 = 0x1103
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x78)      // JRL T
	bus.write16LE(0x1001, 0x0100) // d16 = +256
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1103)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJRL_True_Negative(t *testing.T) {
	// JRL T with negative displacement
	// After fetch: PC = 0x1003, d16 = -256 (0xFF00)
	// Target = 0x1003 + (-256) = 0x0F03
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x78)      // JRL T
	bus.write16LE(0x1001, 0xFF00) // d16 = -256 as int16
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x0F03)
	if cycles != 5 {
		t.Errorf("cycles = %d, want 5", cycles)
	}
}

func TestJRL_False(t *testing.T) {
	// JRL F: never branches
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x70)      // JRL F (cc=0x0)
	bus.write16LE(0x1001, 0x0100) // d16 (ignored)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1003)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

// --- CALL tests ---

func TestCALL16(t *testing.T) {
	// CALL #16: opcode 0x1C followed by 16-bit target
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1C)
	bus.write16LE(0x1001, 0x8000) // target = 0x008000
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x008000)
	// Return address (0x1003) should be pushed on stack
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	retAddr := bus.Read(Long, c.reg.XSP)
	checkReg32(t, "return addr", retAddr, 0x1003)
	if cycles != 9 {
		t.Errorf("cycles = %d, want 9", cycles)
	}
}

func TestCALL24(t *testing.T) {
	// CALL #24: opcode 0x1D followed by 24-bit target
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1D)
	bus.write8(0x1001, 0x56) // low
	bus.write8(0x1002, 0x34) // mid
	bus.write8(0x1003, 0x12) // high
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x123456)
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	retAddr := bus.Read(Long, c.reg.XSP)
	checkReg32(t, "return addr", retAddr, 0x1004)
	if cycles != 10 {
		t.Errorf("cycles = %d, want 10", cycles)
	}
}

func TestCALL_CC_Mem_True(t *testing.T) {
	// CALL T,mem: cc=T (0x8), always true
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0) // dst indirect via R0
	bus.write8(0x1001, 0xE8) // CALL cc=T(0x8)
	c.reg.WriteReg32(0, 0x005000)
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	retAddr := bus.Read(Long, c.reg.XSP)
	checkReg32(t, "return addr", retAddr, 0x1002)
	if cycles != 12 {
		t.Errorf("cycles = %d, want 12", cycles)
	}
}

func TestCALL_CC_Mem_False(t *testing.T) {
	// CALL F,mem: cc=F (0x0), never true
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0) // dst indirect via R0
	bus.write8(0x1001, 0xE0) // CALL cc=F(0x0)
	c.reg.WriteReg32(0, 0x005000)
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1002)
	checkReg32(t, "XSP", c.reg.XSP, origSP) // SP unchanged
	if cycles != 4 {
		t.Errorf("cycles = %d, want 4", cycles)
	}
}

// --- CALR tests ---

func TestCALR(t *testing.T) {
	// CALR $+3+d16: opcode 0x1E, d16=0x0100
	// After fetch: PC = 0x1003, push 0x1003, then PC = 0x1003 + 0x0100 = 0x1103
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1E)
	bus.write16LE(0x1001, 0x0100)
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1103)
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	retAddr := bus.Read(Long, c.reg.XSP)
	checkReg32(t, "return addr", retAddr, 0x1003)
	if cycles != 10 {
		t.Errorf("cycles = %d, want 10", cycles)
	}
}

func TestCALR_Negative(t *testing.T) {
	// CALR with negative displacement
	// After fetch: PC = 0x1003, d16 = -256 (0xFF00)
	// Push 0x1003, target = 0x1003 + (-256) = 0x0F03
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x1E)
	bus.write16LE(0x1001, 0xFF00)
	origSP := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x0F03)
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	retAddr := bus.Read(Long, c.reg.XSP)
	checkReg32(t, "return addr", retAddr, 0x1003)
	if cycles != 10 {
		t.Errorf("cycles = %d, want 10", cycles)
	}
}

// --- RET tests ---

func TestRET(t *testing.T) {
	// RET: opcode 0x0E, pops PC from stack
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x0E)
	// Push a return address onto the stack
	c.push(Long, 0x005000)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	if cycles != 9 {
		t.Errorf("cycles = %d, want 9", cycles)
	}
}

func TestRET_CC_True(t *testing.T) {
	// RET T: cc=T (0x8), always returns
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0)      // dst indirect via R0
	bus.write8(0x1001, 0xF8)      // RET cc=T(0x8)
	c.reg.WriteReg32(0, 0x002000) // opAddr (ignored for RET)
	c.push(Long, 0x005000)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	if cycles != 12 {
		t.Errorf("cycles = %d, want 12", cycles)
	}
}

func TestRET_CC_False(t *testing.T) {
	// RET F: cc=F (0x0), never returns
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0xB0) // dst indirect via R0
	bus.write8(0x1001, 0xF0) // RET cc=F(0x0)
	c.reg.WriteReg32(0, 0x002000)
	origSP := c.reg.XSP
	c.push(Long, 0x005000)
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x1002)
	// SP should remain where it was after the push (not popped)
	checkReg32(t, "XSP", c.reg.XSP, origSP-4)
	if cycles != 4 {
		t.Errorf("cycles = %d, want 4", cycles)
	}
}

// --- RETD tests ---

func TestRETD(t *testing.T) {
	// RETD d16: opcode 0x0F, pops PC then XSP += d16
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x0F)
	bus.write16LE(0x1001, 0x0008) // d16 = +8
	c.push(Long, 0x005000)
	spAfterPush := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	// After pop, SP is at spAfterPush+4, then +8 from d16
	checkReg32(t, "XSP", c.reg.XSP, spAfterPush+4+8)
	if cycles != 11 {
		t.Errorf("cycles = %d, want 11", cycles)
	}
}

func TestRETD_Negative(t *testing.T) {
	// RETD with negative d16
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x0F)
	bus.write16LE(0x1001, 0xFFF8) // d16 = -8
	c.push(Long, 0x005000)
	spAfterPush := c.reg.XSP
	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	// After pop, SP is at spAfterPush+4, then -8 from d16
	checkReg32(t, "XSP", c.reg.XSP, spAfterPush+4-8)
	if cycles != 11 {
		t.Errorf("cycles = %d, want 11", cycles)
	}
}

// --- RETI tests ---

func TestRETI(t *testing.T) {
	// RETI: opcode 0x07, restores SR and PC from stack
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x07)

	// Set up stack frame as processInterrupt would push:
	// SR at XSP (word), PC at XSP+2 (long)
	origSP := c.reg.XSP
	// Write SR value with some flags set (Z and C)
	sr := srMAX | srSYSM | (5 << srIFFShift) | uint16(flagZ|flagC)
	bus.Write(Word, origSP-6, uint32(sr))
	bus.Write(Long, origSP-4, 0x005000)
	c.reg.XSP = origSP - 6

	cycles := c.Step()
	checkReg32(t, "PC", c.reg.PC, 0x005000)
	checkReg32(t, "XSP", c.reg.XSP, origSP)
	// Verify flags restored from SR
	checkFlags(t, c, 0, 1, 0, 0, 0, 1) // Z=1, C=1
	// Verify IFF restored
	if c.iff() != 5 {
		t.Errorf("IFF = %d, want 5", c.iff())
	}
	if cycles != 12 {
		t.Errorf("cycles = %d, want 12", cycles)
	}
}

func TestRETI_DecrementsIntNest(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x07) // RETI

	// Simulate interrupt nesting depth of 2
	c.intNest = 2

	origSP := c.reg.XSP
	sr := srMAX | srSYSM | (3 << srIFFShift)
	bus.Write(Word, origSP-6, uint32(sr))
	bus.Write(Long, origSP-4, 0x005000)
	c.reg.XSP = origSP - 6

	c.Step()
	if c.intNest != 1 {
		t.Errorf("intNest = %d, want 1", c.intNest)
	}
}

func TestRETI_IntNestNoUnderflow(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x07) // RETI

	// intNest already 0 (e.g. mismatched RETI)
	c.intNest = 0

	origSP := c.reg.XSP
	sr := srMAX | srSYSM
	bus.Write(Word, origSP-6, uint32(sr))
	bus.Write(Long, origSP-4, 0x005000)
	c.reg.XSP = origSP - 6

	c.Step()
	if c.intNest != 0 {
		t.Errorf("intNest = %d, want 0 (should not underflow)", c.intNest)
	}
}

func TestInterrupt_IncrementsIntNest(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x00) // NOP at handler

	// Set IFF to 0 to allow all interrupts
	sr := c.reg.SR
	sr &^= srIFFMask
	c.setSR(sr)

	if c.intNest != 0 {
		t.Fatalf("intNest = %d, want 0 initially", c.intNest)
	}

	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+4*4, handler)
	c.RequestInterrupt(1, 4)

	c.Step() // services interrupt
	if c.intNest != 1 {
		t.Errorf("intNest = %d, want 1 after interrupt", c.intNest)
	}
}

func TestInterrupt_RETI_RoundTrip_IntNest(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x00) // NOP (will be skipped by interrupt)

	// Set IFF to 0 to allow all interrupts
	sr := c.reg.SR
	sr &^= srIFFMask
	c.setSR(sr)

	// Set up handler with RETI
	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+4*4, handler)
	bus.write8(0x2000, 0x07) // RETI

	c.RequestInterrupt(1, 4)
	c.Step() // services interrupt, jumps to handler
	if c.intNest != 1 {
		t.Fatalf("intNest = %d, want 1 after interrupt", c.intNest)
	}

	c.Step() // executes RETI
	if c.intNest != 0 {
		t.Errorf("intNest = %d, want 0 after RETI", c.intNest)
	}
}

// --- DJNZ tests ---

func TestDJNZ_Byte_NotZero(t *testing.T) {
	// DJNZ r,$+3+d8: prefix C8+r, op2=0x1C, d8
	// Using byte reg 0 (W), value=5, decrement to 4, branch
	c, _ := setupRegOp(t, 0xC8, 0x1C, 0x10) // prefix=byte reg0, op=DJNZ, d8=+16
	c.reg.WriteReg8(r8From3bit[0], 5)       // W = 5
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 4)
	// After fetch: PC = 0x1003, target = 0x1003 + 16 = 0x1013
	checkReg32(t, "PC", c.reg.PC, 0x1013)
	if cycles != 6 {
		t.Errorf("cycles = %d, want 6", cycles)
	}
}

func TestDJNZ_Byte_Zero(t *testing.T) {
	// DJNZ with register=1: decrements to 0, no branch
	c, _ := setupRegOp(t, 0xC8, 0x1C, 0x10) // d8=+16
	c.reg.WriteReg8(r8From3bit[0], 1)       // W = 1
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0)
	// PC should be past instruction (no branch)
	checkReg32(t, "PC", c.reg.PC, 0x1003)
	if cycles != 4 {
		t.Errorf("cycles = %d, want 4", cycles)
	}
}

func TestDJNZ_Word(t *testing.T) {
	// DJNZ with word register: prefix D8+r, op2=0x1C
	c, _ := setupRegOp(t, 0xD8, 0x1C, 0x10) // word reg0 (WA), d8=+16
	c.reg.WriteReg16(0, 0x0002)             // WA = 2
	cycles := c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0001)
	checkReg32(t, "PC", c.reg.PC, 0x1013)
	if cycles != 6 {
		t.Errorf("cycles = %d, want 6", cycles)
	}
}

func TestDJNZ_Byte_Wrap(t *testing.T) {
	// DJNZ with register=0: wraps to 0xFF (byte), branches
	c, _ := setupRegOp(t, 0xC8, 0x1C, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0) // W = 0
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xFF)
	// Should branch since 0xFF != 0
	checkReg32(t, "PC", c.reg.PC, 0x1013)
	if cycles != 6 {
		t.Errorf("cycles = %d, want 6", cycles)
	}
}

// --- SCC tests ---

func TestSCC_True(t *testing.T) {
	// SCC T,r: cc=T (0x8), always sets to 1
	c, _ := setupRegOp(t, 0xC8, 0x78) // byte reg0, SCC cc=T
	c.reg.WriteReg8(r8From3bit[0], 0) // W = 0 initially
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 1)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

func TestSCC_False(t *testing.T) {
	// SCC F,r: cc=F (0x0), always sets to 0
	c, _ := setupRegOp(t, 0xC8, 0x70) // byte reg0, SCC cc=F
	c.reg.WriteReg8(r8From3bit[0], 0xFF)
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

func TestSCC_EQ_True(t *testing.T) {
	// SCC EQ,r: sets to 1 when Z flag is set
	c, _ := setupRegOp(t, 0xC8, 0x76) // byte reg0, SCC cc=EQ(0x6)
	c.setFlag(flagZ, true)
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 1)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

func TestSCC_EQ_False(t *testing.T) {
	// SCC EQ,r: sets to 0 when Z flag is clear
	c, _ := setupRegOp(t, 0xC8, 0x76) // byte reg0, SCC cc=EQ(0x6)
	// Z flag clear by default
	cycles := c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

func TestSCC_Word(t *testing.T) {
	// SCC T with word register
	c, _ := setupRegOp(t, 0xD8, 0x78) // word reg0, SCC cc=T
	c.reg.WriteReg16(0, 0xFFFF)
	cycles := c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 1)
	if cycles != 2 {
		t.Errorf("cycles = %d, want 2", cycles)
	}
}

// --- CALL then RET round-trip ---

func TestCALL_RET_RoundTrip(t *testing.T) {
	// CALL #16 to subroutine, then RET back
	c, bus := newTestCPU(t, 0x1000)
	// CALL #16 to 0x2000
	bus.write8(0x1000, 0x1C)
	bus.write16LE(0x1001, 0x2000)
	// At 0x2000: RET
	bus.write8(0x2000, 0x0E)

	c.Step() // CALL
	checkReg32(t, "PC after CALL", c.reg.PC, 0x2000)

	c.Step() // RET
	checkReg32(t, "PC after RET", c.reg.PC, 0x1003)
}

// --- Flags preservation ---

func TestBranch_FlagsUnchanged(t *testing.T) {
	// Branch instructions should not modify flags
	c, bus := newTestCPU(t, 0x1000)
	// Set all flags
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)
	bus.write8(0x1000, 0x68) // JR T
	bus.write8(0x1001, 0x10) // d8 = +16
	c.Step()
	checkFlags(t, c, 1, 1, 1, 1, 1, 1) // all flags preserved
}
