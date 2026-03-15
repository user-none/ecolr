package tlcs900h

import "testing"

// --- RLC tests ---

func TestRLC_Byte_Imm(t *testing.T) {
	// RLC #1, W: prefix 0xC8 (byte reg0=W), op2 0xE8, imm count=1
	c, _ := setupRegOp(t, 0xC8, 0xE8, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x81) // W = 1000_0001
	c.Step()
	// Rotate left 1: MSB(1)->LSB, result = 0000_0011 = 0x03, C=1 (old MSB)
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x03)
	// S=0, Z=0, H=0, V=parity(0x03=2 bits, even)=1, N=0, C=1
	checkFlags(t, c, 0, 0, 0, 1, 0, 1)
}

func TestRLC_Word_Imm(t *testing.T) {
	// RLC #1, RW0: prefix 0xD8 (word reg0), op2 0xE8, count=1
	c, _ := setupRegOp(t, 0xD8, 0xE8, 0x01)
	c.reg.WriteReg16(0, 0x8001)
	c.Step()
	// 0x8001 rotated left 1 = 0x0003, C=1
	checkReg16(t, "RW0", c.reg.ReadReg16(0), 0x0003)
	checkFlags(t, c, 0, 0, 0, -1, 0, 1)
}

func TestRLC_Byte_CountA(t *testing.T) {
	// RLC A, W: prefix 0xC8, op2 0xF8
	c, _ := setupRegOp(t, 0xC8, 0xF8)
	c.reg.WriteReg8(r8From3bit[1], 0x02) // A = 2 (count)
	c.reg.WriteReg8(r8From3bit[0], 0x81) // W = 0x81
	c.Step()
	// Rotate left 2: 0x81 -> 0x03 -> 0x06, C=0
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x06)
	checkFlags(t, c, 0, 0, 0, -1, 0, 0)
}

func TestRLC_Mem(t *testing.T) {
	// RLC (mem): src prefix 0x80 (byte indirect R0), op2 0x78
	c, bus := setupMemOp(t, 0x80, 0x78)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x81)
	cycles := c.Step()
	got := bus.Read8(0x2000)
	if got != 0x03 {
		t.Errorf("RLC (mem) = 0x%02X, want 0x03", got)
	}
	checkFlags(t, c, 0, 0, 0, 1, 0, 1)
	if cycles != 6 {
		t.Errorf("RLC (mem) cycles = %d, want 6", cycles)
	}
}

// --- RRC tests ---

func TestRRC_Byte_Imm(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xE9, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x81) // W = 1000_0001
	c.Step()
	// Rotate right 1: LSB(1)->MSB, result = 1100_0000 = 0xC0, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xC0)
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

func TestRRC_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x79)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x01) // 0000_0001
	c.Step()
	got := bus.Read8(0x2000)
	// Rotate right 1: 0x01 -> 0x80, C=1
	if got != 0x80 {
		t.Errorf("RRC (mem) = 0x%02X, want 0x80", got)
	}
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

// --- RL tests ---

func TestRL_Byte_Imm_CarryIn(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEA, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x80) // W = 1000_0000
	c.setFlags(flagC)                    // C=1
	c.Step()
	// RL: C(1)->LSB, MSB(1)->C, result = 0000_0001 = 0x01, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x01)
	checkFlags(t, c, 0, 0, 0, -1, 0, 1)
}

func TestRL_Byte_Imm_NoCarryIn(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEA, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x80) // W = 1000_0000
	c.setFlags(0)                        // C=0
	c.Step()
	// RL: C(0)->LSB, MSB(1)->C, result = 0000_0000 = 0x00, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	checkFlags(t, c, 0, 1, 0, 1, 0, 1) // Z=1, V=parity(0)=even=1
}

func TestRL_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7A)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x55) // 0101_0101
	c.setFlags(flagC)        // C=1
	c.Step()
	got := bus.Read8(0x2000)
	// RL: C(1)->LSB, MSB(0)->C, result = 1010_1011 = 0xAB, C=0
	if got != 0xAB {
		t.Errorf("RL (mem) = 0x%02X, want 0xAB", got)
	}
	checkFlags(t, c, 1, 0, 0, -1, 0, 0)
}

