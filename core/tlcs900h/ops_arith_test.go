package tlcs900h

import "testing"

// Helper to set up a CPU with a register prefix instruction in memory.
// prefix is the first byte (e.g. 0xC8 for byte reg 0), op2 is the second byte.
// Returns the CPU, bus, and the address after the instruction bytes.
func setupRegOp(t *testing.T, prefix uint8, op2 uint8, extra ...uint8) (*CPU, *testBus) {
	t.Helper()
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	addr := pc
	bus.write8(addr, prefix)
	addr++
	bus.write8(addr, op2)
	addr++
	for _, b := range extra {
		bus.write8(addr, b)
		addr++
	}
	return c, bus
}

// setupMemOp sets up a memory-prefix instruction.
func setupMemOp(t *testing.T, prefix uint8, op2 uint8, extra ...uint8) (*CPU, *testBus) {
	t.Helper()
	return setupRegOp(t, prefix, op2, extra...)
}

// checkFlags verifies flag state. Pass -1 to skip checking a flag.
func checkFlags(t *testing.T, c *CPU, s, z, h, v, n, carry int) {
	t.Helper()
	f := c.flags()
	if s >= 0 {
		got := f&flagS != 0
		if got != (s != 0) {
			t.Errorf("S flag = %v, want %v", got, s != 0)
		}
	}
	if z >= 0 {
		got := f&flagZ != 0
		if got != (z != 0) {
			t.Errorf("Z flag = %v, want %v", got, z != 0)
		}
	}
	if h >= 0 {
		got := f&flagH != 0
		if got != (h != 0) {
			t.Errorf("H flag = %v, want %v", got, h != 0)
		}
	}
	if v >= 0 {
		got := f&flagV != 0
		if got != (v != 0) {
			t.Errorf("V flag = %v, want %v", got, v != 0)
		}
	}
	if n >= 0 {
		got := f&flagN != 0
		if got != (n != 0) {
			t.Errorf("N flag = %v, want %v", got, n != 0)
		}
	}
	if carry >= 0 {
		got := f&flagC != 0
		if got != (carry != 0) {
			t.Errorf("C flag = %v, want %v", got, carry != 0)
		}
	}
}

// --- ADD tests ---

func TestADD_RegReg_Byte(t *testing.T) {
	// ADD R,r where R is from second byte, r is prefix register
	// Prefix 0xC9 = byte reg 1 (A), second byte 0x80 = ADD R0(W),r
	// So: W = W + A
	c, _ := setupRegOp(t, 0xC9, 0x80)    // prefix=byte reg1(A), op2=0x80 (ADD R0,r)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10 (prefix reg)
	c.reg.WriteReg8(r8From3bit[0], 0x20) // W = 0x20 (R from second byte)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x30)
	checkFlags(t, c, 0, 0, 0, 0, 0, 0) // no flags set
}

func TestADD_RegReg_Word(t *testing.T) {
	// Prefix 0xD8 = word reg 0 (XWA.W), second byte 0x81 = ADD R1,r
	// R1 = XBC.W, r = XWA.W => XBC.W = XBC.W + XWA.W
	c, _ := setupRegOp(t, 0xD8, 0x81)
	c.reg.WriteReg16(0, 0x1000) // WA = 0x1000
	c.reg.WriteReg16(1, 0x2000) // BC = 0x2000
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x3000)
	checkFlags(t, c, 0, 0, 0, 0, 0, 0)
}

func TestADD_RegReg_Long(t *testing.T) {
	// Prefix 0xE8 = long reg 0 (XWA), second byte 0x81 = ADD R1,r
	c, _ := setupRegOp(t, 0xE8, 0x81)
	c.reg.WriteReg32(0, 0x10000000) // XWA
	c.reg.WriteReg32(1, 0x20000000) // XBC
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x30000000)
}

func TestADD_RegImm_Byte(t *testing.T) {
	// Prefix 0xC9 = byte reg 1 (A), op2 = 0xC8 (ADD r,#), imm follows
	c, _ := setupRegOp(t, 0xC9, 0xC8, 0x05)
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x15)
	checkFlags(t, c, 0, 0, 0, 0, 0, 0)
}

func TestADD_RegImm_Word(t *testing.T) {
	// Prefix 0xD8 = word reg 0, op2 = 0xC8, imm16 follows (LE)
	c, _ := setupRegOp(t, 0xD8, 0xC8, 0x00, 0x10) // imm = 0x1000
	c.reg.WriteReg16(0, 0x2000)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x3000)
}

func TestADD_RegImm_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC8, 0x01) // byte ADD r,#
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("ADD r,# byte cycles = %d, want 3", cycles)
	}
}

func TestADD_RegMem_Byte(t *testing.T) {
	// Prefix 0x81 = byte (R1) indirect, R1 = XBC
	// op2 = 0x81 = ADD R1(A),(mem)
	c, bus := setupMemOp(t, 0x81, 0x81)
	c.reg.WriteReg32(1, 0x2000)          // XBC = pointer
	bus.write8(0x2000, 0x05)             // mem value
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x15)
}

func TestADD_MemReg_Byte(t *testing.T) {
	// Prefix 0x81 = byte (R1) indirect, R1=XBC
	// op2 = 0x89 = ADD (mem),R1 (R1 in byte = A)
	c, bus := setupMemOp(t, 0x81, 0x89)
	c.reg.WriteReg32(1, 0x2000)          // XBC = pointer
	bus.write8(0x2000, 0x10)             // mem = 0x10
	c.reg.WriteReg8(r8From3bit[1], 0x05) // A = 0x05
	c.Step()
	got := bus.Read(Byte, 0x2000)
	if got != 0x15 {
		t.Errorf("(mem) = 0x%02X, want 0x15", got)
	}
}

func TestADD_MemImm_Word(t *testing.T) {
	// Prefix 0x90 = word (R0) indirect
	// op2 = 0x38 = ADD<W> (mem),#
	c, bus := setupMemOp(t, 0x90, 0x38, 0x00, 0x10) // imm16 = 0x1000
	c.reg.WriteReg32(0, 0x2000)
	bus.Write(Word, 0x2000, 0x2000)
	c.Step()
	got := bus.Read(Word, 0x2000)
	if got != 0x3000 {
		t.Errorf("(mem) = 0x%04X, want 0x3000", got)
	}
}

