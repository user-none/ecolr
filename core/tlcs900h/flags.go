package tlcs900h

// Status register flag bits (lower byte = F register).
const (
	flagC uint8 = 1 << 0 // Carry
	flagN uint8 = 1 << 1 // Subtract
	flagV uint8 = 1 << 2 // Overflow / Parity
	flagH uint8 = 1 << 4 // Half-carry
	flagZ uint8 = 1 << 6 // Zero
	flagS uint8 = 1 << 7 // Sign
)

// SR upper byte fields.
const (
	srRFPMask  uint16 = 0x0700 // Register File Pointer (bits 10-8)
	srRFPShift        = 8
	srMAX      uint16 = 0x0800 // Maximum mode (bit 11, always 1)
	srIFFMask  uint16 = 0x7000 // Interrupt mask level (bits 14-12)
	srIFFShift        = 12
	srSYSM     uint16 = 0x8000 // System mode (bit 15)
)

// Exported SR bit values for use by external packages (e.g. HLE BIOS).
const (
	SRInitMax  uint16 = srMAX
	SRInitSYSM uint16 = srSYSM
	SRInitIFF7 uint16 = 7 << srIFFShift // IFF=7: all maskable interrupts masked
)

// flags returns the F register (lower 8 bits of SR).
func (c *CPU) flags() uint8 {
	return uint8(c.reg.SR)
}

// setFlags replaces the F register portion of SR.
func (c *CPU) setFlags(f uint8) {
	c.reg.SR = (c.reg.SR & 0xFF00) | uint16(f)
}

// getFlag returns true if the specified flag bit is set.
func (c *CPU) getFlag(flag uint8) bool {
	return uint8(c.reg.SR)&flag != 0
}

// setFlag sets or clears a flag bit.
func (c *CPU) setFlag(flag uint8, set bool) {
	if set {
		c.reg.SR |= uint16(flag)
	} else {
		c.reg.SR &^= uint16(flag)
	}
}

// swapFlags swaps F and F' (EX F,F' instruction).
func (c *CPU) swapFlags() {
	f := uint8(c.reg.SR)
	c.reg.SR = (c.reg.SR & 0xFF00) | uint16(c.reg.FP)
	c.reg.FP = f
}

// iff returns the current interrupt mask level from SR bits 14-12.
func (c *CPU) iff() uint8 {
	return uint8((c.reg.SR & srIFFMask) >> srIFFShift)
}

// Condition code constants (4-bit encoding from instructions).
const (
	ccF   uint8 = 0x0 // False (never)
	ccLT  uint8 = 0x1 // Signed less than
	ccLE  uint8 = 0x2 // Signed less or equal
	ccULE uint8 = 0x3 // Unsigned less or equal
	ccOV  uint8 = 0x4 // Overflow
	ccMI  uint8 = 0x5 // Minus (negative)
	ccEQ  uint8 = 0x6 // Equal (zero)
	ccULT uint8 = 0x7 // Unsigned less than (carry)
	ccT   uint8 = 0x8 // True (always)
	ccGE  uint8 = 0x9 // Signed greater or equal
	ccGT  uint8 = 0xA // Signed greater than
	ccUGT uint8 = 0xB // Unsigned greater than
	ccNOV uint8 = 0xC // No overflow
	ccPL  uint8 = 0xD // Plus (positive)
	ccNE  uint8 = 0xE // Not equal (not zero)
	ccUGE uint8 = 0xF // Unsigned greater or equal (no carry)
)

// testCondition evaluates one of the 16 condition codes against the
// current flag state.
func (c *CPU) testCondition(cc uint8) bool {
	f := c.flags()
	s := f&flagS != 0
	z := f&flagZ != 0
	v := f&flagV != 0
	carry := f&flagC != 0

	switch cc & 0x0F {
	case ccF:
		return false
	case ccLT:
		return s != v
	case ccLE:
		return (s != v) || z
	case ccULE:
		return carry || z
	case ccOV:
		return v
	case ccMI:
		return s
	case ccEQ:
		return z
	case ccULT:
		return carry
	case ccT:
		return true
	case ccGE:
		return s == v
	case ccGT:
		return (s == v) && !z
	case ccUGT:
		return !carry && !z
	case ccNOV:
		return !v
	case ccPL:
		return !s
	case ccNE:
		return !z
	case ccUGE:
		return !carry
	}
	return false
}

// baseSZFlags computes the Sign and Zero flags for the given size and
// result. Returns the flag byte and the masked result.
func baseSZFlags(sz Size, result uint32) (uint8, uint32) {
	mask := sz.Mask()
	r := result & mask
	var f uint8
	if r&sz.MSB() != 0 {
		f |= flagS
	}
	if r == 0 {
		f |= flagZ
	}
	return f, r
}

// setFlagsArith sets flags after an arithmetic operation.
// The n flag parameter indicates subtraction (sets N flag).
func (c *CPU) setFlagsArith(sz Size, result uint32, carry bool, overflow bool, halfCarry bool, subtract bool) {
	f, _ := baseSZFlags(sz, result)

	if halfCarry {
		f |= flagH
	}
	if overflow {
		f |= flagV
	}
	if subtract {
		f |= flagN
	}
	if carry {
		f |= flagC
	}
	c.setFlags(f)
}

// setFlagsLogic sets flags after a logic operation (AND, OR, XOR).
// H is set for AND, cleared for OR/XOR. N and C are always cleared.
func (c *CPU) setFlagsLogic(sz Size, result uint32, halfCarry bool) {
	f, r := baseSZFlags(sz, result)

	if halfCarry {
		f |= flagH
	}
	// V is set based on parity for byte and word operations.
	// For 32-bit (long), parity is undefined (left as 0).
	f |= parity(sz, r)
	c.setFlags(f)
}

// setFlagsShift sets flags after a shift or rotate operation.
// S:* Z:* H:0 V:P(byte/word) N:0 C:*
func (c *CPU) setFlagsShift(sz Size, result uint32, carry bool) {
	f, r := baseSZFlags(sz, result)

	f |= parity(sz, r)
	if carry {
		f |= flagC
	}
	c.setFlags(f)
}

// parity returns the V flag based on even parity for byte and word sizes.
// For long (32-bit), parity is undefined per the databook; returns 0.
func parity(sz Size, r uint32) uint8 {
	switch sz {
	case Byte:
		return parityTable[r&0xFF]
	case Word:
		return parityTable[(r&0xFF)^((r>>8)&0xFF)]
	default:
		return 0
	}
}

// parityTable maps byte values to the V flag value for even parity.
var parityTable [256]uint8

func init() {
	for i := 0; i < 256; i++ {
		bits := 0
		v := i
		for v != 0 {
			bits += v & 1
			v >>= 1
		}
		if bits%2 == 0 {
			parityTable[i] = flagV
		}
	}
}
