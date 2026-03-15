package tlcs900h

import "testing"

// --- CPI tests ---

func TestCPI_Byte(t *testing.T) {
	// CPI via (XHL) prefix: 0x83 = byte (R3=XHL) indirect, op2=0x14
	// A != mem value, so Z=0. BC decremented, pointer incremented.
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.reg.WriteReg32(3, 0x2000)          // XHL = pointer
	c.reg.WriteReg16(1, 5)               // BC = 5
	bus.write8(0x2000, 0x20)             // mem = 0x20

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2001) // incremented by 1
	checkReg16(t, "BC", c.reg.ReadReg16(1), 4)       // decremented by 1
	checkFlags(t, c, 1, 0, -1, 1, 1, -1)             // S=1 (0x10-0x20=0xF0), Z=0, V=1 (BC!=0), N=1
}

func TestCPI_Byte_Match(t *testing.T) {
	// A == mem value, Z=1.
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A = 0x42
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 3)
	bus.write8(0x2000, 0x42) // mem = 0x42 (match)

	c.Step()

	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)
	checkFlags(t, c, 0, 1, 0, 1, 1, -1) // Z=1 (match), V=1 (BC!=0), N=1
}

func TestCPI_BC_Zero(t *testing.T) {
	// BC reaches 0 after decrement, V=0.
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 1) // BC = 1
	bus.write8(0x2000, 0x20)

	c.Step()

	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)
	checkFlags(t, c, -1, 0, -1, 0, 1, -1) // V=0 (BC==0), N=1
}

func TestCPI_Word(t *testing.T) {
	// Word compare using WA register. Prefix 0x93 = word (R3=XHL).
	c, bus := setupMemOp(t, 0x93, 0x14)
	c.reg.WriteReg16(0, 0x1234) // WA = 0x1234
	c.reg.WriteReg32(3, 0x2000) // XHL = pointer
	c.reg.WriteReg16(1, 3)      // BC = 3
	bus.Write16(0x2000, 0x1234)

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2002) // incremented by 2 (word)
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)
	checkFlags(t, c, 0, 1, 0, 1, 1, -1) // Z=1 (match), V=1 (BC!=0)
}

func TestCPI_Carry_Preserved(t *testing.T) {
	// Carry flag must be unchanged after CPI.
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 2)
	bus.write8(0x2000, 0x20)
	c.setFlag(flagC, true) // set carry before

	c.Step()

	checkFlags(t, c, -1, -1, -1, -1, 1, 1) // C=1 preserved
}

func TestCPI_Carry_Preserved_Clear(t *testing.T) {
	// Carry=0 stays 0 even when subtraction would produce borrow.
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 2)
	bus.write8(0x2000, 0x20) // 0x10 - 0x20 would borrow
	c.setFlag(flagC, false)

	c.Step()

	checkFlags(t, c, -1, -1, -1, -1, 1, 0) // C=0 preserved
}

func TestCPI_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x14)
	c.reg.WriteReg8(r8From3bit[1], 0)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 1)
	bus.write8(0x2000, 0)

	cycles := c.Step()

	if cycles != 6 {
		t.Errorf("CPI cycles = %d, want 6", cycles)
	}
}

// --- CPD tests ---

func TestCPD_Byte(t *testing.T) {
	// CPD: pointer decremented instead of incremented.
	c, bus := setupMemOp(t, 0x83, 0x16)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.reg.WriteReg32(3, 0x2005)          // XHL = 0x2005
	c.reg.WriteReg16(1, 3)               // BC = 3
	bus.write8(0x2005, 0x10)             // mem = match

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2004) // decremented by 1
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)
	checkFlags(t, c, 0, 1, 0, 1, 1, -1) // Z=1, V=1, N=1
}

// --- CPIR tests ---

func TestCPIR_Match(t *testing.T) {
	// CPIR stops on match. Search for 0x42 in memory.
	c, bus := setupMemOp(t, 0x83, 0x15)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A = 0x42
	c.reg.WriteReg32(3, 0x2000)          // XHL
	c.reg.WriteReg16(1, 5)               // BC = 5
	bus.write8(0x2000, 0x01)
	bus.write8(0x2001, 0x02)
	bus.write8(0x2002, 0x42) // match at 3rd byte
	bus.write8(0x2003, 0x03)

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2003) // past the match
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)       // 5-3=2
	checkFlags(t, c, 0, 1, 0, 1, 1, -1)              // Z=1 (match), V=1 (BC!=0)
}