func TestADD_Overflow_Byte(t *testing.T) {
	// 0x7F + 0x01 = 0x80, overflow
	c, _ := setupRegOp(t, 0xC9, 0xC8, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x7F)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x80)
	checkFlags(t, c, 1, 0, 1, 1, 0, 0) // S=1, H=1, V=1
}

func TestADD_Carry_Byte(t *testing.T) {
	// 0xFF + 0x01 = 0x00 with carry
	c, _ := setupRegOp(t, 0xC9, 0xC8, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
	checkFlags(t, c, 0, 1, 1, 0, 0, 1) // Z=1, H=1, C=1
}

func TestADD_HalfCarry_Byte(t *testing.T) {
	// 0x0F + 0x01 = 0x10, half carry
	c, _ := setupRegOp(t, 0xC9, 0xC8, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x10)
	checkFlags(t, c, 0, 0, 1, 0, 0, 0) // H=1
}

// --- ADC tests ---

func TestADC_WithCarry_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC9, 0x01) // ADC r,# byte
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, true) // set carry
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x12) // 0x10 + 0x01 + 1
	checkFlags(t, c, 0, 0, 0, 0, 0, 0)
}

func TestADC_WithoutCarry_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC9, 0x01) // ADC r,# byte
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x11)
}

func TestADC_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC9, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("ADC r,# byte cycles = %d, want 3", cycles)
	}
}

// --- SUB tests ---

func TestSUB_RegImm_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCA, 0x05) // SUB r,# byte
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0B)
	checkFlags(t, c, 0, 0, 1, 0, 1, 0) // H=1, N=1
}

func TestSUB_Underflow_Byte(t *testing.T) {
	// 0x00 - 0x01 = 0xFF with borrow
	c, _ := setupRegOp(t, 0xC9, 0xCA, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xFF)
	checkFlags(t, c, 1, 0, 1, 0, 1, 1) // S=1, H=1, N=1, C=1
}

func TestSUB_Overflow_Byte(t *testing.T) {
	// 0x80 - 0x01 = 0x7F, overflow (negative to positive)
	c, _ := setupRegOp(t, 0xC9, 0xCA, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x80)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x7F)
	checkFlags(t, c, 0, 0, 1, 1, 1, 0) // H=1, V=1, N=1
}

func TestSUB_Zero_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCA, 0x10)
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
	checkFlags(t, c, 0, 1, 0, 0, 1, 0) // Z=1, N=1
}

func TestSUB_RegReg_Word(t *testing.T) {
	// Prefix 0xD8 = word reg 0, op2 = 0xA1 = SUB R1,r
	c, _ := setupRegOp(t, 0xD8, 0xA1)
	c.reg.WriteReg16(0, 0x1000) // WA (prefix reg)
	c.reg.WriteReg16(1, 0x3000) // BC (R from op2)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x2000)
}

// --- SBC tests ---

func TestSBC_WithBorrow_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCB, 0x01) // SBC r,# byte
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0E) // 0x10 - 0x01 - 1
	checkFlags(t, c, 0, 0, 1, 0, 1, 0)                     // H=1, N=1
}

func TestSBC_WithoutBorrow_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCB, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0F)
}

// --- CP tests ---

func TestCP_RegReg_Byte(t *testing.T) {
	// CP R,r: 0xF0 in regOps
	c, _ := setupRegOp(t, 0xC9, 0xF0)    // prefix=byte A, op2=0xF0 CP R0(W),r(A)
	c.reg.WriteReg8(r8From3bit[0], 0x10) // W
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A (prefix)
	c.Step()
	// W should be unchanged (CP discards result)
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x10)
	checkFlags(t, c, 0, 1, 0, 0, 1, 0) // Z=1, N=1 (equal)
}

func TestCP_Quick_Byte(t *testing.T) {
	// CP r,#3 with value from low 3 bits
	// op2 = 0xDB => #3 = 3
	c, _ := setupRegOp(t, 0xC9, 0xDB)    // prefix=byte A, op2=0xDB
	c.reg.WriteReg8(r8From3bit[1], 0x05) // A = 5
	c.Step()
	// 5 - 3 = 2
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x05) // A unchanged
	checkFlags(t, c, 0, 0, 0, 0, 1, 0)                     // N=1
}

func TestCP_Quick_Zero(t *testing.T) {
	// CP r,#3 with #3=0 means compare with 0
	c, _ := setupRegOp(t, 0xC9, 0xD8)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkFlags(t, c, 0, 1, 0, 0, 1, 0) // Z=1
}

func TestCP_RegImm_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCF, 0x10) // CP r,# byte
	c.reg.WriteReg8(r8From3bit[1], 0x20)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x20) // unchanged
	checkFlags(t, c, 0, 0, 0, 0, 1, 0)                     // N=1, result=0x10
}

func TestCP_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xF0)
	c.reg.WriteReg8(r8From3bit[0], 0)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("CP R,r byte cycles = %d, want 2", cycles)
	}
}

// --- INC tests ---

func TestINC_Reg_Byte(t *testing.T) {
	// INC #3,r: regOps 0x61 means #3=1
	c, _ := setupRegOp(t, 0xC9, 0x61) // prefix=byte A, INC 1,A
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, true) // carry should be preserved
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x11)
	checkFlags(t, c, 0, 0, 0, 0, 0, 1) // C preserved=1
}

func TestINC_By8_Byte(t *testing.T) {
	// INC with #3=0 means increment by 8
	c, _ := setupRegOp(t, 0xC9, 0x60)
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x18)
}

func TestINC_CarryPreserved(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x61)
	c.reg.WriteReg8(r8From3bit[1], 0xFF) // will wrap to 0
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
	checkFlags(t, c, 0, 1, 1, 0, 0, 0) // Z=1, H=1, C preserved=0
}

func TestINC_Mem_Word(t *testing.T) {
	// Prefix 0x90 = word (R0) indirect, op2 = 0x62 (INC 2)
	c, bus := setupMemOp(t, 0x90, 0x62)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write(Word, 0x2000, 0x1000)
	c.Step()
	got := bus.Read(Word, 0x2000)
	if got != 0x1002 {
		t.Errorf("(mem) = 0x%04X, want 0x1002", got)
	}
}

func TestINC_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x61)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("INC reg cycles = %d, want 2", cycles)
	}
}

// --- DEC tests ---

func TestDEC_Reg_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x69) // DEC 1,A
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0F)
	checkFlags(t, c, 0, 0, 1, 0, 1, 1) // H=1, N=1, C preserved=1
}

