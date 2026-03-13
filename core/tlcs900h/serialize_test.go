package tlcs900h

import "testing"

func TestSerializeRoundTrip(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	// Set up varied state
	c.reg.Bank[0].XWA = 0x11223344
	c.reg.Bank[0].XBC = 0x55667788
	c.reg.Bank[1].XDE = 0xAABBCCDD
	c.reg.Bank[2].XHL = 0xEEFF0011
	c.reg.Bank[3].XWA = 0xDEADBEEF
	c.reg.XIX = 0x00100000
	c.reg.XIY = 0x00200000
	c.reg.XIZ = 0x00300000
	c.reg.XSP = 0x00006C00
	c.reg.PC = 0x00001000
	c.reg.SR = srMAX | srSYSM | (3 << srIFFShift) | uint16(flagZ|flagC)
	c.reg.FP = flagS | flagN
	c.cycles = 123456
	c.halted = false
	c.stopped = true
	c.deficit = -5
	c.pendingLevel = 4
	c.pendingVec = 8
	c.hasPending = true

	buf := make([]byte, SerializeSize)
	if err := c.Serialize(buf); err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Deserialize into a fresh CPU
	bus := &testBus{}
	bus.write32LE(0xFFFF00, 0x9999)
	c2 := New(bus)
	if err := c2.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize: %v", err)
	}

	// Verify all fields match
	r1 := c.reg
	r2 := c2.reg
	for bank := 0; bank < 4; bank++ {
		if r1.Bank[bank] != r2.Bank[bank] {
			t.Errorf("Bank[%d] mismatch: got %+v, want %+v", bank, r2.Bank[bank], r1.Bank[bank])
		}
	}
	checkReg32(t, "XIX", r2.XIX, r1.XIX)
	checkReg32(t, "XIY", r2.XIY, r1.XIY)
	checkReg32(t, "XIZ", r2.XIZ, r1.XIZ)
	checkReg32(t, "XSP", r2.XSP, r1.XSP)
	checkReg32(t, "PC", r2.PC, r1.PC)
	checkReg16(t, "SR", r2.SR, r1.SR)
	checkReg8(t, "FP", r2.FP, r1.FP)

	if c2.cycles != c.cycles {
		t.Errorf("cycles = %d, want %d", c2.cycles, c.cycles)
	}
	if c2.halted != c.halted {
		t.Errorf("halted = %v, want %v", c2.halted, c.halted)
	}
	if c2.stopped != c.stopped {
		t.Errorf("stopped = %v, want %v", c2.stopped, c.stopped)
	}
	if c2.deficit != c.deficit {
		t.Errorf("deficit = %d, want %d", c2.deficit, c.deficit)
	}
	if c2.pendingLevel != c.pendingLevel {
		t.Errorf("pendingLevel = %d, want %d", c2.pendingLevel, c.pendingLevel)
	}
	if c2.pendingVec != c.pendingVec {
		t.Errorf("pendingVec = %d, want %d", c2.pendingVec, c.pendingVec)
	}
	if c2.hasPending != c.hasPending {
		t.Errorf("hasPending = %v, want %v", c2.hasPending, c.hasPending)
	}
}

func TestSerializeBufferTooSmall(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	buf := make([]byte, SerializeSize-1)
	if err := c.Serialize(buf); err == nil {
		t.Error("Serialize should fail with small buffer")
	}
	if err := c.Deserialize(buf); err == nil {
		t.Error("Deserialize should fail with small buffer")
	}
}

func TestDeserializeBadVersion(t *testing.T) {
	c, _ := newTestCPU(t, 0x1000)

	buf := make([]byte, SerializeSize)
	buf[0] = 99 // Bad version

	if err := c.Deserialize(buf); err == nil {
		t.Error("Deserialize should fail with bad version")
	}
}
