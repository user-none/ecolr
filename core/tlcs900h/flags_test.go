package tlcs900h

import "testing"

func TestConditionCodes(t *testing.T) {
	tests := []struct {
		name  string
		flags uint8
		cc    uint8
		want  bool
	}{
		// Always false / always true
		{"F", 0, ccF, false},
		{"T", 0, ccT, true},
		{"F with flags", flagS | flagZ | flagC, ccF, false},
		{"T with flags", flagS | flagZ | flagC, ccT, true},

		// Zero flag
		{"EQ when Z", flagZ, ccEQ, true},
		{"EQ when !Z", 0, ccEQ, false},
		{"NE when Z", flagZ, ccNE, false},
		{"NE when !Z", 0, ccNE, true},

		// Carry flag
		{"ULT when C", flagC, ccULT, true},
		{"ULT when !C", 0, ccULT, false},
		{"UGE when C", flagC, ccUGE, false},
		{"UGE when !C", 0, ccUGE, true},

		// Sign flag
		{"MI when S", flagS, ccMI, true},
		{"MI when !S", 0, ccMI, false},
		{"PL when S", flagS, ccPL, false},
		{"PL when !S", 0, ccPL, true},

		// Overflow flag
		{"OV when V", flagV, ccOV, true},
		{"OV when !V", 0, ccOV, false},
		{"NOV when V", flagV, ccNOV, false},
		{"NOV when !V", 0, ccNOV, true},

		// Signed comparisons: LT = S xor V
		{"LT S=1 V=0", flagS, ccLT, true},
		{"LT S=0 V=1", flagV, ccLT, true},
		{"LT S=1 V=1", flagS | flagV, ccLT, false},
		{"LT S=0 V=0", 0, ccLT, false},

		// GE = !(S xor V)
		{"GE S=1 V=1", flagS | flagV, ccGE, true},
		{"GE S=0 V=0", 0, ccGE, true},
		{"GE S=1 V=0", flagS, ccGE, false},

		// LE = (S xor V) or Z
		{"LE Z=1", flagZ, ccLE, true},
		{"LE S!=V", flagS, ccLE, true},
		{"LE S==V !Z", 0, ccLE, false},

		// GT = !(S xor V) and !Z
		{"GT S==V !Z", 0, ccGT, true},
		{"GT Z=1", flagZ, ccGT, false},
		{"GT S!=V", flagS, ccGT, false},

		// ULE = C or Z
		{"ULE C=1", flagC, ccULE, true},
		{"ULE Z=1", flagZ, ccULE, true},
		{"ULE !C !Z", 0, ccULE, false},

		// UGT = !C and !Z
		{"UGT !C !Z", 0, ccUGT, true},
		{"UGT C=1", flagC, ccUGT, false},
		{"UGT Z=1", flagZ, ccUGT, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &CPU{}
			c.setFlags(tc.flags)
			got := c.testCondition(tc.cc)
			if got != tc.want {
				t.Errorf("testCondition(0x%X) with flags=0x%02X: got %v, want %v",
					tc.cc, tc.flags, got, tc.want)
			}
		})
	}
}

func TestSwapFlags(t *testing.T) {
	c := &CPU{}
	c.reg.SR = srMAX | uint16(flagS|flagC)
	c.reg.FP = flagZ | flagN

	c.swapFlags()

	gotF := c.flags()
	wantF := flagZ | flagN
	if gotF != wantF {
		t.Errorf("F after swap = 0x%02X, want 0x%02X", gotF, wantF)
	}

	wantFP := flagS | flagC
	if c.reg.FP != wantFP {
		t.Errorf("F' after swap = 0x%02X, want 0x%02X", c.reg.FP, wantFP)
	}
}

func TestSetFlagsArith(t *testing.T) {
	c := &CPU{}

	// Zero result
	c.setFlagsArith(Byte, 0, false, false, false, false)
	if !c.getFlag(flagZ) {
		t.Error("Z should be set for zero result")
	}
	if c.getFlag(flagS) {
		t.Error("S should be clear for zero result")
	}

	// Negative result (byte)
	c.setFlagsArith(Byte, 0x80, false, false, false, false)
	if !c.getFlag(flagS) {
		t.Error("S should be set for 0x80")
	}
	if c.getFlag(flagZ) {
		t.Error("Z should be clear for 0x80")
	}

	// Carry and subtract
	c.setFlagsArith(Byte, 0, true, false, false, true)
	if !c.getFlag(flagC) {
		t.Error("C should be set")
	}
	if !c.getFlag(flagN) {
		t.Error("N should be set for subtraction")
	}
}

func TestParityTable(t *testing.T) {
	// 0x00: 0 bits set -> even parity -> V set
	if parityTable[0x00] != flagV {
		t.Error("parity(0x00) should be even")
	}
	// 0x01: 1 bit set -> odd parity -> V clear
	if parityTable[0x01] != 0 {
		t.Error("parity(0x01) should be odd")
	}
	// 0x03: 2 bits set -> even parity
	if parityTable[0x03] != flagV {
		t.Error("parity(0x03) should be even")
	}
	// 0xFF: 8 bits set -> even parity
	if parityTable[0xFF] != flagV {
		t.Error("parity(0xFF) should be even")
	}
}

func TestIFF(t *testing.T) {
	c := &CPU{}
	c.reg.SR = srMAX | (5 << srIFFShift)
	if c.iff() != 5 {
		t.Errorf("iff() = %d, want 5", c.iff())
	}
}