func TestDEC_By8_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x68) // DEC 8,A
	c.reg.WriteReg8(r8From3bit[1], 0x10)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x08)
}

func TestDEC_ToZero_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x69) // DEC 1,A
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
	checkFlags(t, c, 0, 1, 0, 0, 1, -1) // Z=1, N=1
}

func TestDEC_CarryPreserved(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x69)
	c.reg.WriteReg8(r8From3bit[1], 0x00) // will wrap to 0xFF
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xFF)
	checkFlags(t, c, 1, 0, 1, 0, 1, 1) // S=1, H=1, N=1, C=1 (preserved)
}

// --- NEG tests ---

func TestNEG_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x07) // NEG A
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xFF) // 0 - 1 = 0xFF
	checkFlags(t, c, 1, 0, 1, 0, 1, 1)                     // S=1, H=1, N=1, C=1
}

func TestNEG_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x07)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x00)
	checkFlags(t, c, 0, 1, 0, 0, 1, 0) // Z=1, N=1
}

func TestNEG_MaxNegative_Byte(t *testing.T) {
	// NEG 0x80 = 0 - 0x80 = 0x80 (overflow)
	c, _ := setupRegOp(t, 0xC9, 0x07)
	c.reg.WriteReg8(r8From3bit[1], 0x80)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x80)
	checkFlags(t, c, 1, 0, 0, 1, 1, 1) // S=1, V=1, N=1, C=1
}

func TestNEG_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x07)
	c.reg.WriteReg8(r8From3bit[1], 1)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("NEG cycles = %d, want 2", cycles)
	}
}

// --- MUL tests ---

func TestMUL_RegReg_Byte(t *testing.T) {
	// Prefix 0xC9 = byte reg1(A), op2 = 0x40 = MUL RR,r => R0 pair
	// MUL R0,r => (W:A pair result), r=prefix reg (A)
	// R from second byte = 0 => register pair 0 (WA word)
	// Multiplies: R0 byte * prefix byte => 16-bit into WA
	c, _ := setupRegOp(t, 0xC9, 0x40)
	c.reg.WriteReg8(r8From3bit[1], 10) // A = 10 (prefix reg, multiplier)
	c.reg.WriteReg8(r8From3bit[0], 20) // W = 20 (R from op2, also multiplicand)
	// Wait - the MUL RR,r handler reads readR(sz,op) for b and readOpReg() for a
	// readOpReg reads the prefix register. readR reads from op2 R code.
	// So: a = prefix reg (A=10), b = R0 from op2 (W=20)
	// Result: 10 * 20 = 200, written as word to R0 pair (WA)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 200)
}

func TestMUL_RegReg_Word(t *testing.T) {
	// Prefix 0xD8 = word reg0, op2 = 0x41 = MUL RR,r (R1)
	c, _ := setupRegOp(t, 0xD8, 0x41)
	c.reg.WriteReg16(0, 100) // WA.W = 100 (prefix reg)
	c.reg.WriteReg16(1, 200) // BC.W = 200 (R from op2)
	// a = prefix(WA.W=100), b = R1(BC.W=200)
	// 100 * 200 = 20000, written as long to R1 pair (XBC)
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 20000)
}

func TestMUL_RegImm_Byte(t *testing.T) {
	// Prefix 0xC8 = byte reg0 (W), op2 = 0x08 (MUL rr,#), imm follows
	c, _ := setupRegOp(t, 0xC8, 0x08, 5) // W * 5
	c.reg.WriteReg8(r8From3bit[0], 10)   // W = 10
	c.Step()
	// Result should be in WA word register (reg 0)
	checkReg16(t, "WA", c.reg.ReadReg16(0), 50)
}

func TestMUL_Mem_Byte(t *testing.T) {
	// Prefix 0x81 = byte (R1) indirect, R1=XBC, op2 = 0x40 = MUL RR,(mem) R0 pair
	c, bus := setupMemOp(t, 0x81, 0x40)
	c.reg.WriteReg32(1, 0x2000)       // XBC = pointer
	bus.write8(0x2000, 7)             // mem = 7
	c.reg.WriteReg8(r8From3bit[0], 6) // W = 6 (R0 byte = W)
	// MUL R0 pair: a = W = 6, b = mem = 7, result = 42 => WA word
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 42)
}

func TestMUL_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x40)
	c.reg.WriteReg8(r8From3bit[1], 1)
	c.reg.WriteReg8(r8From3bit[0], 1)
	cycles := c.Step()
	if cycles != 11 {
		t.Errorf("MUL RR,r byte cycles = %d, want 11", cycles)
	}
}

// --- MULS tests ---

func TestMULS_RegReg_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x48)    // MULS RR,r R0
	c.reg.WriteReg8(r8From3bit[1], 0xFE) // A = -2 (signed)
	c.reg.WriteReg8(r8From3bit[0], 3)    // W = 3
	// a = prefix(A=-2), b = R0(W=3)
	// -2 * 3 = -6 => 0xFFFA as uint16
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0xFFFA)
}

func TestMULS_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0x48)
	c.reg.WriteReg8(r8From3bit[1], 1)
	c.reg.WriteReg8(r8From3bit[0], 1)
	cycles := c.Step()
	if cycles != 9 {
		t.Errorf("MULS RR,r byte cycles = %d, want 9", cycles)
	}
}

// --- DIV tests ---

func TestDIV_RegReg_Byte(t *testing.T) {
	// DIV RR,r: op2 = 0x50+R
	// Use prefix reg 0 (W) as divisor, byte code 3 (C) => parent word BC
	c, _ := setupRegOp(t, 0xC8, 0x53) // prefix=byte reg0(W), DIV C-pair(BC),r(W)
	c.reg.WriteReg16(1, 100)          // BC = 100 (dividend, word)
	c.reg.WriteReg8(r8From3bit[0], 7) // W = 7 (divisor, byte)
	c.Step()
	// 100 / 7 = 14 remainder 2
	got := c.reg.ReadReg16(1)
	quotient := uint8(got)
	remainder := uint8(got >> 8)
	if quotient != 14 {
		t.Errorf("quotient = %d, want 14", quotient)
	}
	if remainder != 2 {
		t.Errorf("remainder = %d, want 2", remainder)
	}
}