// --- RR tests ---

func TestRR_Byte_Imm_CarryIn(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEB, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x01) // W = 0000_0001
	c.setFlags(flagC)                    // C=1
	c.Step()
	// RR: C(1)->MSB, LSB(1)->C, result = 1000_0000 = 0x80, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x80)
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

func TestRR_Byte_Imm_NoCarryIn(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEB, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x01) // W = 0000_0001
	c.setFlags(0)                        // C=0
	c.Step()
	// RR: C(0)->MSB, LSB(1)->C, result = 0000_0000 = 0x00, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	checkFlags(t, c, 0, 1, 0, 1, 0, 1) // Z=1, V=parity(0)=1
}

func TestRR_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7B)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xAA) // 1010_1010
	c.setFlags(flagC)        // C=1
	c.Step()
	got := bus.Read8(0x2000)
	// RR: C(1)->MSB, LSB(0)->C, result = 1101_0101 = 0xD5, C=0
	if got != 0xD5 {
		t.Errorf("RR (mem) = 0x%02X, want 0xD5", got)
	}
	checkFlags(t, c, 1, 0, 0, -1, 0, 0)
}

// --- SLA tests ---

func TestSLA_Byte_Imm(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEC, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0xC1) // W = 1100_0001
	c.Step()
	// SLA: shift left 1, LSB=0, C=MSB(1), result = 1000_0010 = 0x82
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x82)
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

func TestSLA_Word_Imm(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xEC, 0x01)
	c.reg.WriteReg16(0, 0x8001)
	c.Step()
	// 0x8001 << 1 = 0x0002, C=1
	checkReg16(t, "RW0", c.reg.ReadReg16(0), 0x0002)
	checkFlags(t, c, 0, 0, 0, -1, 0, 1)
}

func TestSLA_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7C)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x55) // 0101_0101
	c.Step()
	got := bus.Read8(0x2000)
	// SLA: 0x55 << 1 = 0xAA, C=0
	if got != 0xAA {
		t.Errorf("SLA (mem) = 0x%02X, want 0xAA", got)
	}
	checkFlags(t, c, 1, 0, 0, -1, 0, 0)
}

// --- SRA tests ---

func TestSRA_Byte_Imm(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xED, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x81) // W = 1000_0001
	c.Step()
	// SRA: shift right 1, MSB preserved, C=LSB(1)
	// 1000_0001 >> 1 = 1100_0000 = 0xC0, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xC0)
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

func TestSRA_Byte_Positive(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xED, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x42) // W = 0100_0010
	c.Step()
	// SRA: 0100_0010 >> 1 = 0010_0001 = 0x21, C=0
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x21)
	checkFlags(t, c, 0, 0, 0, -1, 0, 0)
}

func TestSRA_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7D)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x80) // 1000_0000
	c.Step()
	got := bus.Read8(0x2000)
	// SRA: 1000_0000 >> 1 = 1100_0000 = 0xC0, C=0
	if got != 0xC0 {
		t.Errorf("SRA (mem) = 0x%02X, want 0xC0", got)
	}
	checkFlags(t, c, 1, 0, 0, -1, 0, 0)
}

// --- SLL tests ---

func TestSLL_Byte_Imm(t *testing.T) {
	// SLL should be identical to SLA
	c, _ := setupRegOp(t, 0xC8, 0xEE, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0xC1)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x82)
	checkFlags(t, c, 1, 0, 0, -1, 0, 1)
}

func TestSLL_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7E)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x55)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xAA {
		t.Errorf("SLL (mem) = 0x%02X, want 0xAA", got)
	}
}

// --- SRL tests ---

func TestSRL_Byte_Imm(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEF, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x81) // W = 1000_0001
	c.Step()
	// SRL: shift right 1, MSB=0, C=LSB(1)
	// 1000_0001 >> 1 = 0100_0000 = 0x40, C=1
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x40)
	checkFlags(t, c, 0, 0, 0, -1, 0, 1)
}

