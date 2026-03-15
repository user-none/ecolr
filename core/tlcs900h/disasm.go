package tlcs900h

import "fmt"

// DisasmResult holds one disassembled instruction.
type DisasmResult struct {
	Addr  uint32 // start address
	Bytes []byte // raw instruction bytes
	Text  string // mnemonic text
}

// regName32 maps 3-bit register codes to 32-bit register names.
var regName32 = [8]string{"XWA", "XBC", "XDE", "XHL", "XIX", "XIY", "XIZ", "XSP"}

// regName16 maps 3-bit register codes to 16-bit register names.
var regName16 = [8]string{"WA", "BC", "DE", "HL", "IX", "IY", "IZ", "SP"}

// regName8 maps 3-bit R register codes to 8-bit register names.
var regName8 = [8]string{"W", "A", "B", "C", "D", "E", "H", "L"}

// ccName maps condition code values to names.
var ccName = [16]string{
	"F", "LT", "LE", "ULE", "PE", "MI", "Z", "C",
	"T", "GE", "GT", "UGT", "PO", "PL", "NZ", "NC",
}

// sizeSuffix returns a size suffix string for the given Size.
func sizeSuffix(sz Size) string {
	switch sz {
	case Byte:
		return ".B"
	case Word:
		return ".W"
	case Long:
		return ".L"
	}
	return ""
}

// disasmReader reads bytes sequentially from a Bus for disassembly.
type disasmReader struct {
	bus   Bus
	start uint32
	pos   uint32
}

func (r *disasmReader) readByte() uint8 {
	val := r.bus.Read8(r.pos & addrMask)
	r.pos++
	return val
}

func (r *disasmReader) read16() uint16 {
	lo := uint16(r.readByte())
	hi := uint16(r.readByte())
	return hi<<8 | lo
}

func (r *disasmReader) read24() uint32 {
	lo := uint32(r.readByte())
	mid := uint32(r.readByte())
	hi := uint32(r.readByte())
	return hi<<16 | mid<<8 | lo
}

func (r *disasmReader) read32() uint32 {
	lo := uint32(r.read16())
	hi := uint32(r.read16())
	return hi<<16 | lo
}

func (r *disasmReader) bytes() []byte {
	n := r.pos - r.start
	b := make([]byte, n)
	for i := uint32(0); i < n; i++ {
		b[i] = r.bus.Read8((r.start + i) & addrMask)
	}
	return b
}

// regNameForSize returns the register name for a 3-bit code at the given size.
func regNameForSize(sz Size, code uint8) string {
	code &= 0x07
	switch sz {
	case Byte:
		return regName8[code]
	case Word:
		return regName16[code]
	case Long:
		return regName32[code]
	}
	return "?"
}

// regNameEx returns the register name for an extended register code.
// Uses the full 8-bit bank-aware code for all sizes.
func regNameEx(sz Size, code uint8) string {
	switch sz {
	case Byte:
		return regName8ByFullCode(code)
	case Word:
		return regName16ByFullCode(code)
	case Long:
		return regName32ByFullCode(code)
	}
	return "?"
}

// regName32ByFullCode returns a 32-bit register name from the full 8-bit code.
// Bits 7-2 identify the register (bits 1-0 are step size for pre/post ops).
func regName32ByFullCode(code uint8) string {
	base := code & 0xFC
	reg := (code >> 2) & 0x03
	regNames := [4]string{"XWA", "XBC", "XDE", "XHL"}
	switch {
	case base <= 0x3C:
		bank := (code >> 4) & 0x03
		return fmt.Sprintf("%s%d", regNames[reg], bank)
	case base >= 0xD0 && base <= 0xDC:
		return regNames[reg] + "p"
	case base >= 0xE0 && base <= 0xEC:
		return regNames[reg]
	case base == 0xF0:
		return "XIX"
	case base == 0xF4:
		return "XIY"
	case base == 0xF8:
		return "XIZ"
	case base == 0xFC:
		return "XSP"
	}
	return fmt.Sprintf("r$%02X", code)
}

// regName16ByFullCode returns a 16-bit register name from a full 8-bit code.
// regName16ByFullCode returns a 16-bit register name from a full 8-bit code.
// Bit 1 selects low(0) or high(1) word of the 32-bit register.
func regName16ByFullCode(code uint8) string {
	base := code & 0xFC
	reg := (code >> 2) & 0x03
	hiWord := code&0x02 != 0
	loNames := [4]string{"WA", "BC", "DE", "HL"}
	hiNames := [4]string{"QWA", "QBC", "QDE", "QHL"}
	pick := func(r uint8) string {
		if hiWord {
			return hiNames[r]
		}
		return loNames[r]
	}
	dedLo := [4]string{"IX", "IY", "IZ", "SP"}
	dedHi := [4]string{"QIX", "QIY", "QIZ", "QSP"}
	switch {
	case base <= 0x3C:
		bank := (code >> 4) & 0x03
		return fmt.Sprintf("%s%d", pick(reg), bank)
	case base >= 0xD0 && base <= 0xDC:
		return pick(reg) + "p"
	case base >= 0xE0 && base <= 0xEC:
		return pick(reg)
	case base >= 0xF0 && base <= 0xFC:
		dedReg := (base - 0xF0) / 4
		if hiWord {
			return dedHi[dedReg]
		}
		return dedLo[dedReg]
	}
	return fmt.Sprintf("?r16_%02X", code)
}

