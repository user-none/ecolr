package tlcs900h

import "testing"

// --- RCF / SCF / CCF / ZCF ---

func TestRCF(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x10)
	// Set all flags so we can see what gets cleared
	c.setFlags(flagS | flagZ | flagH | flagV | flagN | flagC)
	cycles := c.Step()
	// S:- Z:- H:0 V:- N:0 C:0
	checkFlags(t, c, 1, 1, 0, 1, 0, 0)
	if cycles != 2 {
		t.Errorf("RCF cycles = %d, want 2", cycles)
	}
}

func TestSCF(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x11)
	c.setFlags(flagS | flagZ | flagH | flagN) // C=0 initially
	cycles := c.Step()
	// S:- Z:- H:0 V:- N:0 C:1
	checkFlags(t, c, 1, 1, 0, -1, 0, 1)
	if cycles != 2 {
		t.Errorf("SCF cycles = %d, want 2", cycles)
	}
}

func TestCCF_CarrySet(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x12)
	c.setFlags(flagC) // C=1
	c.Step()
	// C was 1 -> C=0, H=1 (old C)
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

func TestCCF_CarryClear(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x12)
	c.setFlags(0) // C=0
	c.Step()
	// C was 0 -> C=1, H=0
	checkFlags(t, c, -1, -1, 0, -1, 0, 1)
}

func TestZCF_ZeroSet(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x13)
	c.setFlags(flagZ) // Z=1
	c.Step()
	// Z=1 -> C=0
	checkFlags(t, c, -1, -1, 0, -1, 0, 0)
}

func TestZCF_ZeroClear(t *testing.T) {
	c, bus := newTestCPU(t, 0x1000)
	bus.write8(0x1000, 0x13)
	c.setFlags(0) // Z=0
	c.Step()
	// Z=0 -> C=1
	checkFlags(t, c, -1, -1, 0, -1, 0, 1)
}

// --- CPL ---

func TestCPL_Byte(t *testing.T) {
	// Prefix 0xC9 = byte reg1 (A), op2 = 0x06
	c, _ := setupRegOp(t, 0xC9, 0x06)
	c.reg.WriteReg8(r8From3bit[1], 0x55) // A = 0x55
	cycles := c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAA)
	checkFlags(t, c, -1, -1, 1, -1, 1, -1)
	if cycles != 2 {
		t.Errorf("CPL byte cycles = %d, want 2", cycles)
	}
}

func TestCPL_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x06) // word reg0
	c.reg.WriteReg16(0, 0x00FF)
	cycles := c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0xFF00)
	checkFlags(t, c, -1, -1, 1, -1, 1, -1)
	if cycles != 2 {
		t.Errorf("CPL word cycles = %d, want 2", cycles)
	}
}

// --- BIT ---

func TestBIT_BitSet(t *testing.T) {
	// BIT #4,r: bit 3 of A (0x08 has bit 3 set)
	c, _ := setupRegOp(t, 0xC9, 0x33, 0x03) // byte reg1, BIT, bit=3
	c.reg.WriteReg8(r8From3bit[1], 0x08)
	c.Step()
	// Bit 3 is set: Z=0, H=1, N=0
	checkFlags(t, c, 0, 0, 1, -1, 0, -1)
}

func TestBIT_BitClear(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x33, 0x03) // bit 3
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	// Bit 3 clear: Z=1, H=1, N=0
	checkFlags(t, c, 0, 1, 1, -1, 0, -1)
}

func TestBIT_SignBit(t *testing.T) {
	// BIT bit 7 of byte - the sign bit
	c, _ := setupRegOp(t, 0xC9, 0x33, 0x07) // bit 7
	c.reg.WriteReg8(r8From3bit[1], 0x80)
	c.Step()
	// Bit 7 is set and is MSB: S=1, Z=0
	checkFlags(t, c, 1, 0, 1, -1, 0, -1)
}