func TestSRL_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7F)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x01)
	c.Step()
	got := bus.Read8(0x2000)
	// SRL: 0x01 >> 1 = 0x00, C=1
	if got != 0x00 {
		t.Errorf("SRL (mem) = 0x%02X, want 0x00", got)
	}
	checkFlags(t, c, 0, 1, 0, 1, 0, 1)
}

// --- Count=0 means 16 ---

func TestRLC_Count0_Means16(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xE8, 0x00) // count=0 in immediate
	c.reg.WriteReg8(r8From3bit[0], 0xAB)
	c.Step()
	// 16 rotations of a byte = 2 full rotations, result unchanged
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xAB)
}

func TestRLC_CountA_Zero_Means16(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xF8)
	c.reg.WriteReg8(r8From3bit[1], 0x00) // A = 0 -> count = 16
	c.reg.WriteReg8(r8From3bit[0], 0xAB) // W = 0xAB
	c.Step()
	// 16 rotations of byte = 2 full rotations, unchanged
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xAB)
}

// --- RLD tests ---

func TestRLD(t *testing.T) {
	// RLD: src prefix 0x83 (byte indirect R3=XHL), op2 0x06
	c, bus := setupMemOp(t, 0x83, 0x06)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x12) // A = 0x12
	bus.write8(0x2000, 0x34)             // mem = 0x34
	c.Step()
	// RLD: A[3:0] <- mem[7:4]=3, mem[7:4] <- mem[3:0]=4, mem[3:0] <- old A[3:0]=2
	// A = 0x13, mem = 0x42
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x13)
	got := bus.Read8(0x2000)
	if got != 0x42 {
		t.Errorf("RLD mem = 0x%02X, want 0x42", got)
	}
}

func TestRLD_Flags(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x06)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x00) // A = 0x00
	bus.write8(0x2000, 0x00)             // mem = 0x00
	c.setFlags(flagC)                    // Set carry to verify it's preserved
	c.Step()
	// A = 0x00, Z=1, S=0, H=0, V=parity(0)=1, N=0, C=1 (unchanged)
	checkFlags(t, c, 0, 1, 0, 1, 0, 1)
}

func TestRLD_CarryUnchanged(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x06)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	bus.write8(0x2000, 0x23)
	c.setFlags(0) // C=0
	c.Step()
	// A = 0x02, C should remain 0
	checkFlags(t, c, 0, 0, 0, -1, 0, 0)
}

func TestRLD_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x06)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	bus.write8(0x2000, 0x00)
	cycles := c.Step()
	if cycles != 14 {
		t.Errorf("RLD cycles = %d, want 14", cycles)
	}
}

// --- RRD tests ---

func TestRRD(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x07)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x12) // A = 0x12
	bus.write8(0x2000, 0x34)             // mem = 0x34
	c.Step()
	// RRD: A[3:0] <- mem[3:0]=4, mem[3:0] <- mem[7:4]=3, mem[7:4] <- old A[3:0]=2
	// A = 0x14, mem = 0x23
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x14)
	got := bus.Read8(0x2000)
	if got != 0x23 {
		t.Errorf("RRD mem = 0x%02X, want 0x23", got)
	}
}

func TestRRD_Flags(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x07)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	bus.write8(0x2000, 0x00)
	c.setFlags(flagC) // C should be preserved
	c.Step()
	checkFlags(t, c, 0, 1, 0, 1, 0, 1)
}

func TestRRD_CarryUnchanged(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x07)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	bus.write8(0x2000, 0x23)
	c.setFlags(0) // C=0
	c.Step()
	// A[3:0] <- mem[3:0]=3 -> A = 0x03, C=0
	checkFlags(t, c, 0, 0, 0, -1, 0, 0)
}

