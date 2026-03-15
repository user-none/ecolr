package tlcs900h

import "testing"

func TestNOP(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00) // NOP

	sr := c.reg.SR
	cyc := c.Step()

	checkReg32(t, "PC", c.reg.PC, pc+1)
	checkReg16(t, "SR", c.reg.SR, sr)
	if cyc != 2 {
		t.Errorf("cycles = %d, want 2", cyc)
	}
}

func TestHALT(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x05) // HALT

	sr := c.reg.SR
	cyc := c.Step()

	if !c.Halted() {
		t.Error("CPU should be halted")
	}
	checkReg16(t, "SR", c.reg.SR, sr)
	if cyc != 6 {
		t.Errorf("cycles = %d, want 6", cyc)
	}
}

func TestHALTResumeOnInterrupt(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x05) // HALT

	// Set IFF to 0 to allow all interrupts
	sr := c.reg.SR
	sr &^= srIFFMask
	c.setSR(sr)

	c.Step() // execute HALT
	if !c.Halted() {
		t.Error("CPU should be halted after HALT")
	}

	// Request interrupt - vector 4, level 1
	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+4*4, handler)
	c.RequestInterrupt(1, 4)

	c.Step() // should service interrupt and resume
	if c.Halted() {
		t.Error("CPU should not be halted after interrupt")
	}
}

func TestInterrupt_AcceptedWhenLevelEqualsIFF(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00) // NOP (will be skipped by interrupt)

	// Set IFF to 5
	sr := c.reg.SR
	sr &^= srIFFMask
	sr |= 5 << srIFFShift
	c.setSR(sr)

	// Request interrupt at level 5 (equal to IFF)
	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+10*4, handler) // vector 10
	c.RequestInterrupt(5, 10)

	c.Step() // should accept interrupt since level >= IFF
	if c.Registers().PC != handler {
		t.Errorf("PC = %06X, want %06X; interrupt at level==IFF should be accepted", c.Registers().PC, handler)
	}
}

func TestInterrupt_RejectedWhenLevelBelowIFF(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00) // NOP

	// Set IFF to 5
	sr := c.reg.SR
	sr &^= srIFFMask
	sr |= 5 << srIFFShift
	c.setSR(sr)

	// Request interrupt at level 4 (below IFF)
	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+10*4, handler)
	c.RequestInterrupt(4, 10)

	c.Step() // should NOT accept interrupt
	if c.Registers().PC == handler {
		t.Error("interrupt at level < IFF should be rejected")
	}
}

func TestInterrupt_SetsIFFToLevelPlusOne(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00) // NOP

	// Set IFF to 0
	sr := c.reg.SR
	sr &^= srIFFMask
	c.setSR(sr)

	// Request interrupt at level 3
	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+4*4, handler)
	c.RequestInterrupt(3, 4)

	c.Step()
	// IFF should be set to level+1 = 4
	iff := c.iff()
	if iff != 4 {
		t.Errorf("IFF = %d, want 4 (level+1)", iff)
	}
}

func TestInterrupt_Level6_SetsIFFTo7(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00) // NOP

	sr := c.reg.SR
	sr &^= srIFFMask
	c.setSR(sr)

	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00+4*4, handler)
	c.RequestInterrupt(6, 4)

	c.Step()
	iff := c.iff()
	if iff != 7 {
		t.Errorf("IFF = %d, want 7 (level 6 + 1)", iff)
	}
}

func TestEI(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x06) // EI
	bus.write8(pc+1, 3)  // #3

	cyc := c.Step()

	if c.iff() != 3 {
		t.Errorf("IFF = %d, want 3", c.iff())
	}
	if cyc != 3 {
		t.Errorf("cycles = %d, want 3", cyc)
	}
}

func TestEIZero(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x06) // EI
	bus.write8(pc+1, 0)  // #0

	c.Step()

	if c.iff() != 0 {
		t.Errorf("IFF = %d, want 0", c.iff())
	}
}

func TestDI(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)

	// First set IFF to something other than 7
	sr := c.reg.SR
	sr &^= srIFFMask
	sr |= 3 << srIFFShift
	c.setSR(sr)

	bus.write8(pc, 0x06) // DI (EI #7)
	bus.write8(pc+1, 7)

	cyc := c.Step()

	if c.iff() != 7 {
		t.Errorf("IFF = %d, want 7", c.iff())
	}
	if cyc != 4 {
		t.Errorf("cycles = %d, want 4", cyc)
	}
}