func TestDIV_ByZero_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x51) // prefix=W, DIV R1(BC)
	c.reg.WriteReg16(1, 100)
	c.reg.WriteReg8(r8From3bit[0], 0) // W = 0 (divisor)
	c.Step()
	checkFlags(t, c, -1, -1, -1, 1, -1, -1) // V=1
}

func TestDIV_Overflow_Byte(t *testing.T) {
	// Quotient > 255
	c, _ := setupRegOp(t, 0xC8, 0x51)
	c.reg.WriteReg16(1, 0x1000)       // 4096
	c.reg.WriteReg8(r8From3bit[0], 1) // W=1, quotient=4096>255
	c.Step()
	checkFlags(t, c, -1, -1, -1, 1, -1, -1) // V=1
}

func TestDIV_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x51)
	c.reg.WriteReg16(1, 10)
	c.reg.WriteReg8(r8From3bit[0], 2) // W=2
	cycles := c.Step()
	if cycles != 15 {
		t.Errorf("DIV RR,r byte cycles = %d, want 15", cycles)
	}
}

// --- DIVS tests ---

func TestDIVS_RegReg_Byte(t *testing.T) {
	// prefix=W (reg0), byte code 3 (C) => parent word BC
	c, _ := setupRegOp(t, 0xC8, 0x5B) // prefix=byte reg0(W), DIVS C-pair(BC),r(W)
	c.reg.WriteReg16(1, 0xFF9C)       // BC = -100 as int16
	c.reg.WriteReg8(r8From3bit[0], 7) // W = 7 (divisor)
	c.Step()
	got := c.reg.ReadReg16(1)
	quotient := int8(got)
	remainder := int8(got >> 8)
	if quotient != -14 {
		t.Errorf("quotient = %d, want -14", quotient)
	}
	if remainder != -2 {
		t.Errorf("remainder = %d, want -2", remainder)
	}
}

func TestDIVS_ByZero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x59)
	c.reg.WriteReg16(1, 100)
	c.reg.WriteReg8(r8From3bit[0], 0) // W=0
	c.Step()
	checkFlags(t, c, -1, -1, -1, 1, -1, -1) // V=1
}

func TestDIVS_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x59)
	c.reg.WriteReg16(1, 10)
	c.reg.WriteReg8(r8From3bit[0], 2) // W=2
	cycles := c.Step()
	if cycles != 18 {
		t.Errorf("DIVS RR,r byte cycles = %d, want 18", cycles)
	}
}

// --- DAA tests ---

func TestDAA_AfterAdd(t *testing.T) {
	// Simulate: A = 0x15 + 0x27 = 0x3C (BCD should be 42)
	// We just test DAA directly on a value that needs correction
	c, _ := setupRegOp(t, 0xC8, 0x10)    // DAA on reg 0 (W), byte prefix
	c.reg.WriteReg8(r8From3bit[0], 0x3C) // raw result of 15+27
	c.setFlag(flagN, false)              // after addition
	c.setFlag(flagH, false)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x42) // BCD corrected
}

func TestDAA_AfterAdd_WithHalfCarry(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0x10) // e.g. 0x09 + 0x07 = 0x10, H was set
	c.setFlag(flagN, false)
	c.setFlag(flagH, true)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x16)
}

func TestDAA_AfterSub(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0xFA) // e.g. 0x00 - 0x06 = 0xFA with borrow
	c.setFlag(flagN, true)               // after subtraction
	c.setFlag(flagH, true)
	c.setFlag(flagC, true)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x94) // BCD: 0xFA - 0x66 = 0x94
}

func TestDAA_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("DAA cycles = %d, want 4", cycles)
	}
}

// --- EXTZ tests ---

func TestEXTZ_Word(t *testing.T) {
	// EXTZ on word: clear high byte
	c, _ := setupRegOp(t, 0xD8, 0x12) // word prefix reg 0, EXTZ
	c.reg.WriteReg16(0, 0xAB12)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0012)
}

func TestEXTZ_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x12) // long prefix reg 0, EXTZ
	c.reg.WriteReg32(0, 0xABCD1234)
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0x00001234)
}

func TestEXTZ_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x12)
	c.reg.WriteReg16(0, 0xFF)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("EXTZ cycles = %d, want 3", cycles)
	}
}

// --- EXTS tests ---

func TestEXTS_Word_Positive(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x13) // word prefix reg 0, EXTS
	c.reg.WriteReg16(0, 0x007F)       // bit 7 = 0
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x007F)
}

func TestEXTS_Word_Negative(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x13)
	c.reg.WriteReg16(0, 0x0080) // bit 7 = 1
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0xFF80)
}

func TestEXTS_Long_Positive(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x13)
	c.reg.WriteReg32(0, 0x00007FFF)
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0x00007FFF)
}

func TestEXTS_Long_Negative(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x13)
	c.reg.WriteReg32(0, 0x00008000) // bit 15 = 1
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0xFFFF8000)
}

// --- PAA tests ---

func TestPAA_Even(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x14) // word prefix reg 0
	c.reg.WriteReg16(0, 0x1000)       // already even
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1000)
}

func TestPAA_Odd(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x14)
	c.reg.WriteReg16(0, 0x1001) // odd
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1002)
}

func TestPAA_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x14)
	c.reg.WriteReg16(0, 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("PAA cycles = %d, want 4", cycles)
	}
}

// --- MINC/MDEC tests ---

func TestMINC1_Wrap(t *testing.T) {
	// MINC1 with mask=0x000F (low nibble), step=1
	// Prefix 0xD8 = word reg 0, op2 = 0x38, mask follows as 16-bit
	c, _ := setupRegOp(t, 0xD8, 0x38, 0x0F, 0x00) // mask=0x000F
	c.reg.WriteReg16(0, 0x100F)                   // WA = 0x100F
	c.Step()
	// Should wrap: (0x100F & ~0x000F) | ((0x100F + 1) & 0x000F) = 0x1000 | 0x0000 = 0x1000
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1000)
}

func TestMINC1_NoWrap(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x38, 0x0F, 0x00) // mask=0x000F
	c.reg.WriteReg16(0, 0x1005)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1006)
}

func TestMINC2(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x39, 0x0F, 0x00) // MINC2, mask=0x000F
	c.reg.WriteReg16(0, 0x100E)
	c.Step()
	// (0x100E & ~0x000F) | ((0x100E + 2) & 0x000F) = 0x1000 | 0x0000 = 0x1000
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1000)
}