func TestBIT_OutOfRange(t *testing.T) {
	// Bit 10 on a byte register - out of range
	c, _ := setupRegOp(t, 0xC9, 0x33, 0x0A) // bit 10
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.Step()
	// Out of range bit always reads as 0: Z=1
	checkFlags(t, c, 0, 1, 1, -1, 0, -1)
}

func TestBIT_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x33, 0x00) // byte BIT #0,r
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("BIT byte cycles = %d, want 3", cycles)
	}

	c, _ = setupRegOp(t, 0xD8, 0x33, 0x00) // word BIT #0,r
	cycles = c.Step()
	if cycles != 3 {
		t.Errorf("BIT word cycles = %d, want 3", cycles)
	}
}

// --- SET ---

func TestSET_Reg(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x31, 0x05) // byte reg1, SET bit 5
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x20)
}

func TestSET_OutOfRange(t *testing.T) {
	// SET bit 10 on byte - should not modify
	c, _ := setupRegOp(t, 0xC9, 0x31, 0x0A)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
}

// --- RES ---

func TestRES_Reg(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x30, 0x03) // byte reg1, RES bit 3
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xF7)
}

// --- CHG ---

func TestCHG_Reg(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x32, 0x02) // byte reg1, CHG bit 2
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x04)
}

func TestCHG_Toggle(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x32, 0x02) // CHG bit 2
	c.reg.WriteReg8(r8From3bit[1], 0x04)    // bit 2 already set
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
}

// --- TSET ---

func TestTSET_Reg_BitClear(t *testing.T) {
	// TSET tests the bit (sets Z/S) then sets the bit to 1
	c, _ := setupRegOp(t, 0xC9, 0x34, 0x03) // byte reg1, TSET bit 3
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	cycles := c.Step()
	// Bit 3 was clear: Z=1, then bit 3 set
	checkFlags(t, c, 0, 1, 1, -1, 0, -1)
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x08)
	if cycles != 4 {
		t.Errorf("TSET byte cycles = %d, want 4", cycles)
	}
}

func TestTSET_Reg_BitSet(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x34, 0x03) // TSET bit 3
	c.reg.WriteReg8(r8From3bit[1], 0x08)    // bit 3 already set
	c.Step()
	// Bit 3 was set: Z=0, bit still 1
	checkFlags(t, c, 0, 0, 1, -1, 0, -1)
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x08)
}

// --- ANDCF/ORCF/XORCF/LDCF/STCF #4,r ---

func TestANDCF_Imm_Reg(t *testing.T) {
	// ANDCF #4,r: C = C AND bit(r)
	// C=1, bit 0 of A = 1 -> C = 1&1 = 1
	c, _ := setupRegOp(t, 0xC9, 0x20, 0x00) // bit 0
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)

	// C=1, bit 0 = 0 -> C = 1&0 = 0
	c, _ = setupRegOp(t, 0xC9, 0x20, 0x00)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

func TestORCF_Imm_Reg(t *testing.T) {
	// C=0, bit 2 = 1 -> C = 0|1 = 1
	c, _ := setupRegOp(t, 0xC9, 0x21, 0x02) // bit 2
	c.reg.WriteReg8(r8From3bit[1], 0x04)
	c.setFlag(flagC, false)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)

	// C=0, bit 2 = 0 -> C = 0
	c, _ = setupRegOp(t, 0xC9, 0x21, 0x02)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.setFlag(flagC, false)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

func TestXORCF_Imm_Reg(t *testing.T) {
	// C=1, bit 0 = 1 -> C = 1^1 = 0
	c, _ := setupRegOp(t, 0xC9, 0x22, 0x00)
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

func TestLDCF_Imm_Reg(t *testing.T) {
	// LDCF #4,r: C = bit(r)
	c, _ := setupRegOp(t, 0xC9, 0x23, 0x07) // bit 7
	c.reg.WriteReg8(r8From3bit[1], 0x80)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)

	c, _ = setupRegOp(t, 0xC9, 0x23, 0x07)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

func TestSTCF_Imm_Reg(t *testing.T) {
	// STCF #4,r: bit(r) = C
	c, _ := setupRegOp(t, 0xC9, 0x24, 0x04) // bit 4
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x10)

	// C=0, clear bit 4
	c, _ = setupRegOp(t, 0xC9, 0x24, 0x04)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // bit 4 set
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
}