// regName8ByFullCode returns an 8-bit register name from a full 8-bit code.
// Bits 7-2 identify the 32-bit parent register (same as regName32ByFullCode).
// Bits 1-0 select which byte of the 32-bit register:
// 0=byte0 (A,C,E,L), 1=byte1 (W,B,D,H), 2=byte2, 3=byte3.
func regName8ByFullCode(code uint8) string {
	base := code & 0xFC
	reg := (code >> 2) & 0x03
	byteIdx := code & 0x03
	// Named byte registers only exist for bytes 0 and 1
	byteNames := [4][4]string{
		{"A", "C", "E", "L"},     // byte 0 (low)
		{"W", "B", "D", "H"},     // byte 1
		{"QA", "QC", "QE", "QL"}, // byte 2 (upper word low)
		{"QW", "QB", "QD", "QH"}, // byte 3 (upper word high)
	}
	dedNames := [4][4]string{
		{"IXL", "IYL", "IZL", "SPL"},
		{"IXH", "IYH", "IZH", "SPH"},
		{"QIX0", "QIY0", "QIZ0", "QSP0"},
		{"QIX1", "QIY1", "QIZ1", "QSP1"},
	}
	switch {
	case base <= 0x3C:
		bank := (code >> 4) & 0x03
		return fmt.Sprintf("%s%d", byteNames[byteIdx][reg], bank)
	case base >= 0xD0 && base <= 0xDC:
		return byteNames[byteIdx][reg] + "p"
	case base >= 0xE0 && base <= 0xEC:
		return byteNames[byteIdx][reg]
	case base >= 0xF0 && base <= 0xFC:
		dedReg := (base - 0xF0) / 4
		return dedNames[byteIdx][dedReg]
	}
	return fmt.Sprintf("?r8_%02X", code)
}

// disasmRegIndirectAddr reads the register indirect sub-mode byte and returns
// an address expression string matching the sub-modes in decode.go.
func disasmRegIndirectAddr(r *disasmReader) string {
	code := r.readByte()
	switch code & 0x03 {
	case 0x00: // (R) register indirect
		return fmt.Sprintf("(%s)", regName32ByFullCode(code))
	case 0x01: // (R+d16)
		rn := regName32ByFullCode(code)
		d16 := int16(r.read16())
		return fmt.Sprintf("(%s%+d)", rn, d16)
	case 0x03: // sub-sub modes
		switch code {
		case 0x03: // (R+r8)
			regCode := r.readByte()
			rn := regName32ByFullCode(regCode)
			idxCode := r.readByte()
			idx := regName8ByFullCode(idxCode)
			return fmt.Sprintf("(%s+%s)", rn, idx)
		case 0x07: // (R+r16)
			regCode := r.readByte()
			rn := regName32ByFullCode(regCode)
			idxCode := r.readByte()
			idx := regName16ByFullCode(idxCode)
			return fmt.Sprintf("(%s+%s)", rn, idx)
		case 0x13: // (PC+d16)
			d16 := int16(r.read16())
			return fmt.Sprintf("(PC%+d)", d16)
		}
	}
	return fmt.Sprintf("(?$%02X)", code)
}

// prefixRegName returns the register name for the prefix operand.
func prefixRegName(sz Size, code uint8, extended bool) string {
	if extended {
		return regNameEx(sz, code)
	}
	return regNameForSize(sz, code)
}

// Disasm decodes one instruction at addr and returns the result.
func Disasm(bus Bus, addr uint32) DisasmResult {
	r := &disasmReader{bus: bus, start: addr, pos: addr}
	op := r.readByte()
	text := disasmBase(r, op)
	return DisasmResult{
		Addr:  addr,
		Bytes: r.bytes(),
		Text:  text,
	}
}

