package tlcs900h

import "testing"

// --- AND R,r ---

func TestAND_RegReg_Byte(t *testing.T) {
	// Prefix 0xC9 = byte reg1(A), op2 = 0xC0 = AND R0(W),r
	c, _ := setupRegOp(t, 0xC9, 0xC0)
	c.reg.WriteReg8(r8From3bit[1], 0x3C) // A = 0x3C
	c.reg.WriteReg8(r8From3bit[0], 0x0F) // W = 0x0F
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x0C)
	// S=0, Z=0, H=1, V=parity(0x0C=00001100, 2 bits, even parity), N=0, C=0
	checkFlags(t, c, 0, 0, 1, 1, 0, 0)
}

func TestAND_RegReg_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xC1) // word reg0(XWA.W), AND R1,r
	c.reg.WriteReg16(0, 0xFF00)       // WA = 0xFF00
	c.reg.WriteReg16(1, 0x0FF0)       // BC = 0x0FF0
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0x0F00)
	// S=0, Z=0, H=1, V=parity(0x0F00: 4 bits, even)=1, N=0, C=0
	checkFlags(t, c, 0, 0, 1, 1, 0, 0)
}

func TestAND_RegReg_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xC1) // long reg0(XWA), AND R1,r
	c.reg.WriteReg32(0, 0xFFFF0000)
	c.reg.WriteReg32(1, 0x12345678)
	c.Step()
	checkReg32(t, "XBC", c.reg.ReadReg32(1), 0x12340000)
	checkFlags(t, c, 0, 0, 1, 0, 0, 0)
}

func TestAND_RegReg_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC0) // byte
	c.reg.WriteReg8(r8From3bit[1], 0xF0)
	c.reg.WriteReg8(r8From3bit[0], 0x0F)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	// Z=1, V=parity(0)=1 (even parity)
	checkFlags(t, c, 0, 1, 1, 1, 0, 0)
}

func TestAND_RegReg_Sign(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC0) // byte
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.reg.WriteReg8(r8From3bit[0], 0x80)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x80)
	// S=1, V=parity(0x80)=0 (1 bit, odd)
	checkFlags(t, c, 1, 0, 1, 0, 0, 0)
}

// --- AND r,# ---

func TestAND_RegImm_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCC, 0x0F) // AND A,#0x0F
	c.reg.WriteReg8(r8From3bit[1], 0xAB)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0B)
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

func TestAND_RegImm_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xCC, 0xFF, 0x00) // AND WA,#0x00FF
	c.reg.WriteReg16(0, 0x1234)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0034)
	checkFlags(t, c, 0, 0, 1, 0, 0, 0)
}

// --- AND R,(mem) ---

func TestAND_RegMem_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xC1) // byte indirect R1, AND R1(A),(mem)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0x0F)
	c.reg.WriteReg8(r8From3bit[1], 0x3C) // A
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x0C)
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

// --- AND (mem),R ---

func TestAND_MemReg_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xC9) // byte indirect R1, AND (mem),R1(A)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0xFF)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x0F {
		t.Errorf("(mem) = 0x%02X, want 0x0F", got)
	}
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

// --- AND (mem),# ---

func TestAND_MemImm_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3C, 0x0F) // byte indirect R0, AND (mem),#0x0F
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xAB)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x0B {
		t.Errorf("(mem) = 0x%02X, want 0x0B", got)
	}
	checkFlags(t, c, 0, 0, 1, -1, 0, 0)
}

func TestAND_MemImm_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x3C, 0xFF, 0x00) // word indirect R0, AND<W> (mem),#0x00FF
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0x1234)
	c.Step()
	got := bus.Read16(0x2000)
	if got != 0x0034 {
		t.Errorf("(mem) = 0x%04X, want 0x0034", got)
	}
}

// --- XOR R,r ---

func TestXOR_RegReg_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xD0)
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.reg.WriteReg8(r8From3bit[0], 0x0F)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xF0)
	// S=1, Z=0, H=0, V=parity(0xF0=4 bits, even)=1, N=0, C=0
	checkFlags(t, c, 1, 0, 0, 1, 0, 0)
}