func TestMINC4(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x3A, 0x0F, 0x00) // MINC4, mask=0x000F
	c.reg.WriteReg16(0, 0x100C)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x1000)
}

func TestMINC_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x38, 0x0F, 0x00)
	c.reg.WriteReg16(0, 0)
	cycles := c.Step()
	if cycles != 5 {
		t.Errorf("MINC1 cycles = %d, want 5", cycles)
	}
}

func TestMDEC1_Wrap(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x3C, 0x0F, 0x00) // MDEC1, mask=0x000F
	c.reg.WriteReg16(0, 0x1000)
	c.Step()
	// (0x1000 & ~0x000F) | ((0x1000 - 1) & 0x000F) = 0x1000 | 0x000F = 0x100F
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x100F)
}

func TestMDEC2(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x3D, 0x0F, 0x00) // MDEC2, mask=0x000F
	c.reg.WriteReg16(0, 0x1000)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x100E)
}

func TestMDEC4(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x3E, 0x0F, 0x00) // MDEC4, mask=0x000F
	c.reg.WriteReg16(0, 0x1000)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x100C)
}

func TestMDEC_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0x3C, 0x0F, 0x00)
	c.reg.WriteReg16(0, 0x10)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("MDEC1 cycles = %d, want 4", cycles)
	}
}

// --- MULA tests ---

func TestMULA_Basic(t *testing.T) {
	// MULA rr: rr = rr + (XDE) * (XHL), XHL -= 2
	// Prefix D8+0 = word reg 0 (WA), op2 = 0x19
	c, bus := setupRegOp(t, 0xD8, 0x19)
	// Set up accumulator: XWA = 100
	c.reg.WriteReg32(0, 100)
	// XDE points to memory with signed word value 3
	c.reg.WriteReg32(2, 0x3000)
	bus.Write(Word, 0x3000, 3)
	// XHL points to memory with signed word value 5
	c.reg.WriteReg32(3, 0x4000)
	bus.Write(Word, 0x4000, 5)
	c.Step()
	// 100 + 3*5 = 115
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 115)
	// XHL decremented by 2
	checkReg32(t, "XHL", c.reg.ReadReg32(3), 0x3FFE)
}

func TestMULA_SignedMemory(t *testing.T) {
	// Verify signed memory reads
	c, bus := setupRegOp(t, 0xD8, 0x19)
	c.reg.WriteReg32(0, 0) // XWA = 0 accumulator
	c.reg.WriteReg32(2, 0x3000)
	bus.Write(Word, 0x3000, 0xFFFC) // (XDE) = -4 as uint16
	c.reg.WriteReg32(3, 0x4000)
	bus.Write(Word, 0x4000, 0xFFFD) // (XHL) = -3 as uint16
	c.Step()
	// 0 + (-4)*(-3) = 12
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 12)
}

func TestMULA_FlagsPreserved(t *testing.T) {
	// H, N, C should be preserved; S, Z should update
	c, bus := setupRegOp(t, 0xD8, 0x19)
	c.reg.WriteReg32(0, 0) // accumulator = 0
	c.reg.WriteReg32(2, 0x3000)
	bus.Write(Word, 0x3000, 1)
	c.reg.WriteReg32(3, 0x4000)
	bus.Write(Word, 0x4000, 0) // 0 + 1*0 = 0 => Z=1
	// Set H, N, C before
	c.setFlag(flagH, true)
	c.setFlag(flagN, true)
	c.setFlag(flagC, true)
	c.Step()
	checkFlags(t, c, 0, 1, 1, 0, 1, 1) // Z=1, H/N/C preserved
}

func TestMULA_Cycles(t *testing.T) {
	c, bus := setupRegOp(t, 0xD8, 0x19)
	c.reg.WriteReg32(0, 0)
	c.reg.WriteReg32(2, 0x3000)
	bus.Write(Word, 0x3000, 0)
	c.reg.WriteReg32(3, 0x4000)
	bus.Write(Word, 0x4000, 0)
	cycles := c.Step()
	if cycles != 19 {
		t.Errorf("MULA cycles = %d, want 19", cycles)
	}
}

// --- DIV/DIVS flag preservation tests ---

func TestDIV_FlagsPreserved(t *testing.T) {
	// DIV should only modify V; other flags preserved
	// Byte code 3 (C) => parent word BC
	c, _ := setupRegOp(t, 0xC8, 0x53)
	c.reg.WriteReg16(1, 10)           // BC = 10
	c.reg.WriteReg8(r8From3bit[0], 3) // W=3 (divisor)
	// Set all flags before
	c.setFlag(flagS, true)
	c.setFlag(flagZ, true)
	c.setFlag(flagH, true)
	c.setFlag(flagN, true)
	c.setFlag(flagC, true)
	c.Step()
	// V=0 (success), all others preserved
	checkFlags(t, c, 1, 1, 1, 0, 1, 1)
}

func TestDIV_OverflowFlagsPreserved(t *testing.T) {
	// DIV overflow should only set V; other flags preserved
	c, _ := setupRegOp(t, 0xC8, 0x51)
	c.reg.WriteReg16(1, 0x1000) // 4096
	c.reg.WriteReg8(r8From3bit[0], 1)
	c.setFlag(flagS, true)
	c.setFlag(flagN, true)
	c.Step()
	// V=1 (overflow), S and N preserved
	checkFlags(t, c, 1, -1, -1, 1, 1, -1)
}

func TestDIVS_FlagsPreserved(t *testing.T) {
	// Byte code 3 (C) => parent word BC
	c, _ := setupRegOp(t, 0xC8, 0x5B)
	c.reg.WriteReg16(1, 0xFF9C) // BC = -100 as int16
	c.reg.WriteReg8(r8From3bit[0], 7)
	c.setFlag(flagS, true)
	c.setFlag(flagC, true)
	c.Step()
	// V=0, S and C preserved
	checkFlags(t, c, 1, -1, -1, 0, -1, 1)
}

// --- Memory prefix tests ---

func TestMem_Disp8_Byte(t *testing.T) {
	// Prefix 0x89 = byte (R1+d8), R1=XBC
	// d8 = +16, so addr = XBC + 16
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0x89)             // prefix: byte (R1+d8)
	bus.write8(0x1001, 0x10)             // d8 = +16
	bus.write8(0x1002, 0x81)             // ADD R1(A),(mem) - R1 in op2 for byte = A
	c.reg.WriteReg32(1, 0x2000)          // XBC (R1) = pointer base
	bus.write8(0x2010, 0x05)             // value at mem = 0x2000+16
	c.reg.WriteReg8(r8From3bit[1], 0x10) // A = 0x10
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x15) // 0x10 + 0x05
}