// disasmBase decodes the first opcode byte.
func disasmBase(r *disasmReader, op uint8) string {
	switch {
	// NOP, misc standalone
	case op == 0x00:
		return "NOP"
	case op == 0x01:
		// NORMAL - undocumented, treat as NOP-like
		return "NORMAL"
	case op == 0x02:
		return "PUSH SR"
	case op == 0x03:
		return "POP SR"
	case op == 0x04:
		// MAX - undocumented
		return "MAX"
	case op == 0x05:
		return "HALT"
	case op == 0x06:
		n := r.readByte() & 0x07
		if n == 7 {
			return "DI"
		}
		return fmt.Sprintf("EI %d", n)
	case op == 0x07:
		return "RETI"
	case op == 0x08:
		addr := r.readByte()
		val := r.readByte()
		return fmt.Sprintf("LD.B ($%02X),$%02X", addr, val)
	case op == 0x09:
		val := r.readByte()
		return fmt.Sprintf("PUSH.B $%02X", val)
	case op == 0x0A:
		addr := r.readByte()
		val := r.read16()
		return fmt.Sprintf("LD.W ($%02X),$%04X", addr, val)
	case op == 0x0B:
		val := r.read16()
		return fmt.Sprintf("PUSH.W $%04X", val)
	case op == 0x0C:
		return "INCF"
	case op == 0x0D:
		return "DECF"
	case op == 0x0E:
		return "RET"
	case op == 0x0F:
		d16 := r.read16()
		return fmt.Sprintf("RETD $%04X", d16)
	case op == 0x10:
		return "RCF"
	case op == 0x11:
		return "SCF"
	case op == 0x12:
		return "CCF"
	case op == 0x13:
		return "ZCF"
	case op == 0x14:
		return "PUSH A"
	case op == 0x15:
		return "POP A"
	case op == 0x16:
		return "EX F,F'"
	case op == 0x17:
		n := r.readByte() & 0x03
		return fmt.Sprintf("LDF %d", n)
	case op == 0x18:
		return "PUSH F"
	case op == 0x19:
		return "POP F"
	case op == 0x1A:
		addr := r.read16()
		return fmt.Sprintf("JP $%04X", addr)
	case op == 0x1B:
		addr := r.read24()
		return fmt.Sprintf("JP $%06X", addr)
	case op == 0x1C:
		addr := r.read16()
		return fmt.Sprintf("CALL $%04X", addr)
	case op == 0x1D:
		addr := r.read24()
		return fmt.Sprintf("CALL $%06X", addr)
	case op == 0x1E:
		d16 := int16(r.read16())
		target := (r.pos + uint32(int32(d16))) & addrMask
		return fmt.Sprintf("CALR $%06X", target)
	case op == 0x1F:
		return "DB $1F"

	// LD R,# (byte): 0x20-0x27
	case op >= 0x20 && op <= 0x27:
		reg := regName8[op&0x07]
		val := r.readByte()
		return fmt.Sprintf("LD %s,$%02X", reg, val)

	// PUSH RR (word): 0x28-0x2F
	case op >= 0x28 && op <= 0x2F:
		reg := regName16[op&0x07]
		return fmt.Sprintf("PUSH %s", reg)

	// LD RR,## (word): 0x30-0x37
	case op >= 0x30 && op <= 0x37:
		reg := regName16[op&0x07]
		val := r.read16()
		return fmt.Sprintf("LD %s,$%04X", reg, val)

	// PUSH XRR (long): 0x38-0x3F
	case op >= 0x38 && op <= 0x3F:
		reg := regName32[op&0x07]
		return fmt.Sprintf("PUSH %s", reg)

	// LD XRR,#### (long): 0x40-0x47
	case op >= 0x40 && op <= 0x47:
		reg := regName32[op&0x07]
		val := r.read32()
		return fmt.Sprintf("LD %s,$%08X", reg, val)

	// POP RR (word): 0x48-0x4F
	case op >= 0x48 && op <= 0x4F:
		reg := regName16[op&0x07]
		return fmt.Sprintf("POP %s", reg)

	// 0x50-0x57: reserved / unimplemented
	case op >= 0x50 && op <= 0x57:
		return fmt.Sprintf("DB $%02X", op)

	// POP XRR (long): 0x58-0x5F
	case op >= 0x58 && op <= 0x5F:
		reg := regName32[op&0x07]
		return fmt.Sprintf("POP %s", reg)

	// JR cc,d8: 0x60-0x6F
	case op >= 0x60 && op <= 0x6F:
		cc := ccName[op&0x0F]
		d8 := int8(r.readByte())
		target := (r.pos + uint32(int32(d8))) & addrMask
		return fmt.Sprintf("JR %s,$%06X", cc, target)

	// JRL cc,d16: 0x70-0x7F
	case op >= 0x70 && op <= 0x7F:
		cc := ccName[op&0x0F]
		d16 := int16(r.read16())
		target := (r.pos + uint32(int32(d16))) & addrMask
		return fmt.Sprintf("JRL %s,$%06X", cc, target)

	// Source memory prefix (R) indirect: 0x80-0x87 (byte), 0x90-0x97 (word), 0xA0-0xA7 (long)
	case op >= 0x80 && op <= 0x87:
		return disasmSrcMem(r, Byte, fmt.Sprintf("(%s)", regName32[op&0x07]))
	case op >= 0x90 && op <= 0x97:
		return disasmSrcMem(r, Word, fmt.Sprintf("(%s)", regName32[op&0x07]))
	case op >= 0xA0 && op <= 0xA7:
		return disasmSrcMem(r, Long, fmt.Sprintf("(%s)", regName32[op&0x07]))

	// Source memory prefix (R+d8): 0x88-0x8F (byte), 0x98-0x9F (word), 0xA8-0xAF (long)
	case op >= 0x88 && op <= 0x8F:
		d8 := int8(r.readByte())
		return disasmSrcMem(r, Byte, fmt.Sprintf("(%s%+d)", regName32[op&0x07], d8))
	case op >= 0x98 && op <= 0x9F:
		d8 := int8(r.readByte())
		return disasmSrcMem(r, Word, fmt.Sprintf("(%s%+d)", regName32[op&0x07], d8))
	case op >= 0xA8 && op <= 0xAF:
		d8 := int8(r.readByte())
		return disasmSrcMem(r, Long, fmt.Sprintf("(%s%+d)", regName32[op&0x07], d8))

	// Destination memory prefix: 0xB0-0xB7 (indirect), 0xB8-0xBF (disp8)
	case op >= 0xB0 && op <= 0xB7:
		return disasmDstMem(r, fmt.Sprintf("(%s)", regName32[op&0x07]))
	case op >= 0xB8 && op <= 0xBF:
		d8 := int8(r.readByte())
		return disasmDstMem(r, fmt.Sprintf("(%s%+d)", regName32[op&0x07], d8))

	// Source memory direct/regind/predec/postinc: C0-C5 (byte), D0-D5 (word), E0-E5 (long)
	case op == 0xC0:
		addr := r.readByte()
		return disasmSrcMem(r, Byte, fmt.Sprintf("($%02X)", addr))
	case op == 0xC1:
		addr := r.read16()
		return disasmSrcMem(r, Byte, fmt.Sprintf("($%04X)", addr))
	case op == 0xC2:
		addr := r.read24()
		return disasmSrcMem(r, Byte, fmt.Sprintf("($%06X)", addr))
	case op == 0xC3:
		mem := disasmRegIndirectAddr(r)
		return disasmSrcMem(r, Byte, mem)
	case op == 0xC4:
		code := r.readByte()
		return disasmSrcMem(r, Byte, fmt.Sprintf("(-%s)", regName32ByFullCode(code)))
	case op == 0xC5:
		code := r.readByte()
		return disasmSrcMem(r, Byte, fmt.Sprintf("(%s+)", regName32ByFullCode(code)))

	case op == 0xD0:
		addr := r.readByte()
		return disasmSrcMem(r, Word, fmt.Sprintf("($%02X)", addr))
	case op == 0xD1:
		addr := r.read16()
		return disasmSrcMem(r, Word, fmt.Sprintf("($%04X)", addr))
	case op == 0xD2:
		addr := r.read24()
		return disasmSrcMem(r, Word, fmt.Sprintf("($%06X)", addr))
	case op == 0xD3:
		mem := disasmRegIndirectAddr(r)
		return disasmSrcMem(r, Word, mem)
	case op == 0xD4:
		code := r.readByte()
		return disasmSrcMem(r, Word, fmt.Sprintf("(-%s)", regName32ByFullCode(code)))
	case op == 0xD5:
		code := r.readByte()
		return disasmSrcMem(r, Word, fmt.Sprintf("(%s+)", regName32ByFullCode(code)))

	case op == 0xE0:
		addr := r.readByte()
		return disasmSrcMem(r, Long, fmt.Sprintf("($%02X)", addr))
	case op == 0xE1:
		addr := r.read16()
		return disasmSrcMem(r, Long, fmt.Sprintf("($%04X)", addr))
	case op == 0xE2:
		addr := r.read24()
		return disasmSrcMem(r, Long, fmt.Sprintf("($%06X)", addr))
	case op == 0xE3:
		mem := disasmRegIndirectAddr(r)
		return disasmSrcMem(r, Long, mem)
	case op == 0xE4:
		code := r.readByte()
		return disasmSrcMem(r, Long, fmt.Sprintf("(-%s)", regName32ByFullCode(code)))
	case op == 0xE5:
		code := r.readByte()
		return disasmSrcMem(r, Long, fmt.Sprintf("(%s+)", regName32ByFullCode(code)))

	// Extended register prefix: C7 (byte), D7 (word), E7 (long)
	case op == 0xC7:
		code := r.readByte()
		return disasmReg(r, Byte, code, true)
	case op == 0xD7:
		code := r.readByte()
		return disasmReg(r, Word, code, true)
	case op == 0xE7:
		code := r.readByte()
		return disasmReg(r, Long, code, true)

	// Register prefix 3-bit: C8-CF (byte), D8-DF (word), E8-EF (long)
	case op >= 0xC8 && op <= 0xCF:
		return disasmReg(r, Byte, op&0x07, false)
	case op >= 0xD8 && op <= 0xDF:
		return disasmReg(r, Word, op&0x07, false)
	case op >= 0xE8 && op <= 0xEF:
		return disasmReg(r, Long, op&0x07, false)

	// Destination memory direct/regind/predec/postinc: F0-F5
	case op == 0xF0:
		addr := r.readByte()
		return disasmDstMem(r, fmt.Sprintf("($%02X)", addr))
	case op == 0xF1:
		addr := r.read16()
		return disasmDstMem(r, fmt.Sprintf("($%04X)", addr))
	case op == 0xF2:
		addr := r.read24()
		return disasmDstMem(r, fmt.Sprintf("($%06X)", addr))
	case op == 0xF3:
		mem := disasmRegIndirectAddr(r)
		return disasmDstMem(r, mem)
	case op == 0xF4:
		code := r.readByte()
		return disasmDstMem(r, fmt.Sprintf("(-%s)", regName32ByFullCode(code)))
	case op == 0xF5:
		code := r.readByte()
		return disasmDstMem(r, fmt.Sprintf("(%s+)", regName32ByFullCode(code)))

	// F6: reserved
	case op == 0xF6:
		return fmt.Sprintf("DB $%02X", op)

	// F7: LDX
	case op == 0xF7:
		r.readByte() // skip 0x00
		addr := r.readByte()
		r.readByte() // skip 0x00
		val := r.readByte()
		r.readByte() // skip 0x00
		return fmt.Sprintf("LDX ($%02X),$%02X", addr, val)

	// SWI #n: 0xF8-0xFF
	case op >= 0xF8:
		n := op & 0x07
		return fmt.Sprintf("SWI %d", n)
	}

	return fmt.Sprintf("DB $%02X", op)
}

