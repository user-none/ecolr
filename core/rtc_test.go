package core

import "testing"

func TestRTCSecondTick(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 6144000)

	// Enable RTC
	r.WriteReg(0x90, 0x01)

	// Tick one full second worth of cycles
	r.Tick(6144000)

	// INT0 should be pending (reg 0, low source = bit 3)
	if ic.regs[0]&0x08 == 0 {
		t.Error("INT0 pending should be set after one second tick")
	}
}

func TestRTCDisabled(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 6144000)

	// RTC disabled (ctrl bit 0 clear)
	r.WriteReg(0x90, 0x00)

	// Tick many cycles
	r.Tick(6144000 * 10)

	// INT0 should NOT be pending
	if ic.regs[0]&0x08 != 0 {
		t.Error("INT0 pending should not be set when RTC is disabled")
	}
}

func TestRTCBCDAdvance(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 1000)

	// Set time to xx:xx:58
	r.WriteReg(0x95, 0x00) // minute = 0
	r.WriteReg(0x96, 0x58) // second = 58 (BCD)
	r.WriteReg(0x90, 0x01) // enable

	// Tick 2 seconds
	r.Tick(2000)

	// Second should have rolled from 58 -> 59 -> 00
	sec := r.regs[5]
	if sec != 0x00 {
		t.Errorf("second = $%02X, want $00", sec)
	}

	// Minute should have incremented by 1
	min := r.regs[4]
	if min != 0x01 {
		t.Errorf("minute = $%02X, want $01", min)
	}
}

func TestRTCLatch(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 1000)

	// Set known values
	r.WriteReg(0x91, 0x25) // year
	r.WriteReg(0x92, 0x03) // month
	r.WriteReg(0x93, 0x07) // day

	// Read $91 to trigger latch
	val := r.ReadReg(0x91)
	if val != 0x25 {
		t.Errorf("$91 read = $%02X, want $25", val)
	}

	// Modify the live register
	r.WriteReg(0x92, 0x12)

	// Read $92 should return latched value, not modified
	val = r.ReadReg(0x92)
	if val != 0x03 {
		t.Errorf("$92 latched read = $%02X, want $03 (latched)", val)
	}
}

func TestRTCReadNoLatch(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 1000)
	r.isLatched = false

	// Set a known value
	r.WriteReg(0x92, 0x07)

	// Read $92 without reading $91 first - should return current value
	val := r.ReadReg(0x92)
	if val != 0x07 {
		t.Errorf("$92 unlatched read = $%02X, want $07", val)
	}
}

func TestBCDHelpers(t *testing.T) {
	tests := []struct {
		input    uint8
		wantVal  uint8
		wantWrap bool
	}{
		{0x00, 0x01, false},
		{0x08, 0x09, false},
		{0x09, 0x10, false},
		{0x19, 0x20, false},
		{0x59, 0x60, false},
		{0x99, 0x00, true},
	}

	for _, tc := range tests {
		got, wrap := bcdInc(tc.input)
		if got != tc.wantVal || wrap != tc.wantWrap {
			t.Errorf("bcdInc($%02X) = ($%02X, %v), want ($%02X, %v)",
				tc.input, got, wrap, tc.wantVal, tc.wantWrap)
		}
	}
}

func TestBCDConversion(t *testing.T) {
	for i := 0; i < 100; i++ {
		bcd := toBCD(i)
		back := fromBCD(bcd)
		if back != i {
			t.Errorf("roundtrip %d -> $%02X -> %d", i, bcd, back)
		}
	}
}

func TestRTCCtrlReadWrite(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 1000)

	r.WriteReg(0x90, 0x03)
	if r.ReadReg(0x90) != 0x03 {
		t.Errorf("ctrl = $%02X, want $03", r.ReadReg(0x90))
	}
}

func TestRTCReg97ReadOnly(t *testing.T) {
	ic := &IntC{}
	r := NewRTC(ic, 1000)

	before := r.regs[6]
	r.WriteReg(0x97, 0xFF)
	if r.regs[6] != before {
		t.Error("$97 should be read-only")
	}
}