func TestXOR_RegReg_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xD1) // word, XOR R1,r
	c.reg.WriteReg16(0, 0xAAAA)
	c.reg.WriteReg16(1, 0x5555)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0xFFFF)
	// S=1, Z=0, H=0, V=parity(0xFFFF: 16 bits, even)=1, N=0, C=0
	checkFlags(t, c, 1, 0, 0, 1, 0, 0)
}

func TestXOR_RegReg_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xD0) // byte
	c.reg.WriteReg8(r8From3bit[1], 0xAB)
	c.reg.WriteReg8(r8From3bit[0], 0xAB)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	checkFlags(t, c, 0, 1, 0, 1, 0, 0) // Z=1, V=parity(0)=1
}

// --- XOR r,# ---

func TestXOR_RegImm_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCD, 0xFF) // XOR A,#0xFF
	c.reg.WriteReg8(r8From3bit[1], 0x55)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAA)
	// parity(0xAA=10101010, 4 bits, even)=1
	checkFlags(t, c, 1, 0, 0, 1, 0, 0)
}

// --- XOR R,(mem) ---

func TestXOR_RegMem_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xD1)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0xFF)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xF0)
}

// --- XOR (mem),R ---

func TestXOR_MemReg_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xD9)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0xAA)
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x55 {
		t.Errorf("(mem) = 0x%02X, want 0x55", got)
	}
}

// --- XOR (mem),# ---

func TestXOR_MemImm_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3D, 0xFF) // XOR (mem),#0xFF
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x55)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xAA {
		t.Errorf("(mem) = 0x%02X, want 0xAA", got)
	}
}

// --- OR R,r ---

func TestOR_RegReg_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xE0)
	c.reg.WriteReg8(r8From3bit[1], 0xF0)
	c.reg.WriteReg8(r8From3bit[0], 0x0F)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0xFF)
	// S=1, H=0, V=parity(0xFF=8 bits, even)=1, N=0, C=0
	checkFlags(t, c, 1, 0, 0, 1, 0, 0)
}

func TestOR_RegReg_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xE1) // word, OR R1,r
	c.reg.WriteReg16(0, 0x00FF)
	c.reg.WriteReg16(1, 0xFF00)
	c.Step()
	checkReg16(t, "BC", c.reg.ReadReg16(1), 0xFFFF)
	// S=1, Z=0, H=0, V=parity(0xFFFF: 16 bits, even)=1, N=0, C=0
	checkFlags(t, c, 1, 0, 0, 1, 0, 0)
}

func TestOR_RegReg_Zero(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xE0) // byte
	c.reg.WriteReg8(r8From3bit[1], 0x00)
	c.reg.WriteReg8(r8From3bit[0], 0x00)
	c.Step()
	checkReg8(t, "W", c.reg.ReadReg8(r8From3bit[0]), 0x00)
	checkFlags(t, c, 0, 1, 0, 1, 0, 0) // Z=1, V=parity(0)=1
}

// --- OR r,# ---

func TestOR_RegImm_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xCE, 0x0F) // OR A,#0x0F
	c.reg.WriteReg8(r8From3bit[1], 0xA0)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xAF)
}

// --- OR R,(mem) ---

func TestOR_RegMem_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xE1)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0xF0)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0xFF)
}

// --- OR (mem),R ---

func TestOR_MemReg_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x81, 0xE9)
	c.reg.WriteReg32(1, 0x2000)
	bus.write8(0x2000, 0xF0)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xFF {
		t.Errorf("(mem) = 0x%02X, want 0xFF", got)
	}
}

// --- OR (mem),# ---

func TestOR_MemImm_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3E, 0x0F) // OR (mem),#0x0F
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0xA0)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0xAF {
		t.Errorf("(mem) = 0x%02X, want 0xAF", got)
	}
}

// --- Cycle count tests ---

func TestAND_RegReg_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xC0) // byte reg0, AND R0,r
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("AND R,r byte cycles = %d, want 2", cycles)
	}
}

func TestAND_RegImm_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xCC, 0x00)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("AND r,# byte cycles = %d, want 3", cycles)
	}
}

func TestAND_RegImm_Cycles_Word(t *testing.T) {
	c, _ := setupRegOp(t, 0xD8, 0xCC, 0x00, 0x00)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("AND r,# word cycles = %d, want 4", cycles)
	}
}