func TestMem_Disp8_Negative(t *testing.T) {
	// d8 = -16 (0xF0), using R1=XBC as base
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0x89)             // byte (R1+d8)
	bus.write8(0x1001, 0xF0)             // d8 = -16
	bus.write8(0x1002, 0x81)             // ADD R1(A),(mem) - R1 in op2 = A
	c.reg.WriteReg32(1, 0x2010)          // XBC = 0x2010
	bus.write8(0x2000, 0x03)             // at 0x2010-16 = 0x2000
	c.reg.WriteReg8(r8From3bit[1], 0x01) // A = 0x01
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x04)
}

func TestMem_Imm16(t *testing.T) {
	// Prefix 0xC1 = byte (#16), addr follows as 16-bit LE
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xC1) // byte (#16)
	bus.write8(0x1001, 0x00) // addr low
	bus.write8(0x1002, 0x20) // addr high => 0x2000
	bus.write8(0x1003, 0x81) // ADD R1(A),(mem)
	bus.write8(0x2000, 0x07)
	c.reg.WriteReg8(r8From3bit[1], 0x03)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0A)
}

// --- Extended register prefix tests ---

func TestExtendedReg_Byte(t *testing.T) {
	// Prefix 0xC7 = byte extended, next byte = register code
	// Use register code 0x00 = A (low byte of XWA[0])
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	bus.write8(0x1000, 0xC7)    // byte extended prefix
	bus.write8(0x1001, 0x00)    // extended reg code = A
	bus.write8(0x1002, 0xC8)    // ADD r,# (regOps)
	bus.write8(0x1003, 0x05)    // imm8 = 5
	c.reg.WriteReg8(0x00, 0x10) // A = 0x10
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(0x00), 0x15)
}

// --- Cycle verification for memory operations ---

func TestADD_RegMem_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x81) // byte (R0), ADD R1,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("ADD R,(mem) byte cycles = %d, want 4", cycles)
	}
}

func TestADD_MemReg_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x89)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	c.reg.WriteReg8(r8From3bit[1], 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("ADD (mem),R byte cycles = %d, want 6", cycles)
	}
}

func TestADD_MemImm_Cycles_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x38, 0x00, 0x00) // word, ADD<W> (mem),#
	c.reg.WriteReg32(0, 0x2000)
	bus.Write(Word, 0x2000, 0)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("ADD<W> (mem),# word cycles = %d, want 8", cycles)
	}
}

func TestINC_Mem_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x61)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write(Word, 0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("INC<W> (mem) cycles = %d, want 6", cycles)
	}
}

func TestDEC_Mem_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x69)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write(Word, 0x2000, 1)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("DEC<W> (mem) cycles = %d, want 6", cycles)
	}
}

func TestCP_MemReg_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0xF8)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	c.reg.WriteReg8(r8From3bit[0], 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("CP (mem),R byte cycles = %d, want 4", cycles)
	}
}

// --- Long size arithmetic ---

func TestADD_Long_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0x81)
	c.reg.WriteReg32(0, 0)
	c.reg.WriteReg32(1, 0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("ADD R,r long cycles = %d, want 2", cycles)
	}
}

func TestSUB_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xA1) // long SUB R1,r
	c.reg.WriteReg32(0, 0x10000000)   // prefix reg
	c.reg.WriteReg32(1, 0x30000000)   // R1
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x20000000)
}

func TestSUB_Long_CarryFlag(t *testing.T) {
	// SUB.L R0,R1 where R1 > R0 should set carry (borrow).
	// 0xE8 = long reg 0 (XWA) prefix, 0xA1 = SUB R1,r
	c, _ := setupRegOp(t, 0xE8, 0xA1)
	c.reg.WriteReg32(0, 0x00000001) // prefix reg (subtrahend)
	c.reg.WriteReg32(1, 0x00000000) // R1 (minuend): 0 - 1 = borrow
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0xFFFFFFFF)
	checkFlags(t, c, 1, 0, -1, 0, 1, 1) // S=1, Z=0, V=0, N=1, C=1
}

func TestSUB_Long_NoCarry(t *testing.T) {
	// SUB.L where minuend > subtrahend should NOT set carry.
	c, _ := setupRegOp(t, 0xE8, 0xA1)
	c.reg.WriteReg32(0, 0x00000001) // subtrahend
	c.reg.WriteReg32(1, 0x00000002) // minuend: 2 - 1 = 1, no borrow
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x00000001)
	checkFlags(t, c, 0, 0, -1, 0, 1, 0) // S=0, Z=0, V=0, N=1, C=0
}

func TestADD_Long_CarryFlag(t *testing.T) {
	// ADD.L where sum overflows 32 bits should set carry.
	// 0xE8 = long reg 0 (XWA) prefix, 0x81 = ADD R1,r
	c, _ := setupRegOp(t, 0xE8, 0x81)
	c.reg.WriteReg32(0, 0x80000000) // prefix reg
	c.reg.WriteReg32(1, 0x80000000) // R1: 0x80000000 + 0x80000000 = overflow
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x00000000)
	checkFlags(t, c, 0, 1, -1, 1, 0, 1) // S=0, Z=1, V=1, N=0, C=1
}

func TestADD_Long_NoCarry(t *testing.T) {
	// ADD.L where sum fits in 32 bits should NOT set carry.
	c, _ := setupRegOp(t, 0xE8, 0x81)
	c.reg.WriteReg32(0, 0x00000001)
	c.reg.WriteReg32(1, 0x00000002) // 2 + 1 = 3, no carry
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x00000003)
	checkFlags(t, c, 0, 0, -1, 0, 0, 0) // S=0, Z=0, V=0, N=0, C=0
}

func TestADD_RegImm_Long_Cycles(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xC8, 0x01, 0x00, 0x00, 0x00) // long ADD r,#
	c.reg.WriteReg32(0, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("ADD r,# long cycles = %d, want 6", cycles)
	}
}