func TestCPIR_NoMatch(t *testing.T) {
	// CPIR exhausts BC without finding match.
	c, bus := setupMemOp(t, 0x83, 0x15)
	c.reg.WriteReg8(r8From3bit[1], 0xFF) // A = 0xFF
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 3) // BC = 3
	bus.write8(0x2000, 0x01)
	bus.write8(0x2001, 0x02)
	bus.write8(0x2002, 0x03)

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2003) // scanned all 3
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0)       // exhausted
	checkFlags(t, c, -1, 0, -1, 0, 1, -1)            // Z=0 (no match), V=0 (BC==0)
}

func TestCPIR_Cycles(t *testing.T) {
	// 3 iterations: 6*3+1 = 19 cycles.
	c, bus := setupMemOp(t, 0x83, 0x15)
	c.reg.WriteReg8(r8From3bit[1], 0xFF) // no match
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 3)
	bus.write8(0x2000, 0x01)
	bus.write8(0x2001, 0x02)
	bus.write8(0x2002, 0x03)

	cycles := c.Step()

	if cycles != 19 {
		t.Errorf("CPIR cycles = %d, want 19 (6*3+1)", cycles)
	}
}

func TestCPIR_BC_Zero(t *testing.T) {
	// BC=0 means 65536 iterations (same as LDIR/LDDR).
	// Place match at offset 3 so CPIR stops after 4 iterations.
	c, bus := setupMemOp(t, 0x83, 0x15)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A = 0x42
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 0) // BC = 0 -> 65536
	bus.write8(0x2000, 0x01)
	bus.write8(0x2001, 0x02)
	bus.write8(0x2002, 0x03)
	bus.write8(0x2003, 0x42) // match at 4th byte
	c.setFlag(flagC, true)   // carry should be preserved

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2004) // past the match
	checkReg16(t, "BC", c.reg.ReadReg16(1), 65532)   // 65536-4
	checkFlags(t, c, 0, 1, 0, 1, 1, 1)               // Z=1 (match), V=1 (BC!=0), C=1 preserved
}

func TestCPDR_BC_Zero(t *testing.T) {
	// BC=0 means 65536 iterations for CPDR too.
	c, bus := setupMemOp(t, 0x83, 0x17)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A = 0x42
	c.reg.WriteReg32(3, 0x2004)
	c.reg.WriteReg16(1, 0) // BC = 0 -> 65536
	bus.write8(0x2004, 0x01)
	bus.write8(0x2003, 0x02)
	bus.write8(0x2002, 0x42) // match at 3rd iteration
	c.setFlag(flagC, true)

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2001) // decremented past match
	checkReg16(t, "BC", c.reg.ReadReg16(1), 65533)   // 65536-3
	checkFlags(t, c, 0, 1, 0, 1, 1, 1)               // Z=1 (match), V=1 (BC!=0), C=1 preserved
}

// --- CPDR tests ---

func TestCPDR_Byte(t *testing.T) {
	// CPDR: repeat decrement variant.
	c, bus := setupMemOp(t, 0x83, 0x17)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A = 0x42
	c.reg.WriteReg32(3, 0x2004)          // XHL starts at end
	c.reg.WriteReg16(1, 5)               // BC = 5
	bus.write8(0x2004, 0x01)
	bus.write8(0x2003, 0x02)
	bus.write8(0x2002, 0x42) // match at 3rd iteration
	bus.write8(0x2001, 0x03)

	c.Step()

	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x2001) // decremented past match
	checkReg16(t, "BC", c.reg.ReadReg16(1), 2)       // 5-3=2
	checkFlags(t, c, 0, 1, 0, 1, 1, -1)              // Z=1 (match), V=1 (BC!=0)
}

func TestCPIR_Carry_Preserved(t *testing.T) {
	// Carry preserved during CPIR.
	c, bus := setupMemOp(t, 0x83, 0x15)
	c.reg.WriteReg8(r8From3bit[1], 0x42) // A
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg16(1, 2)
	bus.write8(0x2000, 0x42) // immediate match
	c.setFlag(flagC, true)

	c.Step()

	checkFlags(t, c, -1, 1, -1, -1, 1, 1) // C=1 preserved, Z=1 (match)
}
