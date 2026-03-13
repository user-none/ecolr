package tlcs900h

// r8From3bit maps 3-bit R byte-register codes to 4-bit register codes.
//
//	0:W(0x08) 1:A(0x00) 2:B(0x09) 3:C(0x01)
//	4:D(0x0A) 5:E(0x02) 6:H(0x0B) 7:L(0x03)
var r8From3bit = [8]uint8{0x08, 0x00, 0x09, 0x01, 0x0A, 0x02, 0x0B, 0x03}

// rfp returns the active register file pointer (bank index 0-3)
// from the SR RFP field (bits 10-8). Only the low 2 bits are used
// since there are 4 banks; the hardware defines 3 bits but values
// above 3 are not valid.
func (r *Registers) rfp() int {
	return int((r.SR&srRFPMask)>>srRFPShift) & 0x03
}

// bank returns a pointer to the active register bank.
func (r *Registers) bank() *RegisterBank {
	return &r.Bank[r.rfp()]
}

// regPtr32 returns a pointer to a 32-bit register by 3-bit code.
// Codes 0-3 map to the active bank, 4-7 map to dedicated registers.
//
//	0: XWA  1: XBC  2: XDE  3: XHL
//	4: XIX  5: XIY  6: XIZ  7: XSP
func (r *Registers) regPtr32(code uint8) *uint32 {
	switch code & 0x07 {
	case 0:
		return &r.bank().XWA
	case 1:
		return &r.bank().XBC
	case 2:
		return &r.bank().XDE
	case 3:
		return &r.bank().XHL
	case 4:
		return &r.XIX
	case 5:
		return &r.XIY
	case 6:
		return &r.XIZ
	case 7:
		return &r.XSP
	}
	return nil
}

// bankRegPtr32 returns a pointer to a 32-bit bank register (codes 0-3 only)
// in the specified bank.
func (r *Registers) bankRegPtr32(bank int, code uint8) *uint32 {
	b := &r.Bank[bank&0x03]
	switch code & 0x03 {
	case 0:
		return &b.XWA
	case 1:
		return &b.XBC
	case 2:
		return &b.XDE
	case 3:
		return &b.XHL
	}
	return nil
}

// regPtrByFullCode32 returns a pointer to a 32-bit register using the full
// 8-bit register code encoding. Bits 7-2 identify the register, bits 1-0
// are ignored (caller should mask them for pre-dec/post-inc step size).
//
// Code ranges (after masking bits 1-0):
//
//	0x00-0x0C: Bank 0 (XWA, XBC, XDE, XHL)
//	0x10-0x1C: Bank 1
//	0x20-0x2C: Bank 2
//	0x30-0x3C: Bank 3
//	0xD0-0xDC: Previous bank (rfp-1)
//	0xE0-0xEC: Current bank
//	0xF0-0xFC: XIX, XIY, XIZ, XSP
func (r *Registers) regPtrByFullCode32(code uint8) *uint32 {
	base := code & 0xFC
	reg := (code >> 2) & 0x03
	switch {
	case base <= 0x3C:
		bank := int((code >> 4) & 0x03)
		return r.bankRegPtr32(bank, reg)
	case base >= 0xD0 && base <= 0xDC:
		prev := (r.rfp() - 1) & 0x03
		return r.bankRegPtr32(prev, reg)
	case base >= 0xE0 && base <= 0xEC:
		return r.bankRegPtr32(r.rfp(), reg)
	case base == 0xF0:
		return &r.XIX
	case base == 0xF4:
		return &r.XIY
	case base == 0xF8:
		return &r.XIZ
	case base == 0xFC:
		return &r.XSP
	}
	// Invalid code: return XWA of current bank as fallback
	return &r.bank().XWA
}

// ReadReg32 reads a 32-bit register by 3-bit code.
func (r *Registers) ReadReg32(code uint8) uint32 {
	return *r.regPtr32(code)
}

// WriteReg32 writes a 32-bit register by 3-bit code.
func (r *Registers) WriteReg32(code uint8, val uint32) {
	*r.regPtr32(code) = val
}