func TestCarryOps_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x20, 0x00) // ANDCF byte
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("ANDCF byte cycles = %d, want 3", cycles)
	}

	c, _ = setupRegOp(t, 0xD8, 0x20, 0x00) // ANDCF word
	cycles = c.Step()
	if cycles != 3 {
		t.Errorf("ANDCF word cycles = %d, want 3", cycles)
	}
}

// --- ANDCF/ORCF/XORCF/LDCF/STCF A,r ---

func TestANDCF_A_Reg(t *testing.T) {
	// Use extended prefix to separate A from the operand register.
	// C7 = byte extended prefix, next byte = full register code,
	// then 0x28 = ANDCF A,r
	// Full code $E5 = current bank XBC high byte = B
	c, _ := setupRegOp(t, 0xC7, 0xE5, 0x28) // byte ext, reg=B, ANDCF A,r
	c.reg.WriteReg8(r8From3bit[1], 0x02)    // A = 2 (bit index)
	c.reg.WriteReg8(0x09, 0x04)             // B = 0x04 (bit 2 set)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1) // C=1 AND bit2=1 -> 1
}

func TestLDCF_A_Reg(t *testing.T) {
	// Full code $E5 = current bank XBC high byte = B
	c, _ := setupRegOp(t, 0xC7, 0xE5, 0x2B) // LDCF A,r
	c.reg.WriteReg8(r8From3bit[1], 0x03)    // A = 3 (bit index)
	c.reg.WriteReg8(0x09, 0x08)             // B = 0x08 (bit 3 set)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)
}

func TestSTCF_A_Reg(t *testing.T) {
	// Full code $E5 = current bank XBC high byte = B
	c, _ := setupRegOp(t, 0xC7, 0xE5, 0x2C) // STCF A,r
	c.reg.WriteReg8(r8From3bit[1], 0x01)    // A = 1 (bit index)
	c.reg.WriteReg8(0x09, 0x00)             // B = 0
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "B", c.reg.ReadReg8(0x09), 0x02) // bit 1 set
}

// --- BS1F / BS1B ---

func TestBS1F_Found(t *testing.T) {
	// Word prefix 0xD8+r, op2=0x0E
	c, _ := setupRegOp(t, 0xD8, 0x0E) // word reg0, BS1F
	c.reg.WriteReg16(0, 0x0040)       // bit 6 is lowest set
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 6)
	checkFlags(t, c, -1, -1, -1, 0, -1, -1)
}

func TestBS1F_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x0E)
	c.reg.WriteReg16(0, 0x0000)
	c.Step()
	checkFlags(t, c, -1, -1, -1, 1, -1, -1) // V=1 for zero
}

func TestBS1B_Found(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x0F) // BS1B
	c.reg.WriteReg16(0, 0x0040)       // bit 6 is highest set
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 6)
	checkFlags(t, c, -1, -1, -1, 0, -1, -1)
}

func TestBS1B_HighBit(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x0F)
	c.reg.WriteReg16(0, 0x8001) // bit 15 is highest set
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 15)
}

func TestBS1B_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x0F)
	c.reg.WriteReg16(0, 0x0000)
	c.Step()
	checkFlags(t, c, -1, -1, -1, 1, -1, -1)
}

func TestBS1F_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x0E)
	c.reg.WriteReg16(0, 0x0001)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("BS1F cycles = %d, want 3", cycles)
	}
}

// --- Memory bit ops ---

// setupDstMemOp creates an instruction using the B0 (indirect XWA) dst prefix.
// Writes the second byte at PC+1. Returns the CPU and bus.
func setupDstMemOp(t *testing.T, op2 uint8) (*CPU, *testBus) {
	t.Helper()
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xB0)  // dst indirect via reg 0 (XWA)
	bus.write8(pc+1, op2) // second byte
	return c, bus
}

