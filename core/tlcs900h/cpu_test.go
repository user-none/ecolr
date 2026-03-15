package tlcs900h

import "testing"

func TestNew(t *testing.T) {
	bus := &testBus{}
	// Place a known PC value in the reset vector
	bus.write32LE(0xFFFF00, 0x00200000)

	c := New(bus)
	c.LoadResetVector()
	regs := c.Registers()

	checkReg32(t, "PC", regs.PC, 0x200000)
	checkReg32(t, "XSP", regs.XSP, 0x6C00)

	// SR should have MAX, SYSM set, IFF=7
	wantSR := srMAX | srSYSM | (7 << srIFFShift)
	checkReg16(t, "SR", regs.SR, wantSR)

	if c.Halted() {
		t.Error("CPU should not be halted after reset")
	}
	if c.Cycles() != 0 {
		t.Errorf("Cycles = %d, want 0", c.Cycles())
	}
}

func TestReset(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Dirty up some state
	c.reg.Bank[0].XWA = 0xDEADBEEF
	c.halted = true
	c.cycles = 999

	// Change reset vector for re-reset
	bus.write32LE(0xFFFF00, 0x2000)
	c.Reset()

	regs := c.Registers()
	checkReg32(t, "PC after reset", regs.PC, 0x2000)
	checkReg32(t, "XWA after reset", regs.Bank[0].XWA, 0)

	if c.Halted() {
		t.Error("CPU should not be halted after reset")
	}
}

func TestStepUnhandledOpcode(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	// Memory is zeroed, so opcode at 0x1000 is 0x00 (unhandled = NOP)
	cycles := c.Step()

	regs := c.Registers()
	// PC should advance by 1 (fetched one byte)
	checkReg32(t, "PC", regs.PC, 0x1001)

	if cycles < 1 {
		t.Errorf("Step should consume at least 1 cycle, got %d", cycles)
	}
}

func TestStepCyclesDeficit(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	// Step with budget of 1; NOP consumes 2 cycles so deficit is 1
	used := c.StepCycles(1)
	if used != 1 {
		t.Errorf("StepCycles(1) = %d, want 1", used)
	}
	if c.Deficit() != 1 {
		t.Errorf("Deficit = %d, want 1", c.Deficit())
	}
}

func TestAddCycles(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.AddCycles(100)
	if c.Cycles() != 100 {
		t.Errorf("Cycles = %d, want 100", c.Cycles())
	}
}

func TestSetState(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	want := Registers{
		PC:  0x4000,
		XSP: 0x8000,
		SR:  srMAX | srSYSM,
	}
	want.Bank[0].XWA = 0x12345678

	c.SetState(want)
	got := c.Registers()

	checkReg32(t, "PC", got.PC, want.PC)
	checkReg32(t, "XSP", got.XSP, want.XSP)
	checkReg16(t, "SR", got.SR, want.SR)
	checkReg32(t, "Bank[0].XWA", got.Bank[0].XWA, 0x12345678)
}

func TestRegisterOp(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Use opcode 0xFE (unhandled by default)
	bus.write8(0x1000, 0xFE)
	// Place a NOP-like unhandled byte after for the next step
	bus.write8(0x1001, 0x00)

	called := false
	c.RegisterOp(0xFE, func(cpu *CPU) {
		called = true
		cpu.AddCycles(3)
	})

	cycles := c.Step()
	if !called {
		t.Error("RegisterOp handler was not called")
	}
	if cycles != 3 {
		t.Errorf("Step cycles = %d, want 3", cycles)
	}

	regs := c.Registers()
	// PC should have advanced past the opcode byte (fetchPC in execute)
	checkReg32(t, "PC", regs.PC, 0x1001)

	// Next step should use default decode (opcode 0x00 = unhandled = NOP)
	called = false
	c.Step()
	if called {
		t.Error("handler should not be called for opcode 0x00")
	}
}

func TestRegisterOpOverridesBaseOps(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Pick an opcode that has a baseOps handler - 0x00 is unhandled,
	// so use a prefix like 0x06 (NOP-ish slot) that is also unhandled.
	// Actually let's override 0x00 which is currently unhandled/NOP.
	bus.write8(0x1000, 0x00)

	called := false
	c.RegisterOp(0x00, func(cpu *CPU) {
		called = true
		cpu.AddCycles(5)
	})

	cycles := c.Step()
	if !called {
		t.Error("custom handler for 0x00 was not called")
	}
	if cycles != 5 {
		t.Errorf("Step cycles = %d, want 5", cycles)
	}
}

func TestReadBank3W(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	// W is bits 15-8 of XWA
	c.reg.Bank[3].XWA = 0x00001200
	got := c.ReadBank3W()
	if got != 0x0012 {
		t.Errorf("ReadBank3W() = 0x%04X, want 0x0012", got)
	}

	c.reg.Bank[3].XWA = 0xFFFF1AFF
	got = c.ReadBank3W()
	if got != 0x001A {
		t.Errorf("ReadBank3W() = 0x%04X, want 0x001A", got)
	}
}

