package core

import "testing"

func TestADC_WriteADMOD_StartConversion(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04) // start, ch0, no scan, 160 cycles
	m := a.ReadADMOD()
	if m&0x40 == 0 {
		t.Error("BUSY should be set after start")
	}
	if m&0x04 != 0 {
		t.Error("START bit should be auto-cleared")
	}
}

func TestADC_WriteADMOD_PreservesReadOnly(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	// Force EOCF set
	a.admod = 0x80
	a.WriteADMOD(0x00) // write 0 - should not clear EOCF
	if a.ReadADMOD()&0x80 == 0 {
		t.Error("EOCF should be preserved on write")
	}
}

func TestADC_Tick_CompletesConversion(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)
	a.SetAN(0, 0x3FF) // full battery

	a.WriteADMOD(0x04) // start ch0, 160 cycles
	a.Tick(160)

	m := a.ReadADMOD()
	if m&0x80 == 0 {
		t.Error("EOCF should be set after conversion")
	}
	if m&0x40 != 0 {
		t.Error("BUSY should be cleared after conversion")
	}
}

func TestADC_Tick_SetsINTADPending(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04) // start ch0
	a.Tick(160)

	if ic.regs[0]&0x80 == 0 {
		t.Error("INTAD pending should be set after conversion")
	}
}

func TestADC_Tick_NoCompletionBeforeTimeout(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04) // start ch0, 160 cycles
	a.Tick(100)        // only 100 of 160

	m := a.ReadADMOD()
	if m&0x80 != 0 {
		t.Error("EOCF should not be set before conversion completes")
	}
	if m&0x40 == 0 {
		t.Error("BUSY should still be set")
	}
	if ic.regs[0]&0x80 != 0 {
		t.Error("INTAD pending should not be set yet")
	}
}

func TestADC_Tick_SlowSpeed(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x0C) // start ch0, speed=1 (320 cycles)
	a.Tick(160)
	if a.ReadADMOD()&0x80 != 0 {
		t.Error("should not complete at 160 cycles with slow speed")
	}
	a.Tick(160)
	if a.ReadADMOD()&0x80 == 0 {
		t.Error("should complete at 320 cycles")
	}
}

func TestADC_ReadADREG_Result(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)
	a.SetAN(0, 0x3FF) // $3FF = 1023

	a.WriteADMOD(0x04)
	a.Tick(160)

	// Low byte: (0x3FF << 6) | 0x3F = 0xFC | 0x3F (lower 8 bits)
	// 0x3FF = 0b11_1111_1111
	// << 6 = 0b1111_1111_1100_0000
	// lower 8 bits = 0b1100_0000 | 0x3F = 0xFF
	lo := a.ReadADREG(0) // ADREG0L
	if lo != 0xFF {
		t.Errorf("ADREG0L = %02X, want $FF", lo)
	}
	// High byte: 0x3FF >> 2 = 0xFF
	hi := a.ReadADREG(1) // ADREG0H
	if hi != 0xFF {
		t.Errorf("ADREG0H = %02X, want $FF", hi)
	}
}

func TestADC_ReadADREG_ClearsEOCF(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04)
	a.Tick(160)
	if a.ReadADMOD()&0x80 == 0 {
		t.Fatal("EOCF should be set before read")
	}

	a.ReadADREG(0)
	if a.admod&0x80 != 0 {
		t.Error("EOCF should be cleared by ADREG read")
	}
}

func TestADC_ReadADREG_LowClearsINTADPending(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04)
	a.Tick(160)
	if ic.regs[0]&0x80 == 0 {
		t.Fatal("INTAD pending should be set before read")
	}

	a.ReadADREG(0) // low byte read clears INTAD pending
	if ic.regs[0]&0x80 != 0 {
		t.Error("INTAD pending should be cleared by low byte read")
	}
}

func TestADC_ReadADREG_HighDoesNotClearINTADPending(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x04)
	a.Tick(160)

	a.ReadADREG(1) // high byte read should NOT clear INTAD pending
	if ic.regs[0]&0x80 == 0 {
		t.Error("INTAD pending should not be cleared by high byte read")
	}
}

func TestADC_ScanMode(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)
	a.SetAN(0, 0x100)
	a.SetAN(1, 0x200)
	a.SetAN(2, 0x300)

	a.WriteADMOD(0x16) // start, scan, ch2 (converts 0,1,2)
	a.Tick(160)

	// Check all three channels have results
	// CH0: result[0] should be 0x100
	hi0 := a.ReadADREG(1) // ADREG0H
	if hi0 != uint8(0x100>>2) {
		t.Errorf("ADREG0H = %02X, want %02X", hi0, uint8(0x100>>2))
	}
	hi1 := a.ReadADREG(3) // ADREG1H
	if hi1 != uint8(0x200>>2) {
		t.Errorf("ADREG1H = %02X, want %02X", hi1, uint8(0x200>>2))
	}
	hi2 := a.ReadADREG(5) // ADREG2H
	if hi2 != uint8(0x300>>2) {
		t.Errorf("ADREG2H = %02X, want %02X", hi2, uint8(0x300>>2))
	}
}

func TestADC_RepeatMode(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	a.WriteADMOD(0x24) // start, repeat, ch0
	a.Tick(160)        // first conversion completes

	if a.ReadADMOD()&0x80 == 0 {
		t.Error("EOCF should be set after first conversion")
	}

	// Should auto-restart: cyclesLeft should be non-zero
	if a.cyclesLeft <= 0 {
		t.Error("repeat mode should restart conversion")
	}
}

func TestADC_DefaultAN0_FullBattery(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)

	if a.an[0] != 0x3FF {
		t.Errorf("default AN0 = %03X, want $3FF (full battery)", a.an[0])
	}
}

func TestADC_Reset(t *testing.T) {
	ic := &IntC{}
	a := NewADC(ic)
	a.WriteADMOD(0x04)
	a.Tick(160)

	a.Reset()
	if a.admod != 0 {
		t.Errorf("ADMOD = %02X after reset, want 0", a.admod)
	}
	if a.cyclesLeft != 0 {
		t.Error("cyclesLeft should be 0 after reset")
	}
	// AN values should be preserved
	if a.an[0] != 0x3FF {
		t.Error("AN0 should be preserved after reset")
	}
}