func TestEIPreservesOtherSRBits(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)

	// Set some flags
	c.setFlags(flagS | flagZ | flagC)
	origSR := c.reg.SR

	bus.write8(pc, 0x06) // EI
	bus.write8(pc+1, 2)  // #2

	c.Step()

	// IFF should be 2, other bits unchanged
	wantSR := (origSR &^ srIFFMask) | (2 << srIFFShift) | srMAX
	if c.reg.SR != wantSR {
		t.Errorf("SR = 0x%04X, want 0x%04X", c.reg.SR, wantSR)
	}
	// Flags should be preserved
	if c.flags() != (flagS | flagZ | flagC) {
		t.Errorf("flags = 0x%02X, want 0x%02X", c.flags(), flagS|flagZ|flagC)
	}
}

func TestSWI0(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xF8) // SWI 0

	handler := uint32(0x2000)
	bus.write32LE(0xFFFF00, handler) // vector 0

	origSR := c.reg.SR
	origXSP := c.reg.XSP

	cyc := c.Step()

	// PC should be loaded from vector
	checkReg32(t, "PC", c.reg.PC, handler)

	// Stack should have SR at XSP, PC at XSP+2
	expectedXSP := origXSP - 6
	checkReg32(t, "XSP", c.reg.XSP, expectedXSP)

	storedSR := bus.Read16(expectedXSP)
	if storedSR != origSR {
		t.Errorf("stored SR = 0x%04X, want 0x%04X", storedSR, origSR)
	}

	// PC stored should be pc+1 (after fetching the opcode byte)
	storedPC := bus.Read32(expectedXSP + 2)
	if storedPC != pc+1 {
		t.Errorf("stored PC = 0x%06X, want 0x%06X", storedPC, pc+1)
	}

	if cyc != 19 {
		t.Errorf("cycles = %d, want 19", cyc)
	}
}

func TestSWI7(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xFF) // SWI 7

	handler := uint32(0x3000)
	bus.write32LE(0xFFFF00+7*4, handler) // vector 7

	c.Step()

	checkReg32(t, "PC", c.reg.PC, handler)
}

func TestSWIStackLayout(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xFA) // SWI 2

	handler := uint32(0x4000)
	bus.write32LE(0xFFFF00+2*4, handler)

	// Set distinctive SR value
	c.setFlags(flagS | flagC)
	origSR := c.reg.SR
	origXSP := c.reg.XSP

	c.Step()

	xsp := origXSP - 6

	// Verify layout: SR(word) at XSP, PC(long) at XSP+2
	gotSR := bus.Read16(xsp)
	gotPC := bus.Read32(xsp + 2)

	if gotSR != origSR {
		t.Errorf("SR on stack = 0x%04X, want 0x%04X", gotSR, origSR)
	}
	if gotPC != pc+1 {
		t.Errorf("PC on stack = 0x%06X, want 0x%06X", gotPC, pc+1)
	}
}

func TestMIRR_0001_to_8000(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x16) // word reg0
	c.reg.WriteReg16(0, 0x0001)

	sr := c.reg.SR
	cyc := c.Step()

	got := c.reg.ReadReg16(0)
	if got != 0x8000 {
		t.Errorf("MIRR 0x0001 = 0x%04X, want 0x8000", got)
	}
	checkReg16(t, "SR", c.reg.SR, sr)
	if cyc != 3 {
		t.Errorf("cycles = %d, want 3", cyc)
	}
}

func TestMIRR_8000_to_0001(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x16)
	c.reg.WriteReg16(0, 0x8000)

	c.Step()

	got := c.reg.ReadReg16(0)
	if got != 0x0001 {
		t.Errorf("MIRR 0x8000 = 0x%04X, want 0x0001", got)
	}
}

func TestMIRR_A5A5(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x16)
	c.reg.WriteReg16(0, 0xA5A5)

	c.Step()

	got := c.reg.ReadReg16(0)
	if got != 0xA5A5 {
		t.Errorf("MIRR 0xA5A5 = 0x%04X, want 0xA5A5", got)
	}
}

func TestMIRRFlagsUnchanged(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x16)
	c.reg.WriteReg16(0, 0x1234)
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)

	c.Step()

	checkFlags(t, c, 1, 1, 1, 1, 1, 1)
}

func TestNOPFlagsUnchanged(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x00)
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)

	c.Step()

	checkFlags(t, c, 1, 1, 1, 1, 1, 1)
}

func TestHALTFlagsUnchanged(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0x05)
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)

	c.Step()

	checkFlags(t, c, 1, 1, 1, 1, 1, 1)
}

func TestSWIFlagsUnchanged(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xF8) // SWI 0
	bus.write32LE(0xFFFF00, 0x2000)
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)

	c.Step()

	checkFlags(t, c, 1, 1, 1, 1, 1, 1)
}