func TestINC_Word_NoFlagsChange(t *testing.T) {
	// INC #3,r with word operand should NOT change any flags.
	// Prefix 0xD8 = word reg 0 (WA), op 0x61 = INC 1
	c, _ := setupRegOp(t, 0xD8, 0x61)
	c.reg.WriteReg16(0, 0xFFFF) // will wrap to 0, but flags must not change
	c.setFlag(flagS, true)
	c.setFlag(flagZ, false)
	c.setFlag(flagH, true)
	c.setFlag(flagV, true)
	c.setFlag(flagN, true)
	c.setFlag(flagC, true)
	oldFlags := c.flags()
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0000)
	if c.flags() != oldFlags {
		t.Errorf("INC word flags = 0x%02X, want 0x%02X (unchanged)", c.flags(), oldFlags)
	}
}

func TestINC_Long_NoFlagsChange(t *testing.T) {
	// INC #3,r with long operand should NOT change any flags.
	// Prefix 0xE8 = long reg 0 (XWA), op 0x61 = INC 1
	c, _ := setupRegOp(t, 0xE8, 0x61)
	c.reg.WriteReg32(0, 0x00000010)
	c.setFlag(flagS, true)
	c.setFlag(flagZ, true)
	c.setFlag(flagC, true)
	oldFlags := c.flags()
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0x00000011)
	if c.flags() != oldFlags {
		t.Errorf("INC long flags = 0x%02X, want 0x%02X (unchanged)", c.flags(), oldFlags)
	}
}

func TestDEC_Word_NoFlagsChange(t *testing.T) {
	// DEC #3,r with word operand should NOT change any flags.
	// Prefix 0xD8 = word reg 0 (WA), op 0x69 = DEC 1
	c, _ := setupRegOp(t, 0xD8, 0x69)
	c.reg.WriteReg16(0, 0x0001) // will become 0, but flags must not change
	c.setFlag(flagS, true)
	c.setFlag(flagH, true)
	c.setFlag(flagV, true)
	c.setFlag(flagC, true)
	oldFlags := c.flags()
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0000)
	if c.flags() != oldFlags {
		t.Errorf("DEC word flags = 0x%02X, want 0x%02X (unchanged)", c.flags(), oldFlags)
	}
}

func TestDEC_Long_NoFlagsChange(t *testing.T) {
	// DEC #3,r with long operand should NOT change any flags.
	// Prefix 0xE8 = long reg 0 (XWA), op 0x69 = DEC 1
	c, _ := setupRegOp(t, 0xE8, 0x69)
	c.reg.WriteReg32(0, 0x00000010)
	c.setFlag(flagS, true)
	c.setFlag(flagZ, true)
	c.setFlag(flagN, true)
	oldFlags := c.flags()
	c.Step()
	checkReg32(t, "XWA", c.reg.ReadReg32(0), 0x0000000F)
	if c.flags() != oldFlags {
		t.Errorf("DEC long flags = 0x%02X, want 0x%02X (unchanged)", c.flags(), oldFlags)
	}
}

func TestDAA_HFlag_ActualHalfCarry(t *testing.T) {
	// After ADD: val=0x10, H was set (e.g. 0x09+0x07=0x10).
	// DAA adds 0x06 correction. 0x10 + 0x06 = 0x16.
	// Actual half-carry of adjustment: (0x10 ^ 0x06 ^ 0x16) & 0x10 = 0.
	// H flag should be 0.
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0x10)
	c.setFlag(flagN, false)
	c.setFlag(flagH, true)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x16)
	// H should be 0: adjustment 0x10+0x06=0x16 has no half-carry
	checkFlags(t, c, 0, 0, 0, -1, 0, 0)
}

func TestDAA_HFlag_WithHalfCarry(t *testing.T) {
	// After ADD: val=0x0A, no H, no C.
	// Low nibble > 9 so correction = 0x06. 0x0A + 0x06 = 0x10.
	// Actual half-carry: (0x0A ^ 0x06 ^ 0x10) & 0x10 = (0x1C) & 0x10 = 0x10 != 0.
	// H flag should be 1.
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0x0A)
	c.setFlag(flagN, false)
	c.setFlag(flagH, false)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x10)
	// H should be 1: adjustment 0x0A+0x06=0x10 carries from bit 3 to bit 4
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

func TestDAA_Sub_HFlag(t *testing.T) {
	// After SUB: val=0x00, H=1, C=0, N=1.
	// Correction = 0x06 (subtract). 0x00 - 0x06 = 0xFA.
	// Actual half-carry (borrow): (0x00 ^ 0x06 ^ 0xFA) & 0x10 = 0xFC & 0x10 = 0x10 != 0.
	// H flag should be 1.
	c, _ := setupRegOp(t, 0xC8, 0x10)
	c.reg.WriteReg8(r8From3bit[0], 0x00)
	c.setFlag(flagN, true)
	c.setFlag(flagH, true)
	c.setFlag(flagC, false)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xFA)
	checkFlags(t, c, 1, 0, 1, -1, 1, 0)
}

// --- Extended register MUL/MULS/DIV/DIVS tests ---
// These test the C7/D7 (extended register prefix) paths which use full
// 8-bit register codes. The bugs are that bank information is lost when
// computing the destination register for the double-width result.

func TestMUL_RegImm_Byte_Extended_Bank1(t *testing.T) {
	// C7 (byte extended prefix), 0x10 (bank 1 byte 0 = A1), 0x08 (MUL rr,#), 5
	// Current bank is 0. Result should go to bank 1 WA (code 0x10), not bank 0.
	c, _ := setupRegOp(t, 0xC7, 0x10, 0x08, 5)
	c.reg.Bank[1].XWA = 0               // clear bank 1
	c.reg.Bank[0].XWA = 0xDEAD0000      // sentinel in bank 0
	c.reg.Bank[1].XWA = uint32(10) << 0 // A1 = 10 (byte 0)
	c.Step()
	// 10 * 5 = 50, result in bank 1 WA (low word of XWA1)
	got := uint16(c.reg.Bank[1].XWA)
	if got != 50 {
		t.Errorf("Bank1 WA = %d, want 50", got)
	}
	// Bank 0 upper bits should be untouched
	if c.reg.Bank[0].XWA&0xFFFF0000 != 0xDEAD0000 {
		t.Errorf("Bank0 XWA upper = 0x%08X, want 0xDEAD0000", c.reg.Bank[0].XWA&0xFFFF0000)
	}
}