func TestBank3ReadHelpers(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	c.reg.Bank[3].XWA = 0xAABBCCDD
	c.reg.Bank[3].XBC = 0x11223344
	c.reg.Bank[3].XDE = 0x55667788
	c.reg.Bank[3].XHL = 0x99AABBCC

	checkReg8(t, "ReadBank3RA", c.ReadBank3RA(), 0xDD)
	checkReg8(t, "ReadBank3RB", c.ReadBank3RB(), 0x33)
	checkReg8(t, "ReadBank3RC", c.ReadBank3RC(), 0x44)
	checkReg8(t, "ReadBank3RD", c.ReadBank3RD(), 0x77)
	checkReg16(t, "ReadBank3BC", c.ReadBank3BC(), 0x3344)
	checkReg32(t, "ReadBank3XDE", c.ReadBank3XDE(), 0x55667788)
	checkReg32(t, "ReadBank3XHL", c.ReadBank3XHL(), 0x99AABBCC)
}

func TestBank3WriteHelpers(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	c.reg.Bank[3].XWA = 0xAABBCCDD

	c.WriteBank3RA(0x42)
	checkReg32(t, "XWA after WriteBank3RA", c.reg.Bank[3].XWA, 0xAABBCC42)

	c.reg.Bank[3].XWA = 0xAABBCCDD
	c.WriteBank3RWA(0x1234)
	checkReg32(t, "XWA after WriteBank3RWA", c.reg.Bank[3].XWA, 0xAABB1234)
}

func TestRETIPublicMethod(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	// Set up a stack frame as processInterrupt would push:
	// SR at XSP (word), PC at XSP+2 (long)
	c.reg.XSP = 0x7000
	wantSR := srMAX | srSYSM | (5 << srIFFShift) | uint16(flagZ|flagC)
	wantPC := uint32(0x123456)
	bus.write16LE(0x7000, wantSR)
	bus.write32LE(0x7002, wantPC)

	c.intNest = 2
	before := c.Cycles()
	c.RETI()
	after := c.Cycles()

	regs := c.Registers()
	checkReg32(t, "PC", regs.PC, wantPC)
	checkReg32(t, "XSP", regs.XSP, 0x7006)
	checkReg16(t, "SR", regs.SR, wantSR)

	if c.intNest != 1 {
		t.Errorf("intNest = %d, want 1", c.intNest)
	}

	if after-before != 12 {
		t.Errorf("RETI cycles = %d, want 12", after-before)
	}
}

func TestRETIPublicMethodNoUnderflow(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	c.reg.XSP = 0x7000
	bus.write16LE(0x7000, srMAX|srSYSM)
	bus.write32LE(0x7002, 0x2000)

	c.intNest = 0
	c.RETI()

	if c.intNest != 0 {
		t.Errorf("intNest = %d, want 0 (should not underflow)", c.intNest)
	}
}

func TestHalt(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	if c.Halted() {
		t.Error("CPU should not be halted initially")
	}
	c.Halt()
	if !c.Halted() {
		t.Error("CPU should be halted after Halt()")
	}
}

func TestHaltedStepCost(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)
	c.halted = true

	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("Halted Step should cost 4 cycles, got %d", cycles)
	}
}

func TestPushPop(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	c.reg.XSP = 0x7000

	c.push(Long, 0xDEADBEEF)
	checkReg32(t, "XSP after push", c.reg.XSP, 0x6FFC)

	// Verify memory contents (little-endian)
	got := bus.Read32(0x6FFC)
	if got != 0xDEADBEEF {
		t.Errorf("pushed value = 0x%08X, want 0xDEADBEEF", got)
	}

	popped := c.pop(Long)
	checkReg32(t, "XSP after pop", c.reg.XSP, 0x7000)
	if popped != 0xDEADBEEF {
		t.Errorf("popped value = 0x%08X, want 0xDEADBEEF", popped)
	}
}

func TestFetchPC(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)

	bus.write8(0x1000, 0xAB)
	bus.write8(0x1001, 0xCD)
	bus.write8(0x1002, 0xEF)
	bus.write8(0x1003, 0x12)

	got := c.fetchPC()
	if got != 0xAB {
		t.Errorf("fetchPC() = 0x%02X, want 0xAB", got)
	}
	checkReg32(t, "PC", c.reg.PC, 0x1001)

	got16 := c.fetchPC16()
	if got16 != 0xEFCD {
		t.Errorf("fetchPC16() = 0x%04X, want 0xEFCD", got16)
	}
	checkReg32(t, "PC", c.reg.PC, 0x1003)
}

func TestFetchPC24(t *testing.T) {
	c, bus := newTestCPU(t, 0x2000)

	bus.write8(0x2000, 0x56)
	bus.write8(0x2001, 0x34)
	bus.write8(0x2002, 0x12)

	got := c.fetchPC24()
	if got != 0x123456 {
		t.Errorf("fetchPC24() = 0x%06X, want 0x123456", got)
	}
	checkReg32(t, "PC", c.reg.PC, 0x2003)
}

func TestFetchPC32(t *testing.T) {
	c, bus := newTestCPU(t, 0x2000)

	bus.write8(0x2000, 0x78)
	bus.write8(0x2001, 0x56)
	bus.write8(0x2002, 0x34)
	bus.write8(0x2003, 0x12)

	got := c.fetchPC32()
	if got != 0x12345678 {
		t.Errorf("fetchPC32() = 0x%08X, want 0x12345678", got)
	}
	checkReg32(t, "PC", c.reg.PC, 0x2004)
}