// disasmReg decodes the second opcode byte after a register prefix.
func disasmReg(r *disasmReader, sz Size, code uint8, extended bool) string {
	op2 := r.readByte()
	rn := prefixRegName(sz, code, extended)
	sfx := sizeSuffix(sz)

	switch {
	case op2 == 0x03:
		val := disasmImm(r, sz)
		return fmt.Sprintf("LD%s %s,%s", sfx, rn, val)
	case op2 == 0x04:
		return fmt.Sprintf("PUSH%s %s", sfx, rn)
	case op2 == 0x05:
		return fmt.Sprintf("POP%s %s", sfx, rn)
	case op2 == 0x06:
		return fmt.Sprintf("CPL%s %s", sfx, rn)
	case op2 == 0x07:
		return fmt.Sprintf("NEG%s %s", sfx, rn)
	case op2 == 0x08:
		val := disasmImm(r, sz)
		return fmt.Sprintf("MUL%s %s,%s", sfx, rn, val)
	case op2 == 0x09:
		val := disasmImm(r, sz)
		return fmt.Sprintf("MULS%s %s,%s", sfx, rn, val)
	case op2 == 0x0A:
		val := disasmImm(r, sz)
		return fmt.Sprintf("DIV%s %s,%s", sfx, rn, val)
	case op2 == 0x0B:
		val := disasmImm(r, sz)
		return fmt.Sprintf("DIVS%s %s,%s", sfx, rn, val)
	case op2 == 0x0C:
		d16 := r.read16()
		return fmt.Sprintf("LINK %s,$%04X", rn, d16)
	case op2 == 0x0D:
		return fmt.Sprintf("UNLK %s", rn)
	case op2 == 0x0E:
		return fmt.Sprintf("BS1F A,%s", rn)
	case op2 == 0x0F:
		return fmt.Sprintf("BS1B A,%s", rn)
	case op2 == 0x10:
		return fmt.Sprintf("DAA %s", rn)
	case op2 == 0x12:
		return fmt.Sprintf("EXTZ %s", rn)
	case op2 == 0x13:
		return fmt.Sprintf("EXTS %s", rn)
	case op2 == 0x14:
		return fmt.Sprintf("PAA %s", rn)
	case op2 == 0x16:
		return fmt.Sprintf("MIRR %s", rn)
	case op2 == 0x19:
		return fmt.Sprintf("MULA %s", rn)
	case op2 == 0x1C:
		d8 := int8(r.readByte())
		target := (r.pos + uint32(int32(d8))) & addrMask
		return fmt.Sprintf("DJNZ%s %s,$%06X", sfx, rn, target)

	// Bit/carry flag operations with immediate bit number
	case op2 == 0x20:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("ANDCF %d,%s", bit, rn)
	case op2 == 0x21:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("ORCF %d,%s", bit, rn)
	case op2 == 0x22:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("XORCF %d,%s", bit, rn)
	case op2 == 0x23:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("LDCF %d,%s", bit, rn)
	case op2 == 0x24:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("STCF %d,%s", bit, rn)

	// Bit/carry flag operations with A as bit number
	case op2 == 0x28:
		return fmt.Sprintf("ANDCF A,%s", rn)
	case op2 == 0x29:
		return fmt.Sprintf("ORCF A,%s", rn)
	case op2 == 0x2A:
		return fmt.Sprintf("XORCF A,%s", rn)
	case op2 == 0x2B:
		return fmt.Sprintf("LDCF A,%s", rn)
	case op2 == 0x2C:
		return fmt.Sprintf("STCF A,%s", rn)

	// LDC cr
	case op2 == 0x2E:
		cr := r.readByte()
		return fmt.Sprintf("LDC cr$%02X,%s", cr, rn)
	case op2 == 0x2F:
		cr := r.readByte()
		return fmt.Sprintf("LDC %s,cr$%02X", rn, cr)

	// RES/SET/CHG/BIT/TSET with immediate bit
	case op2 == 0x30:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("RES %d,%s", bit, rn)
	case op2 == 0x31:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("SET %d,%s", bit, rn)
	case op2 == 0x32:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("CHG %d,%s", bit, rn)
	case op2 == 0x33:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("BIT %d,%s", bit, rn)
	case op2 == 0x34:
		bit := r.readByte() & 0x0F
		return fmt.Sprintf("TSET %d,%s", bit, rn)

	// MINC/MDEC
	case op2 == 0x38:
		mask := r.read16()
		return fmt.Sprintf("MINC1 $%04X,%s", mask, rn)
	case op2 == 0x39:
		mask := r.read16()
		return fmt.Sprintf("MINC2 $%04X,%s", mask, rn)
	case op2 == 0x3A:
		mask := r.read16()
		return fmt.Sprintf("MINC4 $%04X,%s", mask, rn)
	case op2 == 0x3C:
		mask := r.read16()
		return fmt.Sprintf("MDEC1 $%04X,%s", mask, rn)
	case op2 == 0x3D:
		mask := r.read16()
		return fmt.Sprintf("MDEC2 $%04X,%s", mask, rn)
	case op2 == 0x3E:
		mask := r.read16()
		return fmt.Sprintf("MDEC4 $%04X,%s", mask, rn)

	// MUL/MULS R,r
	case op2 >= 0x40 && op2 <= 0x47:
		return fmt.Sprintf("MUL%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0x48 && op2 <= 0x4F:
		return fmt.Sprintf("MULS%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	// DIV/DIVS R,r
	case op2 >= 0x50 && op2 <= 0x57:
		return fmt.Sprintf("DIV%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0x58 && op2 <= 0x5F:
		return fmt.Sprintf("DIVS%s %s,%s", sfx, regNameForSize(sz, op2), rn)

	// INC #3,r
	case op2 >= 0x60 && op2 <= 0x67:
		n := op2 & 0x07
		if n == 0 {
			n = 8
		}
		return fmt.Sprintf("INC %d,%s", n, rn)
	// DEC #3,r
	case op2 >= 0x68 && op2 <= 0x6F:
		n := op2 & 0x07
		if n == 0 {
			n = 8
		}
		return fmt.Sprintf("DEC %d,%s", n, rn)

	// SCC cc,r
	case op2 >= 0x70 && op2 <= 0x7F:
		cc := ccName[op2&0x0F]
		return fmt.Sprintf("SCC %s,%s", cc, rn)

	// ADD/ADC/SUB/SBC R,r
	case op2 >= 0x80 && op2 <= 0x87:
		return fmt.Sprintf("ADD%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0x88 && op2 <= 0x8F:
		return fmt.Sprintf("LD%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0x90 && op2 <= 0x97:
		return fmt.Sprintf("ADC%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0x98 && op2 <= 0x9F:
		return fmt.Sprintf("LD%s %s,%s", sfx, rn, regNameForSize(sz, op2))
	case op2 >= 0xA0 && op2 <= 0xA7:
		return fmt.Sprintf("SUB%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0xA8 && op2 <= 0xAF:
		n := op2 & 0x07
		return fmt.Sprintf("LD%s %s,%d", sfx, rn, n)
	case op2 >= 0xB0 && op2 <= 0xB7:
		return fmt.Sprintf("SBC%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0xB8 && op2 <= 0xBF:
		return fmt.Sprintf("EX%s %s,%s", sfx, regNameForSize(sz, op2), rn)

	// AND/XOR/OR R,r
	case op2 >= 0xC0 && op2 <= 0xC7:
		return fmt.Sprintf("AND%s %s,%s", sfx, regNameForSize(sz, op2), rn)

	// ALU r,#
	case op2 == 0xC8:
		val := disasmImm(r, sz)
		return fmt.Sprintf("ADD%s %s,%s", sfx, rn, val)
	case op2 == 0xC9:
		val := disasmImm(r, sz)
		return fmt.Sprintf("ADC%s %s,%s", sfx, rn, val)
	case op2 == 0xCA:
		val := disasmImm(r, sz)
		return fmt.Sprintf("SUB%s %s,%s", sfx, rn, val)
	case op2 == 0xCB:
		val := disasmImm(r, sz)
		return fmt.Sprintf("SBC%s %s,%s", sfx, rn, val)
	case op2 == 0xCC:
		val := disasmImm(r, sz)
		return fmt.Sprintf("AND%s %s,%s", sfx, rn, val)
	case op2 == 0xCD:
		val := disasmImm(r, sz)
		return fmt.Sprintf("XOR%s %s,%s", sfx, rn, val)
	case op2 == 0xCE:
		val := disasmImm(r, sz)
		return fmt.Sprintf("OR%s %s,%s", sfx, rn, val)
	case op2 == 0xCF:
		val := disasmImm(r, sz)
		return fmt.Sprintf("CP%s %s,%s", sfx, rn, val)

	case op2 >= 0xD0 && op2 <= 0xD7:
		return fmt.Sprintf("XOR%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	case op2 >= 0xD8 && op2 <= 0xDF:
		n := op2 & 0x07
		return fmt.Sprintf("CP%s %s,%d", sfx, rn, n)
	case op2 >= 0xE0 && op2 <= 0xE7:
		return fmt.Sprintf("OR%s %s,%s", sfx, regNameForSize(sz, op2), rn)

	// Shift/rotate with immediate count
	case op2 >= 0xE8 && op2 <= 0xEF:
		count := r.readByte() & 0x0F
		if count == 0 {
			count = 16
		}
		return fmt.Sprintf("%s%s %d,%s", shiftName(op2), sfx, count, rn)
	// CP R,r
	case op2 >= 0xF0 && op2 <= 0xF7:
		return fmt.Sprintf("CP%s %s,%s", sfx, regNameForSize(sz, op2), rn)
	// Shift/rotate with A count
	case op2 >= 0xF8 && op2 <= 0xFF:
		return fmt.Sprintf("%s%s A,%s", shiftName(op2), sfx, rn)
	}

	return fmt.Sprintf("DB+reg $%02X", op2)
}

// shiftName returns the mnemonic for a shift/rotate opcode offset.
func shiftName(op uint8) string {
	names := [8]string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}
	return names[op&0x07]
}

// disasmImm reads and formats an immediate value of the given size.
func disasmImm(r *disasmReader, sz Size) string {
	switch sz {
	case Byte:
		return fmt.Sprintf("$%02X", r.readByte())
	case Word:
		return fmt.Sprintf("$%04X", r.read16())
	case Long:
		return fmt.Sprintf("$%08X", r.read32())
	}
	return "?"
}

// disasmSrcMem decodes the second opcode byte after a source memory prefix.
func disasmSrcMem(r *disasmReader, sz Size, mem string) string {
	op2 := r.readByte()
	sfx := sizeSuffix(sz)

	switch {
	case op2 == 0x04:
		return fmt.Sprintf("PUSH%s %s", sfx, mem)
	case op2 == 0x06:
		return fmt.Sprintf("RLD%s %s", sfx, mem)
	case op2 == 0x07:
		return fmt.Sprintf("RRD%s %s", sfx, mem)
	case op2 == 0x10:
		return fmt.Sprintf("LDI%s %s", sfx, mem)
	case op2 == 0x11:
		return fmt.Sprintf("LDIR%s %s", sfx, mem)
	case op2 == 0x12:
		return fmt.Sprintf("LDD%s %s", sfx, mem)
	case op2 == 0x13:
		return fmt.Sprintf("LDDR%s %s", sfx, mem)
	case op2 == 0x14:
		return fmt.Sprintf("CPI%s %s", sfx, mem)
	case op2 == 0x15:
		return fmt.Sprintf("CPIR%s %s", sfx, mem)
	case op2 == 0x16:
		return fmt.Sprintf("CPD%s %s", sfx, mem)
	case op2 == 0x17:
		return fmt.Sprintf("CPDR%s %s", sfx, mem)
	case op2 == 0x19:
		addr := r.read16()
		return fmt.Sprintf("LD%s ($%04X),%s", sfx, addr, mem)

	// LD R,(mem)
	case op2 >= 0x20 && op2 <= 0x27:
		return fmt.Sprintf("LD%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	// EX (mem),R
	case op2 >= 0x30 && op2 <= 0x37:
		return fmt.Sprintf("EX%s %s,%s", sfx, mem, regNameForSize(sz, op2))

	// ALU (mem),#
	case op2 == 0x38:
		val := disasmImm(r, sz)
		return fmt.Sprintf("ADD%s %s,%s", sfx, mem, val)
	case op2 == 0x39:
		val := disasmImm(r, sz)
		return fmt.Sprintf("ADC%s %s,%s", sfx, mem, val)
	case op2 == 0x3A:
		val := disasmImm(r, sz)
		return fmt.Sprintf("SUB%s %s,%s", sfx, mem, val)
	case op2 == 0x3B:
		val := disasmImm(r, sz)
		return fmt.Sprintf("SBC%s %s,%s", sfx, mem, val)
	case op2 == 0x3C:
		val := disasmImm(r, sz)
		return fmt.Sprintf("AND%s %s,%s", sfx, mem, val)
	case op2 == 0x3D:
		val := disasmImm(r, sz)
		return fmt.Sprintf("XOR%s %s,%s", sfx, mem, val)
	case op2 == 0x3E:
		val := disasmImm(r, sz)
		return fmt.Sprintf("OR%s %s,%s", sfx, mem, val)
	case op2 == 0x3F:
		val := disasmImm(r, sz)
		return fmt.Sprintf("CP%s %s,%s", sfx, mem, val)

	// MUL/MULS RR,(mem)
	case op2 >= 0x40 && op2 <= 0x47:
		return fmt.Sprintf("MUL%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0x48 && op2 <= 0x4F:
		return fmt.Sprintf("MULS%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	// DIV/DIVS RR,(mem)
	case op2 >= 0x50 && op2 <= 0x57:
		return fmt.Sprintf("DIV%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0x58 && op2 <= 0x5F:
		return fmt.Sprintf("DIVS%s %s,%s", sfx, regNameForSize(sz, op2), mem)

	// INC/DEC #3,(mem)
	case op2 >= 0x60 && op2 <= 0x67:
		n := op2 & 0x07
		if n == 0 {
			n = 8
		}
		return fmt.Sprintf("INC%s %d,%s", sfx, n, mem)
	case op2 >= 0x68 && op2 <= 0x6F:
		n := op2 & 0x07
		if n == 0 {
			n = 8
		}
		return fmt.Sprintf("DEC%s %d,%s", sfx, n, mem)

	// Shift/rotate (mem), count=1
	case op2 >= 0x78 && op2 <= 0x7F:
		return fmt.Sprintf("%s%s %s", shiftName(op2), sfx, mem)

	// ADD/ADC/SUB/SBC R,(mem)
	case op2 >= 0x80 && op2 <= 0x87:
		return fmt.Sprintf("ADD%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0x88 && op2 <= 0x8F:
		return fmt.Sprintf("ADD%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	case op2 >= 0x90 && op2 <= 0x97:
		return fmt.Sprintf("ADC%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0x98 && op2 <= 0x9F:
		return fmt.Sprintf("ADC%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	case op2 >= 0xA0 && op2 <= 0xA7:
		return fmt.Sprintf("SUB%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xA8 && op2 <= 0xAF:
		return fmt.Sprintf("SUB%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	case op2 >= 0xB0 && op2 <= 0xB7:
		return fmt.Sprintf("SBC%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xB8 && op2 <= 0xBF:
		return fmt.Sprintf("SBC%s %s,%s", sfx, mem, regNameForSize(sz, op2))

	// AND/XOR/OR R,(mem) and (mem),R
	case op2 >= 0xC0 && op2 <= 0xC7:
		return fmt.Sprintf("AND%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xC8 && op2 <= 0xCF:
		return fmt.Sprintf("AND%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	case op2 >= 0xD0 && op2 <= 0xD7:
		return fmt.Sprintf("XOR%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xD8 && op2 <= 0xDF:
		return fmt.Sprintf("XOR%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	case op2 >= 0xE0 && op2 <= 0xE7:
		return fmt.Sprintf("OR%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xE8 && op2 <= 0xEF:
		return fmt.Sprintf("OR%s %s,%s", sfx, mem, regNameForSize(sz, op2))

	// CP R,(mem) and (mem),R
	case op2 >= 0xF0 && op2 <= 0xF7:
		return fmt.Sprintf("CP%s %s,%s", sfx, regNameForSize(sz, op2), mem)
	case op2 >= 0xF8 && op2 <= 0xFF:
		return fmt.Sprintf("CP%s %s,%s", sfx, mem, regNameForSize(sz, op2))
	}

	return fmt.Sprintf("DB+mem $%02X", op2)
}

// disasmDstMem decodes the second opcode byte after a destination memory prefix.
func disasmDstMem(r *disasmReader, mem string) string {
	op2 := r.readByte()

	switch {
	case op2 == 0x00:
		val := r.readByte()
		return fmt.Sprintf("LD.B %s,$%02X", mem, val)
	case op2 == 0x02:
		val := r.read16()
		return fmt.Sprintf("LD.W %s,$%04X", mem, val)
	case op2 == 0x04:
		return fmt.Sprintf("POP.B %s", mem)
	case op2 == 0x06:
		return fmt.Sprintf("POP.W %s", mem)
	case op2 == 0x14:
		addr := r.read16()
		return fmt.Sprintf("LD.B %s,($%04X)", mem, addr)
	case op2 == 0x16:
		addr := r.read16()
		return fmt.Sprintf("LD.W %s,($%04X)", mem, addr)

	// LDA W R,mem
	case op2 >= 0x20 && op2 <= 0x27:
		return fmt.Sprintf("LDA %s,%s", regName16[op2&0x07], mem)
	// LDA L R,mem
	case op2 >= 0x30 && op2 <= 0x37:
		return fmt.Sprintf("LDA %s,%s", regName32[op2&0x07], mem)

	// LD (mem),R byte/word/long
	case op2 >= 0x40 && op2 <= 0x47:
		return fmt.Sprintf("LD.B %s,%s", mem, regName8[op2&0x07])
	case op2 >= 0x50 && op2 <= 0x57:
		return fmt.Sprintf("LD.W %s,%s", mem, regName16[op2&0x07])
	case op2 >= 0x60 && op2 <= 0x67:
		return fmt.Sprintf("LD.L %s,%s", mem, regName32[op2&0x07])

	// Bit operations with #3
	case op2 >= 0x80 && op2 <= 0x87:
		return fmt.Sprintf("ANDCF %d,%s", op2&0x07, mem)
	case op2 >= 0x88 && op2 <= 0x8F:
		return fmt.Sprintf("ORCF %d,%s", op2&0x07, mem)
	case op2 >= 0x90 && op2 <= 0x97:
		return fmt.Sprintf("XORCF %d,%s", op2&0x07, mem)
	case op2 >= 0x98 && op2 <= 0x9F:
		return fmt.Sprintf("LDCF %d,%s", op2&0x07, mem)
	case op2 >= 0xA0 && op2 <= 0xA7:
		return fmt.Sprintf("STCF %d,%s", op2&0x07, mem)
	case op2 >= 0xA8 && op2 <= 0xAF:
		return fmt.Sprintf("TSET %d,%s", op2&0x07, mem)
	case op2 >= 0xB0 && op2 <= 0xB7:
		return fmt.Sprintf("RES %d,%s", op2&0x07, mem)
	case op2 >= 0xB8 && op2 <= 0xBF:
		return fmt.Sprintf("SET %d,%s", op2&0x07, mem)
	case op2 >= 0xC0 && op2 <= 0xC7:
		return fmt.Sprintf("CHG %d,%s", op2&0x07, mem)
	case op2 >= 0xC8 && op2 <= 0xCF:
		return fmt.Sprintf("BIT %d,%s", op2&0x07, mem)

	// Bit operations with A
	case op2 == 0x28:
		return fmt.Sprintf("ANDCF A,%s", mem)
	case op2 == 0x29:
		return fmt.Sprintf("ORCF A,%s", mem)
	case op2 == 0x2A:
		return fmt.Sprintf("XORCF A,%s", mem)
	case op2 == 0x2B:
		return fmt.Sprintf("LDCF A,%s", mem)
	case op2 == 0x2C:
		return fmt.Sprintf("STCF A,%s", mem)

	// JP cc,mem
	case op2 >= 0xD0 && op2 <= 0xDF:
		cc := ccName[op2&0x0F]
		return fmt.Sprintf("JP %s,%s", cc, mem)
	// CALL cc,mem
	case op2 >= 0xE0 && op2 <= 0xEF:
		cc := ccName[op2&0x0F]
		return fmt.Sprintf("CALL %s,%s", cc, mem)
	// RET cc
	case op2 >= 0xF0 && op2 <= 0xFF:
		cc := ccName[op2&0x0F]
		return fmt.Sprintf("RET %s", cc)
	}

	return fmt.Sprintf("DB+dst $%02X", op2)
}