func TestBIT_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xCB) // BIT #3,(mem) - bit 3
	c.reg.WriteReg32(0, 0x2000)      // XWA = pointer
	bus.write8(0x2000, 0x08)         // bit 3 set
	cycles := c.Step()
	checkFlags(t, c, 0, 0, 1, -1, 0, -1)
	if cycles != 6 {
		t.Errorf("BIT mem cycles = %d, want 6", cycles)
	}
}

func TestBIT_Mem_Clear(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xC8) // BIT #0,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xFE) // bit 0 clear
	c.Step()
	checkFlags(t, c, 0, 1, 1, -1, 0, -1)
}

func TestSET_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xBA) // SET #2,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	cycles := c.Step()
	got := bus.Read8(0x2000)
	if got != 0x04 {
		t.Errorf("SET mem: got 0x%02X, want 0x04", got)
	}
	if cycles != 7 {
		t.Errorf("SET mem cycles = %d, want 7", cycles)
	}
}

func TestRES_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xB5) // RES #5,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xFF)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xDF {
		t.Errorf("RES mem: got 0x%02X, want 0xDF", got)
	}
}

func TestCHG_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xC1) // CHG #1,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x02 {
		t.Errorf("CHG mem: got 0x%02X, want 0x02", got)
	}
}

func TestTSET_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xAA) // TSET #2,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	cycles := c.Step()
	// Bit 2 was clear: Z=1, then bit 2 set
	checkFlags(t, c, 0, 1, 1, -1, 0, -1)
	got := bus.Read8(0x2000)
	if got != 0x04 {
		t.Errorf("TSET mem: got 0x%02X, want 0x04", got)
	}
	if cycles != 7 {
		t.Errorf("TSET mem cycles = %d, want 7", cycles)
	}
}

// --- Memory carry flag ops ---

func TestANDCF_Imm_Mem(t *testing.T) {
	// 0x83 = ANDCF #3,(mem) - bit 3
	c, bus := setupDstMemOp(t, 0x83)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x08) // bit 3 set
	c.setFlag(flagC, true)
	cycles := c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1) // 1 AND 1 = 1
	if cycles != 6 {
		t.Errorf("ANDCF mem cycles = %d, want 6", cycles)
	}

	// bit 3 clear, C=1 -> C=0
	c, bus = setupDstMemOp(t, 0x83)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

func TestORCF_Imm_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0x89) // ORCF #1,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x02) // bit 1 set
	c.setFlag(flagC, false)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)
}

func TestXORCF_Imm_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0x92) // XORCF #2,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x04) // bit 2 set
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 0) // 1^1 = 0
}

func TestLDCF_Imm_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0x9D) // LDCF #5,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x20) // bit 5 set
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)
}

func TestSTCF_Imm_Mem(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xA4) // STCF #4,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	c.setFlag(flagC, true)
	cycles := c.Step()
	got := bus.Read8(0x2000)
	if got != 0x10 {
		t.Errorf("STCF mem: got 0x%02X, want 0x10", got)
	}
	if cycles != 7 {
		t.Errorf("STCF mem cycles = %d, want 7", cycles)
	}
}

// --- Memory carry ops with A ---

// setupDstMemOpReg creates a dst-prefix instruction using B0+reg indirect.
// Use a register other than XWA (0) when A is needed as an operand,
// since A is the low byte of XWA.
func setupDstMemOpReg(t *testing.T, reg uint8, op2 uint8) (*CPU, *testBus) {
	t.Helper()
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(pc, 0xB0+reg) // dst indirect via reg
	bus.write8(pc+1, op2)
	return c, bus
}

func TestANDCF_A_Mem(t *testing.T) {
	// Use XBC (reg 1) as pointer since A is low byte of XWA
	c, bus := setupDstMemOpReg(t, 1, 0x28) // ANDCF A,(mem)
	c.reg.WriteReg32(1, 0x2000)            // XBC = pointer
	c.reg.WriteReg8(r8From3bit[1], 0x05)   // A=5 (bit index)
	bus.write8(0x2000, 0x20)               // bit 5 set
	c.setFlag(flagC, true)
	cycles := c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)
	if cycles != 6 {
		t.Errorf("ANDCF A,mem cycles = %d, want 6", cycles)
	}
}