// ReadReg16 reads the low 16 bits of a register by 3-bit code.
func (r *Registers) ReadReg16(code uint8) uint16 {
	return uint16(*r.regPtr32(code))
}

// WriteReg16 writes the low 16 bits of a register by 3-bit code,
// preserving the upper 16 bits.
func (r *Registers) WriteReg16(code uint8, val uint16) {
	p := r.regPtr32(code)
	*p = (*p & 0xFFFF0000) | uint32(val)
}

// ReadReg16ByFullCode reads a 16-bit value from a register using the full
// 8-bit register code (bank-aware). Bit 1 selects which word of the
// 32-bit register: 0=low word, 1=high word (bit 0 is ignored for word
// alignment). Used by (R+r16) addressing and extended register prefix.
func (r *Registers) ReadReg16ByFullCode(code uint8) uint16 {
	p := r.regPtrByFullCode32(code)
	if code&0x02 != 0 {
		return uint16(*p >> 16)
	}
	return uint16(*p)
}

// ReadReg8ByFullCode reads an 8-bit register using the full 8-bit register
// code (bank-aware). Bits 1-0 select which byte of the 32-bit register:
// 0=byte0 (lowest), 1=byte1, 2=byte2, 3=byte3 (highest).
// Used by (R+r8) addressing and extended register prefix.
func (r *Registers) ReadReg8ByFullCode(code uint8) uint8 {
	p := r.regPtrByFullCode32(code)
	shift := (code & 0x03) * 8
	return uint8(*p >> shift)
}

// WriteReg8ByFullCode writes an 8-bit register using the full 8-bit register
// code (bank-aware). Bits 1-0 select which byte of the 32-bit register:
// 0=byte0 (lowest), 1=byte1, 2=byte2, 3=byte3 (highest).
// Used by extended register prefix.
func (r *Registers) WriteReg8ByFullCode(code uint8, val uint8) {
	p := r.regPtrByFullCode32(code)
	shift := (code & 0x03) * 8
	mask := uint32(0xFF) << shift
	*p = (*p &^ mask) | uint32(val)<<shift
}

// WriteReg16ByFullCode writes a 16-bit value to a register using the full
// 8-bit register code (bank-aware). Bit 1 selects which word of the
// 32-bit register: 0=low word, 1=high word. Used by extended register prefix.
func (r *Registers) WriteReg16ByFullCode(code uint8, val uint16) {
	p := r.regPtrByFullCode32(code)
	if code&0x02 != 0 {
		*p = (*p & 0x0000FFFF) | uint32(val)<<16
	} else {
		*p = (*p & 0xFFFF0000) | uint32(val)
	}
}

// ReadReg32ByFullCode reads a 32-bit register using the full 8-bit register
// code (bank-aware). Used by extended register prefix.
func (r *Registers) ReadReg32ByFullCode(code uint8) uint32 {
	return *r.regPtrByFullCode32(code)
}

// WriteReg32ByFullCode writes a 32-bit register using the full 8-bit register
// code (bank-aware). Used by extended register prefix.
func (r *Registers) WriteReg32ByFullCode(code uint8, val uint32) {
	*r.regPtrByFullCode32(code) = val
}

// ReadReg8 reads an 8-bit register by 4-bit code.
// The low 3 bits select the 32-bit register, bit 3 selects high/low byte
// of the low 16-bit word.
//
//	Bit 3 = 0: low byte (A, C, E, L, IXL, IYL, IZL, SPL)
//	Bit 3 = 1: high byte (W, B, D, H, IXH, IYH, IZH, SPH)
func (r *Registers) ReadReg8(code uint8) uint8 {
	p := r.regPtr32(code & 0x07)
	if code&0x08 != 0 {
		return uint8(*p >> 8)
	}
	return uint8(*p)
}

// WriteReg8 writes an 8-bit register by 4-bit code.
// See ReadReg8 for code encoding.
func (r *Registers) WriteReg8(code uint8, val uint8) {
	p := r.regPtr32(code & 0x07)
	if code&0x08 != 0 {
		*p = (*p & 0xFFFF00FF) | uint32(val)<<8
	} else {
		*p = (*p & 0xFFFFFF00) | uint32(val)
	}
}