func TestAND_RegImm_Cycles_Long(t *testing.T) {
	c, _ := setupRegOp(t, 0xE8, 0xCC, 0x00, 0x00, 0x00, 0x00)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("AND r,# long cycles = %d, want 6", cycles)
	}
}

func TestAND_RegMem_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0xC1) // byte indirect R0, AND R1,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 4 {
		t.Errorf("AND R,(mem) byte cycles = %d, want 4", cycles)
	}
}

func TestAND_MemReg_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0xC9)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("AND (mem),R byte cycles = %d, want 6", cycles)
	}
}

func TestAND_MemImm_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3C, 0x00)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("AND (mem),# byte cycles = %d, want 7", cycles)
	}
}

func TestAND_MemImm_Cycles_Word(t *testing.T) {
	c, bus := setupMemOp(t, 0x90, 0x3C, 0x00, 0x00)
	c.reg.WriteReg32(0, 0x2000)
	bus.Write16(0x2000, 0)
	cycles := c.Step()
	if cycles != 8 {
		t.Errorf("AND (mem),# word cycles = %d, want 8", cycles)
	}
}

func TestXOR_RegReg_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xD0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("XOR R,r byte cycles = %d, want 2", cycles)
	}
}

func TestXOR_RegImm_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xCD, 0x00)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("XOR r,# byte cycles = %d, want 3", cycles)
	}
}

func TestXOR_MemReg_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0xD9)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("XOR (mem),R byte cycles = %d, want 6", cycles)
	}
}

func TestXOR_MemImm_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3D, 0x00)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("XOR (mem),# byte cycles = %d, want 7", cycles)
	}
}

func TestOR_RegReg_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xE0)
	cycles := c.Step()
	if cycles != 2 {
		t.Errorf("OR R,r byte cycles = %d, want 2", cycles)
	}
}

func TestOR_RegImm_Cycles_Byte(t *testing.T) {
	c, _ := setupRegOp(t, 0xC8, 0xCE, 0x00)
	cycles := c.Step()
	if cycles != 3 {
		t.Errorf("OR r,# byte cycles = %d, want 3", cycles)
	}
}

func TestOR_MemReg_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0xE9)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 6 {
		t.Errorf("OR (mem),R byte cycles = %d, want 6", cycles)
	}
}

func TestOR_MemImm_Cycles_Byte(t *testing.T) {
	c, bus := setupMemOp(t, 0x80, 0x3E, 0x00)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0)
	cycles := c.Step()
	if cycles != 7 {
		t.Errorf("OR (mem),# byte cycles = %d, want 7", cycles)
	}
}

// --- Verify CHG/BIT #3,(mem) work via dst prefix ---

func TestCHG_Mem_AfterWrap(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xC1) // CHG #1,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x00)
	c.Step()
	got := bus.Read8(0x2000)
	if got != 0x02 {
		t.Errorf("CHG #1,(mem) after wrap: got 0x%02X, want 0x02", got)
	}
}

func TestBIT_Mem_AfterWrap(t *testing.T) {
	c, bus := setupDstMemOp(t, 0xCB) // BIT #3,(mem)
	c.reg.WriteReg32(0, 0x2000)
	bus.write8(0x2000, 0x08)
	cycles := c.Step()
	checkFlags(t, c, 0, 0, 1, -1, 0, -1)
	if cycles != 6 {
		t.Errorf("BIT #3,(mem) after wrap cycles = %d, want 6", cycles)
	}
}

// --- Flags: AND always sets H, clears N and C ---

func TestAND_ClearsCarryAndN(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xC0)
	c.setFlag(flagC, true)
	c.setFlag(flagN, true)
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.reg.WriteReg8(r8From3bit[0], 0xFF)
	c.Step()
	checkFlags(t, c, 1, 0, 1, -1, 0, 0) // H=1, N=0, C=0
}

// --- Flags: OR/XOR clear H, N, C ---

func TestOR_ClearsHNC(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xE0)
	c.setFlag(flagC, true)
	c.setFlag(flagN, true)
	c.setFlag(flagH, true)
	c.reg.WriteReg8(r8From3bit[1], 0xFF)
	c.reg.WriteReg8(r8From3bit[0], 0xFF)
	c.Step()
	checkFlags(t, c, 1, 0, 0, -1, 0, 0) // H=0, N=0, C=0
}

