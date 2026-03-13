package tlcs900h

import "testing"

func TestRegAccess32(t *testing.T) {
	r := &Registers{}
	r.SR = srMAX // RFP = 0

	// Bank registers (codes 0-3)
	r.WriteReg32(0, 0x11223344)
	r.WriteReg32(1, 0x55667788)
	r.WriteReg32(2, 0xAABBCCDD)
	r.WriteReg32(3, 0xEEFF0011)

	checkReg32(t, "XWA", r.ReadReg32(0), 0x11223344)
	checkReg32(t, "XBC", r.ReadReg32(1), 0x55667788)
	checkReg32(t, "XDE", r.ReadReg32(2), 0xAABBCCDD)
	checkReg32(t, "XHL", r.ReadReg32(3), 0xEEFF0011)

	// Dedicated registers (codes 4-7)
	r.WriteReg32(4, 0x00100000)
	r.WriteReg32(5, 0x00200000)
	r.WriteReg32(6, 0x00300000)
	r.WriteReg32(7, 0x00006C00)

	checkReg32(t, "XIX", r.ReadReg32(4), 0x00100000)
	checkReg32(t, "XIY", r.ReadReg32(5), 0x00200000)
	checkReg32(t, "XIZ", r.ReadReg32(6), 0x00300000)
	checkReg32(t, "XSP", r.ReadReg32(7), 0x00006C00)
}

func TestRegAccess16(t *testing.T) {
	r := &Registers{}
	r.SR = srMAX

	r.WriteReg32(0, 0xAABB1234)
	checkReg16(t, "WA", r.ReadReg16(0), 0x1234)

	r.WriteReg16(0, 0x5678)
	checkReg32(t, "XWA after WriteReg16", r.ReadReg32(0), 0xAABB5678)
}

func TestRegAccess8(t *testing.T) {
	r := &Registers{}
	r.SR = srMAX

	r.WriteReg32(0, 0x00001234)

	// Code 0 = A (low byte of WA)
	checkReg8(t, "A", r.ReadReg8(0), 0x34)
	// Code 8 = W (high byte of WA)
	checkReg8(t, "W", r.ReadReg8(8), 0x12)

	r.WriteReg8(0, 0xAB) // Write A
	checkReg32(t, "XWA after WriteReg8 lo", r.ReadReg32(0), 0x000012AB)

	r.WriteReg8(8, 0xCD) // Write W
	checkReg32(t, "XWA after WriteReg8 hi", r.ReadReg32(0), 0x0000CDAB)
}

func TestBankSwitching(t *testing.T) {
	r := &Registers{}

	// Write to bank 0
	r.SR = srMAX // RFP = 0
	r.WriteReg32(0, 0x11111111)

	// Switch to bank 1
	r.SR = srMAX | (1 << srRFPShift)
	r.WriteReg32(0, 0x22222222)

	// Bank 0 value should be preserved
	r.SR = srMAX // Back to bank 0
	checkReg32(t, "Bank0.XWA", r.ReadReg32(0), 0x11111111)

	// Bank 1 value should be preserved
	r.SR = srMAX | (1 << srRFPShift)
	checkReg32(t, "Bank1.XWA", r.ReadReg32(0), 0x22222222)
}

func TestDedicatedRegsNotBanked(t *testing.T) {
	r := &Registers{}

	// Write XIX in bank 0
	r.SR = srMAX
	r.WriteReg32(4, 0xAAAAAAAA)

	// Switch to bank 2 and read XIX
	r.SR = srMAX | (2 << srRFPShift)
	checkReg32(t, "XIX in bank 2", r.ReadReg32(4), 0xAAAAAAAA)
}

func TestBankRegPtr32(t *testing.T) {
	r := &Registers{}
	r.Bank[2].XDE = 0x99887766

	got := *r.bankRegPtr32(2, 2)
	if got != 0x99887766 {
		t.Errorf("bankRegPtr32(2,2) = 0x%08X, want 0x99887766", got)
	}
}