func TestRRD_Cycles(t *testing.T) {
	c, bus := setupMemOp(t, 0x83, 0x07)
	c.reg.WriteReg32(3, 0x2000)
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	bus.write8(0x2000, 0x00)
	cycles := c.Step()
	if cycles != 14 {
		t.Errorf("RRD cycles = %d, want 14", cycles)
	}
}

// --- Cycle count tests ---

func TestRLC_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xE8, 0x03) // count=3
	c.reg.WriteReg8(r8From3bit[0], 0x00)
	cycles := c.Step()
	// B/W: 3+n = 3+3 = 6
	if cycles != 6 {
		t.Errorf("RLC byte imm cycles = %d, want 6", cycles)
	}
}

func TestRLC_Cycles_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xE8, 0x02) // word, count=2
	c.reg.WriteReg16(0, 0x00)
	cycles := c.Step()
	// W: 3+n = 3+2 = 5
	if cycles != 5 {
		t.Errorf("RLC word imm cycles = %d, want 5", cycles)
	}
}

func TestRLC_Cycles_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xE8, 0x04) // long reg0, count=4
	c.reg.WriteReg32(0, 0x00)
	cycles := c.Step()
	// L: 4+n = 4+4 = 8
	if cycles != 8 {
		t.Errorf("RLC long imm cycles = %d, want 8", cycles)
	}
}

func TestSRL_Cycles_Mem(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x7F) // byte mem
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("SRL mem cycles = %d, want 6", cycles)
	}
}

func TestSRL_Cycles_Mem_Word(t *testing.T) {
	// Word memory: src prefix 0x90 (word indirect R0), op2 0x7F
	c, bus := setupMemOp(t, 0x90, 0x7F)
	c.reg.WriteReg32(0, 0x2000)
	bus.write16LE(0x2000, 0x0000)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("SRL mem word cycles = %d, want 6", cycles)
	}
}

// --- POP<W>(mem) works via dst prefix ---

func TestPOP_Mem_Word_StillWorks(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Word, 0xCAFE)
	bus.write8(0x1000, 0xB0) // dst indirect R0
	bus.write8(0x1001, 0x06) // POP<W>(mem)
	c.reg.WriteReg32(0, 0x2000)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0xCAFE {
		t.Errorf("POP<W>(mem) after wrap = 0x%04X, want 0xCAFE", got)
	}
}

func TestPOP_Mem_Word_Cycles_StillWorks(t *testing.T) {
	pc := uint32(0x1000)
	c, bus := newTestCPU(t, pc)
	c.push(Word, 0)
	bus.write8(0x1000, 0xB0)
	bus.write8(0x1001, 0x06)
	c.reg.WriteReg32(0, 0x2000)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("POP<W>(mem) cycles after wrap = %d, want 7", cycles)
	}
}

// --- Flag detail tests ---

func TestSLA_FlagsZero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xEC, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0x80) // shift left: 0x80 -> 0x00
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	// S=0, Z=1, H=0, V=parity(0)=1(even), N=0, C=1
	checkFlags(t, c, 0, 1, 0, 1, 0, 1)
}

func TestSRL_FlagsSign(t *testing.T) {
	// Shift right logical can't produce a sign bit, MSB always 0
	c, _ := setupRegOp(t, 0xC8, 0xEF, 0x01)
	c.reg.WriteReg8(r8From3bit[0], 0xFF) // 0xFF >> 1 = 0x7F
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x7F)
	// S=0 (MSB=0), Z=0, H=0, V=parity(0x7F=7 bits, odd)=0, N=0, C=1
	checkFlags(t, c, 0, 0, 0, 0, 0, 1)
}

func TestSRA_SignPreserved_MultiCount(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xED, 0x04)
	c.reg.WriteReg8(r8From3bit[0], 0x80) // 1000_0000
	c.Step()
	// SRA 4 times: 1000_0000 -> 1100_0000 -> 1110_0000 -> 1111_0000 -> 1111_1000 = 0xF8
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xF8)
	checkFlags(t, c, 1, 0, 0, -1, 0, 0) // C=0 (last bit shifted was 0)
}