func TestXOR_ClearsHNC(t *testing.T) {
	c, _ := setupRegOp(t, 0xC9, 0xD0)
	c.setFlag(flagC, true)
	c.setFlag(flagN, true)
	c.setFlag(flagH, true)
	c.reg.WriteReg8(r8From3bit[1], 0x0F)
	c.reg.WriteReg8(r8From3bit[0], 0xF0)
	c.Step()
	checkFlags(t, c, 1, 0, 0, -1, 0, 0) // H=0, N=0, C=0
}

// --- Parity flag tests ---

func TestAND_Parity_Odd(t *testing.T) {
	// 0x01 has 1 bit set - odd parity, V=0
	c, _ := setupRegOp(t, 0xC9, 0xCC, 0x01)
	c.reg.WriteReg8(r8From3bit[1], 0x01)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x01)
	checkFlags(t, c, 0, 0, 1, 0, 0, 0) // V=0 (odd parity)
}

func TestAND_Parity_Even(t *testing.T) {
	// 0x03 has 2 bits set - even parity, V=1
	c, _ := setupRegOp(t, 0xC9, 0xCC, 0x03)
	c.reg.WriteReg8(r8From3bit[1], 0x03)
	c.Step()
	checkReg8(t, "A", c.reg.ReadReg8(r8From3bit[1]), 0x03)
	checkFlags(t, c, 0, 0, 1, 1, 0, 0) // V=1 (even parity)
}

// --- Word parity tests ---

func TestAND_Parity_Word_Even(t *testing.T) {
	// 0x0F00 AND 0xFFFF = 0x0F00 (4 bits set, even parity, V=1)
	c, _ := setupRegOp(t, 0xD8, 0xCC, 0x00, 0x0F) // AND WA,#0x0F00
	c.reg.WriteReg16(0, 0xFFFF)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0F00)
	checkFlags(t, c, 0, 0, 1, 1, 0, 0) // V=1 (even parity)
}

func TestAND_Parity_Word_Odd(t *testing.T) {
	// 0x0100 AND 0xFFFF = 0x0100 (1 bit set, odd parity, V=0)
	c, _ := setupRegOp(t, 0xD8, 0xCC, 0x00, 0x01) // AND WA,#0x0100
	c.reg.WriteReg16(0, 0xFFFF)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0100)
	checkFlags(t, c, 0, 0, 1, 0, 0, 0) // V=0 (odd parity)
}

func TestOR_Parity_Word_Odd(t *testing.T) {
	// 0x0000 OR 0x0001 = 0x0001 (1 bit set, odd parity, V=0)
	c, _ := setupRegOp(t, 0xD8, 0xCE, 0x01, 0x00) // OR WA,#0x0001
	c.reg.WriteReg16(0, 0x0000)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0001)
	checkFlags(t, c, 0, 0, 0, 0, 0, 0) // V=0 (odd parity)
}

func TestXOR_Parity_Word_Even(t *testing.T) {
	// 0xFF00 XOR 0x00FF = 0xFFFF (16 bits set, even parity, V=1)
	c, _ := setupRegOp(t, 0xD8, 0xCD, 0xFF, 0x00) // XOR WA,#0x00FF
	c.reg.WriteReg16(0, 0xFF00)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0xFFFF)
	checkFlags(t, c, 1, 0, 0, 1, 0, 0) // V=1 (even parity)
}

func TestXOR_Parity_Word_Odd(t *testing.T) {
	// 0xFF00 XOR 0x0000 = 0xFF00 (8 bits set, even parity, V=1)
	// 0x0100 XOR 0x0000 = 0x0100 (1 bit set, odd parity, V=0)
	c, _ := setupRegOp(t, 0xD8, 0xCD, 0x00, 0x00) // XOR WA,#0x0000
	c.reg.WriteReg16(0, 0x0100)
	c.Step()
	checkReg16(t, "WA", c.reg.ReadReg16(0), 0x0100)
	checkFlags(t, c, 0, 0, 0, 0, 0, 0) // V=0 (odd parity)
}