func TestLDCF_A_Mem(t *testing.T) {
	c, bus := setupDstMemOpReg(t, 1, 0x2B)
	c.reg.WriteReg32(1, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x06) // A=6
	bus.write8(0x2000, 0x40)             // bit 6 set
	c.Step()
	checkFlags(t, c, -1, -1, -1, -1, -1, 1)
}

func TestSTCF_A_Mem(t *testing.T) {
	c, bus := setupDstMemOpReg(t, 1, 0x2C)
	c.reg.WriteReg32(1, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x03) // A=3
	bus.write8(0x2000, 0x00)
	c.setFlag(flagC, true)
	cycles := c.Step()
	got := bus.Read8(0x2000)
	if got != 0x08 {
		t.Errorf("STCF A,mem: got 0x%02X, want 0x08", got)
	}
	if cycles != 7 {
		t.Errorf("STCF A,mem cycles = %d, want 7", cycles)
	}
}

func TestSTCF_A_Mem_OutOfRange(t *testing.T) {
	// A=10 (>7) on byte memory: operand should not change per docs
	c, bus := setupDstMemOpReg(t, 1, 0x2C)
	c.reg.WriteReg32(1, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x0A) // A=10, out of range for byte
	bus.write8(0x2000, 0x55)
	c.setFlag(flagC, true)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x55 {
		t.Errorf("STCF A,mem out-of-range: got 0x%02X, want 0x55 (unchanged)", got)
	}
}

func TestLDCF_A_Mem_OutOfRange(t *testing.T) {
	// A=12 (>7) on byte memory: bit out of range, getBit returns false
	c, bus := setupDstMemOpReg(t, 1, 0x2B)
	c.reg.WriteReg32(1, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x0C) // A=12
	bus.write8(0x2000, 0xFF)
	c.setFlag(flagC, true) // C was 1
	c.Step()
	// Out-of-range bit reads as 0, so C=0
	checkFlags(t, c, -1, -1, -1, -1, -1, 0)
}

// --- Verify arith ops work through source prefix ---

func TestADD_MemReg_AfterWrap(t *testing.T) {
	// Use source prefix (0x81 = byte indirect via XBC) with op2=0x88 (ADD (mem),R0)
	// This slot was wrapped with ORCF for dst prefix
	c, bus := setupMemOp(t, 0x81, 0x88)
	c.reg.WriteReg32(1, 0x2000)          // XBC = pointer
	bus.write8(0x2000, 0x10)             // mem = 0x10
	c.reg.WriteReg8(r8From3bit[0], 0x05) // W = 0x05
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x15 {
		t.Errorf("ADD (mem),R after wrap: got 0x%02X, want 0x15", got)
	}
}

func TestSBC_RegMem_AfterWrap(t *testing.T) {
	// Source prefix 0x81, op2=0xB0 (SBC R0,(mem))
	// This slot was wrapped with RES for dst prefix
	c, bus := setupMemOp(t, 0x81, 0xB0)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x05) // W = 0x05
	c.setFlag(flagC, false)              // no borrow
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x04)
}

// --- Word register bit ops ---

func TestBIT_Word(t *testing.T) {
	// BIT bit 15 of word register
	c, _ := setupRegOp(t, 0xD8, 0x33, 0x0F) // word reg0, BIT bit 15
	c.reg.WriteReg16(0, 0x8000)
	c.Step()
	// Bit 15 set (MSB of word): S=1, Z=0
	checkFlags(t, c, 1, 0, 1, -1, 0, -1)
}

func TestSET_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x31, 0x0A) // word reg0, SET bit 10
	c.reg.WriteReg16(0, 0x0000)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0400)
}

func TestRES_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x30, 0x08) // word reg0, RES bit 8
	c.reg.WriteReg16(0, 0xFFFF)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0xFEFF)
}