func TestMUL_RegImm_Word_Extended_Bank1(t *testing.T) {
	// D7 (word extended prefix), 0x14 (bank 1 XBC low word), 0x08 (MUL rr,#), imm16
	// Result should go to bank 1 XBC (32-bit), not XIX.
	c, _ := setupRegOp(t, 0xD7, 0x14, 0x08, 0x64, 0x00) // imm = 100
	c.reg.Bank[1].XBC = 200                             // BC1 = 200
	c.reg.XIX = 0xDEADBEEF                              // sentinel
	c.Step()
	// 200 * 100 = 20000
	if c.reg.Bank[1].XBC != 20000 {
		t.Errorf("Bank1 XBC = %d, want 20000", c.reg.Bank[1].XBC)
	}
	if c.reg.XIX != 0xDEADBEEF {
		t.Errorf("XIX = 0x%08X, should be untouched", c.reg.XIX)
	}
}

func TestMULS_RegImm_Byte_Extended_Bank1(t *testing.T) {
	// C7, 0x10 (bank 1 A), 0x09 (MULS rr,#), 0xFE (-2)
	c, _ := setupRegOp(t, 0xC7, 0x10, 0x09, 0xFE)
	c.reg.Bank[1].XWA = 3 // A1 = 3
	c.reg.Bank[0].XWA = 0xDEAD0000
	c.Step()
	// 3 * -2 = -6 = 0xFFFA
	got := uint16(c.reg.Bank[1].XWA)
	if got != 0xFFFA {
		t.Errorf("Bank1 WA = 0x%04X, want 0xFFFA", got)
	}
}

func TestMULS_RegImm_Word_Extended_Bank1(t *testing.T) {
	// D7, 0x14 (bank 1 BC word), 0x09 (MULS rr,#), imm16
	c, _ := setupRegOp(t, 0xD7, 0x14, 0x09, 0x9C, 0xFF) // imm = -100 (0xFF9C)
	c.reg.Bank[1].XBC = 200
	c.reg.XIX = 0xDEADBEEF
	c.Step()
	// 200 * -100 = -20000 = 0xFFFFB1E0
	if c.reg.Bank[1].XBC != 0xFFFFB1E0 {
		t.Errorf("Bank1 XBC = 0x%08X, want 0xFFFFB1E0", c.reg.Bank[1].XBC)
	}
	if c.reg.XIX != 0xDEADBEEF {
		t.Errorf("XIX = 0x%08X, should be untouched", c.reg.XIX)
	}
}

func TestDIV_RegImm_Byte_Extended_Bank1(t *testing.T) {
	// C7, 0x10 (bank 1 byte 0 = A1), 0x0A (DIV rr,#), 7
	// Dividend is in the parent word register: bank 1 WA
	// Result goes back to bank 1 WA
	c, _ := setupRegOp(t, 0xC7, 0x10, 0x0A, 7)
	c.reg.Bank[1].XWA = 100        // WA1 = 100 (dividend)
	c.reg.Bank[0].XWA = 0xDEADDEAD // sentinel
	c.Step()
	// 100 / 7 = 14 remainder 2 => WA = (2 << 8) | 14 = 0x020E
	got := uint16(c.reg.Bank[1].XWA)
	if got != 0x020E {
		t.Errorf("Bank1 WA = 0x%04X, want 0x020E", got)
	}
	if c.reg.Bank[0].XWA != 0xDEADDEAD {
		t.Errorf("Bank0 XWA = 0x%08X, should be untouched", c.reg.Bank[0].XWA)
	}
}

func TestDIV_RegImm_Word_Extended_Bank1(t *testing.T) {
	// D7, 0x14 (bank 1 BC word), 0x0A (DIV rr,#), imm16
	// Dividend is in parent long register: bank 1 XBC
	// Result goes back to bank 1 XBC
	c, _ := setupRegOp(t, 0xD7, 0x14, 0x0A, 0x07, 0x00) // imm = 7
	c.reg.Bank[1].XBC = 100
	c.reg.XIX = 0xDEADBEEF
	c.Step()
	// 100 / 7 = 14 remainder 2 => XBC = (2 << 16) | 14 = 0x0002000E
	if c.reg.Bank[1].XBC != 0x0002000E {
		t.Errorf("Bank1 XBC = 0x%08X, want 0x0002000E", c.reg.Bank[1].XBC)
	}
	if c.reg.XIX != 0xDEADBEEF {
		t.Errorf("XIX = 0x%08X, should be untouched", c.reg.XIX)
	}
}

func TestDIVS_RegImm_Byte_Extended_Bank1(t *testing.T) {
	// C7, 0x10 (bank 1 A), 0x0B (DIVS rr,#), 7
	c, _ := setupRegOp(t, 0xC7, 0x10, 0x0B, 7)
	c.reg.Bank[1].XWA = uint32(0xFF9C) // WA1 = -100 as int16 (0xFF9C)
	c.reg.Bank[0].XWA = 0xDEADDEAD
	c.Step()
	// -100 / 7 = -14 remainder -2
	got := c.reg.Bank[1].XWA & 0xFFFF
	quotient := int8(uint8(got))
	remainder := int8(uint8(got >> 8))
	if quotient != -14 {
		t.Errorf("quotient = %d, want -14", quotient)
	}
	if remainder != -2 {
		t.Errorf("remainder = %d, want -2", remainder)
	}
	if c.reg.Bank[0].XWA != 0xDEADDEAD {
		t.Errorf("Bank0 XWA = 0x%08X, should be untouched", c.reg.Bank[0].XWA)
	}
}

func TestDIVS_RegImm_Word_Extended_Bank1(t *testing.T) {
	// D7, 0x14 (bank 1 BC word), 0x0B (DIVS rr,#), imm16
	c, _ := setupRegOp(t, 0xD7, 0x14, 0x0B, 0x07, 0x00) // imm = 7
	c.reg.Bank[1].XBC = 0xFFFFFF9C                      // XBC1 = -100 as int32
	c.reg.XIX = 0xDEADBEEF
	c.Step()
	// -100 / 7 = -14 remainder -2
	got := c.reg.Bank[1].XBC
	quotient := int16(uint16(got))
	remainder := int16(uint16(got >> 16))
	if quotient != -14 {
		t.Errorf("quotient = %d, want -14", quotient)
	}
	if remainder != -2 {
		t.Errorf("remainder = %d, want -2", remainder)
	}
	if c.reg.XIX != 0xDEADBEEF {
		t.Errorf("XIX = 0x%08X, should be untouched", c.reg.XIX)
	}
}
