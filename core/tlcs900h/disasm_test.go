package tlcs900h

import "testing"

// disasmWith loads data into the testBus at addr and disassembles there.
func disasmWith(data []byte, addr uint32) DisasmResult {
	bus := &testBus{}
	for i, b := range data {
		bus.write8(addr+uint32(i), b)
	}
	return Disasm(bus, addr)
}

func TestDisasmNOP(t *testing.T) {
	r := disasmWith([]byte{0x00}, 0)
	if r.Text != "NOP" {
		t.Errorf("got %q, want NOP", r.Text)
	}
	if len(r.Bytes) != 1 {
		t.Errorf("len = %d, want 1", len(r.Bytes))
	}
}

func TestDisasmPUSHSR(t *testing.T) {
	r := disasmWith([]byte{0x02}, 0)
	if r.Text != "PUSH SR" {
		t.Errorf("got %q, want PUSH SR", r.Text)
	}
}

func TestDisasmPOPSR(t *testing.T) {
	r := disasmWith([]byte{0x03}, 0)
	if r.Text != "POP SR" {
		t.Errorf("got %q, want POP SR", r.Text)
	}
}

func TestDisasmEI(t *testing.T) {
	r := disasmWith([]byte{0x06, 0x03}, 0)
	if r.Text != "EI 3" {
		t.Errorf("got %q, want EI 3", r.Text)
	}
	if len(r.Bytes) != 2 {
		t.Errorf("len = %d, want 2", len(r.Bytes))
	}
}

func TestDisasmDI(t *testing.T) {
	r := disasmWith([]byte{0x06, 0x07}, 0)
	if r.Text != "DI" {
		t.Errorf("got %q, want DI", r.Text)
	}
}

func TestDisasmRET(t *testing.T) {
	r := disasmWith([]byte{0x0E}, 0)
	if r.Text != "RET" {
		t.Errorf("got %q, want RET", r.Text)
	}
}

func TestDisasmRETI(t *testing.T) {
	r := disasmWith([]byte{0x07}, 0)
	if r.Text != "RETI" {
		t.Errorf("got %q, want RETI", r.Text)
	}
}

func TestDisasmLDByte(t *testing.T) {
	r := disasmWith([]byte{0x08, 0x20, 0xFF}, 0)
	if r.Text != "LD.B ($20),$FF" {
		t.Errorf("got %q, want LD.B ($20),$FF", r.Text)
	}
	if len(r.Bytes) != 3 {
		t.Errorf("len = %d, want 3", len(r.Bytes))
	}
}

func TestDisasmLDWord(t *testing.T) {
	r := disasmWith([]byte{0x0A, 0x30, 0x34, 0x12}, 0)
	if r.Text != "LD.W ($30),$1234" {
		t.Errorf("got %q, want LD.W ($30),$1234", r.Text)
	}
	if len(r.Bytes) != 4 {
		t.Errorf("len = %d, want 4", len(r.Bytes))
	}
}

func TestDisasmJP16(t *testing.T) {
	r := disasmWith([]byte{0x1A, 0x34, 0x12}, 0)
	if r.Text != "JP $1234" {
		t.Errorf("got %q, want JP $1234", r.Text)
	}
	if len(r.Bytes) != 3 {
		t.Errorf("len = %d, want 3", len(r.Bytes))
	}
}

func TestDisasmJP24(t *testing.T) {
	r := disasmWith([]byte{0x1B, 0x10, 0x00, 0xFF}, 0)
	if r.Text != "JP $FF0010" {
		t.Errorf("got %q, want JP $FF0010", r.Text)
	}
	if len(r.Bytes) != 4 {
		t.Errorf("len = %d, want 4", len(r.Bytes))
	}
}

func TestDisasmCALL16(t *testing.T) {
	r := disasmWith([]byte{0x1C, 0xCD, 0xAB}, 0)
	if r.Text != "CALL $ABCD" {
		t.Errorf("got %q, want CALL $ABCD", r.Text)
	}
}

func TestDisasmCALL24(t *testing.T) {
	r := disasmWith([]byte{0x1D, 0x56, 0x34, 0x12}, 0)
	if r.Text != "CALL $123456" {
		t.Errorf("got %q, want CALL $123456", r.Text)
	}
}

func TestDisasmPUSHA(t *testing.T) {
	r := disasmWith([]byte{0x14}, 0)
	if r.Text != "PUSH A" {
		t.Errorf("got %q, want PUSH A", r.Text)
	}
}

func TestDisasmPOPA(t *testing.T) {
	r := disasmWith([]byte{0x15}, 0)
	if r.Text != "POP A" {
		t.Errorf("got %q, want POP A", r.Text)
	}
}

func TestDisasmSWI(t *testing.T) {
	r := disasmWith([]byte{0xFB}, 0)
	if r.Text != "SWI 3" {
		t.Errorf("got %q, want SWI 3", r.Text)
	}
	if len(r.Bytes) != 1 {
		t.Errorf("len = %d, want 1", len(r.Bytes))
	}
}

func TestDisasmSWI0(t *testing.T) {
	r := disasmWith([]byte{0xF8}, 0)
	if r.Text != "SWI 0" {
		t.Errorf("got %q, want SWI 0", r.Text)
	}
}

func TestDisasmJRZ(t *testing.T) {
	// JR Z - opcode $66 (Z=6), displacement $04
	// PC after reading disp = 2, target = 2 + 4 = 6
	r := disasmWith([]byte{0x66, 0x04}, 0)
	if r.Text != "JR Z,$000006" {
		t.Errorf("got %q, want JR Z,$000006", r.Text)
	}
	if len(r.Bytes) != 2 {
		t.Errorf("len = %d, want 2", len(r.Bytes))
	}
}

func TestDisasmJRNZ(t *testing.T) {
	// JR NZ - opcode $6E, displacement backwards (-4 = 0xFC)
	// PC after reading disp = 2, target = 2 + (-4) = -2 -> masked to 0xFFFFFE
	r := disasmWith([]byte{0x6E, 0xFC}, 0)
	if r.Text != "JR NZ,$FFFFFE" {
		t.Errorf("got %q, want JR NZ,$FFFFFE", r.Text)
	}
}

func TestDisasmJRLT(t *testing.T) {
	// JRL T,$0102 - opcode $78 (T=8), displacement LE $00, $01
	// PC after reading disp = 3, target = 3 + 256 = 259 = 0x103
	r := disasmWith([]byte{0x78, 0x00, 0x01}, 0)
	if r.Text != "JRL T,$000103" {
		t.Errorf("got %q, want JRL T,$000103", r.Text)
	}
	if len(r.Bytes) != 3 {
		t.Errorf("len = %d, want 3", len(r.Bytes))
	}
}

func TestDisasmLDReg8Imm(t *testing.T) {
	// LD A,$42 - opcode $21 (A=1)
	r := disasmWith([]byte{0x21, 0x42}, 0)
	if r.Text != "LD A,$42" {
		t.Errorf("got %q, want LD A,$42", r.Text)
	}
}

func TestDisasmLDReg16Imm(t *testing.T) {
	// LD WA,$1234 - opcode $30 (WA=0)
	r := disasmWith([]byte{0x30, 0x34, 0x12}, 0)
	if r.Text != "LD WA,$1234" {
		t.Errorf("got %q, want LD WA,$1234", r.Text)
	}
}

func TestDisasmLDReg32Imm(t *testing.T) {
	// LD XWA,$12345678 - opcode $40
	r := disasmWith([]byte{0x40, 0x78, 0x56, 0x34, 0x12}, 0)
	if r.Text != "LD XWA,$12345678" {
		t.Errorf("got %q, want LD XWA,$12345678", r.Text)
	}
	if len(r.Bytes) != 5 {
		t.Errorf("len = %d, want 5", len(r.Bytes))
	}
}

func TestDisasmPUSHRR(t *testing.T) {
	r := disasmWith([]byte{0x28}, 0)
	if r.Text != "PUSH WA" {
		t.Errorf("got %q, want PUSH WA", r.Text)
	}
}

func TestDisasmPOPRR(t *testing.T) {
	r := disasmWith([]byte{0x4B}, 0)
	if r.Text != "POP HL" {
		t.Errorf("got %q, want POP HL", r.Text)
	}
}

func TestDisasmPUSHXRR(t *testing.T) {
	r := disasmWith([]byte{0x3A}, 0)
	if r.Text != "PUSH XDE" {
		t.Errorf("got %q, want PUSH XDE", r.Text)
	}
}

func TestDisasmPOPXRR(t *testing.T) {
	r := disasmWith([]byte{0x5B}, 0)
	if r.Text != "POP XHL" {
		t.Errorf("got %q, want POP XHL", r.Text)
	}
}

func TestDisasmINCF(t *testing.T) {
	r := disasmWith([]byte{0x0C}, 0)
	if r.Text != "INCF" {
		t.Errorf("got %q, want INCF", r.Text)
	}
}

func TestDisasmDECF(t *testing.T) {
	r := disasmWith([]byte{0x0D}, 0)
	if r.Text != "DECF" {
		t.Errorf("got %q, want DECF", r.Text)
	}
}

func TestDisasmEXFF(t *testing.T) {
	r := disasmWith([]byte{0x16}, 0)
	if r.Text != "EX F,F'" {
		t.Errorf("got %q, want EX F,F'", r.Text)
	}
}

func TestDisasmHALT(t *testing.T) {
	r := disasmWith([]byte{0x05}, 0)
	if r.Text != "HALT" {
		t.Errorf("got %q, want HALT", r.Text)
	}
}

func TestDisasmFlagOps(t *testing.T) {
	tests := []struct {
		op   byte
		want string
	}{
		{0x10, "RCF"},
		{0x11, "SCF"},
		{0x12, "CCF"},
		{0x13, "ZCF"},
	}
	for _, tt := range tests {
		r := disasmWith([]byte{tt.op}, 0)
		if r.Text != tt.want {
			t.Errorf("op $%02X: got %q, want %q", tt.op, r.Text, tt.want)
		}
	}
}

func TestDisasmRegPrefixLDImm(t *testing.T) {
	// Register prefix D8 (WA, word) + op2 $03 (LD r,#) + imm $1234 LE
	r := disasmWith([]byte{0xD8, 0x03, 0x34, 0x12}, 0)
	if r.Text != "LD.W WA,$1234" {
		t.Errorf("got %q, want LD.W WA,$1234", r.Text)
	}
	if len(r.Bytes) != 4 {
		t.Errorf("len = %d, want 4", len(r.Bytes))
	}
}

func TestDisasmRegPrefixPUSH(t *testing.T) {
	// Register prefix C8 (W, byte) + op2 $04 (PUSH)
	r := disasmWith([]byte{0xC8, 0x04}, 0)
	if r.Text != "PUSH.B W" {
		t.Errorf("got %q, want PUSH.B W", r.Text)
	}
}

func TestDisasmRegPrefixPOP(t *testing.T) {
	// Register prefix D9 (BC, word) + op2 $05 (POP)
	r := disasmWith([]byte{0xD9, 0x05}, 0)
	if r.Text != "POP.W BC" {
		t.Errorf("got %q, want POP.W BC", r.Text)
	}
}

func TestDisasmRegPrefixADDImm(t *testing.T) {
	// Register prefix CA (B, byte) + op2 $C8 (ADD r,#) + imm $10
	r := disasmWith([]byte{0xCA, 0xC8, 0x10}, 0)
	if r.Text != "ADD.B B,$10" {
		t.Errorf("got %q, want ADD.B B,$10", r.Text)
	}
}

func TestDisasmRegPrefixCPImm(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $CF (CP r,#) + imm $00
	r := disasmWith([]byte{0xC9, 0xCF, 0x00}, 0)
	if r.Text != "CP.B A,$00" {
		t.Errorf("got %q, want CP.B A,$00", r.Text)
	}
}

func TestDisasmRegPrefixINC(t *testing.T) {
	// Register prefix D8 (WA, word) + op2 $61 (INC 1,r)
	r := disasmWith([]byte{0xD8, 0x61}, 0)
	if r.Text != "INC 1,WA" {
		t.Errorf("got %q, want INC 1,WA", r.Text)
	}
}

func TestDisasmRegPrefixDEC(t *testing.T) {
	// Register prefix E8 (XWA, long) + op2 $69 (DEC 1,r)
	r := disasmWith([]byte{0xE8, 0x69}, 0)
	if r.Text != "DEC 1,XWA" {
		t.Errorf("got %q, want DEC 1,XWA", r.Text)
	}
}

func TestDisasmRegPrefixExtended(t *testing.T) {
	// Extended register prefix C7 (byte ext) + reg code $E0 (current bank A) + op2 $03 (LD) + imm $FF
	r := disasmWith([]byte{0xC7, 0xE0, 0x03, 0xFF}, 0)
	if r.Text != "LD.B A,$FF" {
		t.Errorf("got %q, want LD.B A,$FF", r.Text)
	}
}

func TestDisasmRegPrefixSCC(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $76 (SCC Z)
	r := disasmWith([]byte{0xC9, 0x76}, 0)
	if r.Text != "SCC Z,A" {
		t.Errorf("got %q, want SCC Z,A", r.Text)
	}
}

func TestDisasmRegPrefixLDRR(t *testing.T) {
	// Register prefix D8 (WA, word) + op2 $89 (LD R,r where R=BC)
	r := disasmWith([]byte{0xD8, 0x89}, 0)
	if r.Text != "LD.W BC,WA" {
		t.Errorf("got %q, want LD.W BC,WA", r.Text)
	}
}

func TestDisasmRegPrefixShift(t *testing.T) {
	// Register prefix D8 (WA, word) + op2 $E8 (RLC with count) + count $04
	r := disasmWith([]byte{0xD8, 0xE8, 0x04}, 0)
	if r.Text != "RLC.W 4,WA" {
		t.Errorf("got %q, want RLC.W 4,WA", r.Text)
	}
}

func TestDisasmRegPrefixShiftA(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $FC (SLA with A count)
	r := disasmWith([]byte{0xC9, 0xFC}, 0)
	if r.Text != "SLA.B A,A" {
		t.Errorf("got %q, want SLA.B A,A", r.Text)
	}
}

func TestDisasmRegPrefixBIT(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $33 (BIT #,r) + bit 3
	r := disasmWith([]byte{0xC9, 0x33, 0x03}, 0)
	if r.Text != "BIT 3,A" {
		t.Errorf("got %q, want BIT 3,A", r.Text)
	}
}

func TestDisasmRegPrefixSET(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $31 (SET #,r) + bit 5
	r := disasmWith([]byte{0xC9, 0x31, 0x05}, 0)
	if r.Text != "SET 5,A" {
		t.Errorf("got %q, want SET 5,A", r.Text)
	}
}

func TestDisasmRegPrefixRES(t *testing.T) {
	// Register prefix C9 (A, byte) + op2 $30 (RES #,r) + bit 7
	r := disasmWith([]byte{0xC9, 0x30, 0x07}, 0)
	if r.Text != "RES 7,A" {
		t.Errorf("got %q, want RES 7,A", r.Text)
	}
}

func TestDisasmSrcMemIndirect(t *testing.T) {
	// Source mem prefix $83 (byte, (XHL)) + op2 $20 (LD W,(mem))
	r := disasmWith([]byte{0x83, 0x20}, 0)
	if r.Text != "LD.B W,(XHL)" {
		t.Errorf("got %q, want LD.B W,(XHL)", r.Text)
	}
}

func TestDisasmSrcMemIndirectWord(t *testing.T) {
	// Source mem prefix $90 (word, (XWA)) + op2 $21 (LD BC,(mem))
	r := disasmWith([]byte{0x90, 0x21}, 0)
	if r.Text != "LD.W BC,(XWA)" {
		t.Errorf("got %q, want LD.W BC,(XWA)", r.Text)
	}
}

func TestDisasmSrcMemDisp8(t *testing.T) {
	// Source mem prefix $88 (byte, (XWA+d8)) + disp $10 + op2 $23 (LD C,(mem))
	// Byte size register code 3 = C
	r := disasmWith([]byte{0x88, 0x10, 0x23}, 0)
	if r.Text != "LD.B C,(XWA+16)" {
		t.Errorf("got %q, want LD.B C,(XWA+16)", r.Text)
	}
}

func TestDisasmSrcMemDirect8(t *testing.T) {
	// Source mem prefix $C0 (byte, direct $XX) + addr $40 + op2 $20 (LD W,(mem))
	r := disasmWith([]byte{0xC0, 0x40, 0x20}, 0)
	if r.Text != "LD.B W,($40)" {
		t.Errorf("got %q, want LD.B W,($40)", r.Text)
	}
}

func TestDisasmSrcMemDirect16(t *testing.T) {
	// Source mem prefix $C1 (byte, direct $XXXX) + addr LE $00,$40 + op2 $20 (LD W,(mem))
	r := disasmWith([]byte{0xC1, 0x00, 0x40, 0x20}, 0)
	if r.Text != "LD.B W,($4000)" {
		t.Errorf("got %q, want LD.B W,($4000)", r.Text)
	}
}

func TestDisasmSrcMemRegIndirect(t *testing.T) {
	// Source mem prefix $C3 (byte, reg indirect) + sub-mode $E0 (code&0x03==0, (R))
	// $E0: base=0xE0, reg=(0xE0>>2)&0x03=0 -> XWA (current bank)
	r := disasmWith([]byte{0xC3, 0xE0, 0x04}, 0)
	if r.Text != "PUSH.B (XWA)" {
		t.Errorf("got %q, want PUSH.B (XWA)", r.Text)
	}
}

func TestDisasmSrcMemRegIndirectDisp16(t *testing.T) {
	// Source mem prefix $D3 (word, reg indirect) + sub-mode $E1 (code&0x03==1, (R+d16))
	// $E1: base=0xE0, reg=0 -> XWA + d16=0x0010
	// op2 $20 = LD R,(mem) where R=WA (word size, code 0)
	r := disasmWith([]byte{0xD3, 0xE1, 0x10, 0x00, 0x20}, 0)
	if r.Text != "LD.W WA,(XWA+16)" {
		t.Errorf("got %q, want LD.W WA,(XWA+16)", r.Text)
	}
}

func TestDisasmSrcMemPredec(t *testing.T) {
	// Source mem prefix $C4 (byte, predec) + full reg code $EC (XHL, current bank, step=1)
	// + op2 $04 (PUSH)
	r := disasmWith([]byte{0xC4, 0xEC, 0x04}, 0)
	if r.Text != "PUSH.B (-XHL)" {
		t.Errorf("got %q, want PUSH.B (-XHL)", r.Text)
	}
}

func TestDisasmSrcMemPostinc(t *testing.T) {
	// Source mem prefix $C5 (byte, postinc) + full reg code $EC (XHL, current bank, step=1)
	// + op2 $04 (PUSH)
	r := disasmWith([]byte{0xC5, 0xEC, 0x04}, 0)
	if r.Text != "PUSH.B (XHL+)" {
		t.Errorf("got %q, want PUSH.B (XHL+)", r.Text)
	}
}

func TestDisasmSrcMemALUImm(t *testing.T) {
	// Source mem prefix $80 (byte, (XWA)) + op2 $38 (ADD (mem),#) + imm $05
	r := disasmWith([]byte{0x80, 0x38, 0x05}, 0)
	if r.Text != "ADD.B (XWA),$05" {
		t.Errorf("got %q, want ADD.B (XWA),$05", r.Text)
	}
}

func TestDisasmSrcMemINC(t *testing.T) {
	// Source mem prefix $80 (byte, (XWA)) + op2 $62 (INC 2,(mem))
	r := disasmWith([]byte{0x80, 0x62}, 0)
	if r.Text != "INC.B 2,(XWA)" {
		t.Errorf("got %q, want INC.B 2,(XWA)", r.Text)
	}
}

func TestDisasmSrcMemShift(t *testing.T) {
	// Source mem prefix $80 (byte, (XWA)) + op2 $78 (RLC (mem))
	r := disasmWith([]byte{0x80, 0x78}, 0)
	if r.Text != "RLC.B (XWA)" {
		t.Errorf("got %q, want RLC.B (XWA)", r.Text)
	}
}

func TestDisasmDstMemIndirect(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $00 (LD.B (mem),#) + imm $FF
	r := disasmWith([]byte{0xB3, 0x00, 0xFF}, 0)
	if r.Text != "LD.B (XHL),$FF" {
		t.Errorf("got %q, want LD.B (XHL),$FF", r.Text)
	}
	if len(r.Bytes) != 3 {
		t.Errorf("len = %d, want 3", len(r.Bytes))
	}
}

func TestDisasmDstMemLDWord(t *testing.T) {
	// Dst mem prefix $B0 (indirect (XWA)) + op2 $02 (LD.W (mem),#) + imm LE $34,$12
	r := disasmWith([]byte{0xB0, 0x02, 0x34, 0x12}, 0)
	if r.Text != "LD.W (XWA),$1234" {
		t.Errorf("got %q, want LD.W (XWA),$1234", r.Text)
	}
}

func TestDisasmDstMemLDReg(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $41 (LD.B (mem),A)
	r := disasmWith([]byte{0xB3, 0x41}, 0)
	if r.Text != "LD.B (XHL),A" {
		t.Errorf("got %q, want LD.B (XHL),A", r.Text)
	}
}

func TestDisasmDstMemLDRegWord(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $50 (LD.W (mem),WA)
	r := disasmWith([]byte{0xB3, 0x50}, 0)
	if r.Text != "LD.W (XHL),WA" {
		t.Errorf("got %q, want LD.W (XHL),WA", r.Text)
	}
}

func TestDisasmDstMemLDRegLong(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $60 (LD.L (mem),XWA)
	r := disasmWith([]byte{0xB3, 0x60}, 0)
	if r.Text != "LD.L (XHL),XWA" {
		t.Errorf("got %q, want LD.L (XHL),XWA", r.Text)
	}
}

func TestDisasmDstMemJP(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $D8 (JP T,(mem))
	r := disasmWith([]byte{0xB3, 0xD8}, 0)
	if r.Text != "JP T,(XHL)" {
		t.Errorf("got %q, want JP T,(XHL)", r.Text)
	}
}

func TestDisasmDstMemCALL(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $E8 (CALL T,(mem))
	r := disasmWith([]byte{0xB3, 0xE8}, 0)
	if r.Text != "CALL T,(XHL)" {
		t.Errorf("got %q, want CALL T,(XHL)", r.Text)
	}
}

func TestDisasmDstMemRET(t *testing.T) {
	// Dst mem prefix $B0 (indirect (XWA)) + op2 $F8 (RET T)
	r := disasmWith([]byte{0xB0, 0xF8}, 0)
	if r.Text != "RET T" {
		t.Errorf("got %q, want RET T", r.Text)
	}
}

func TestDisasmDstMemLDA(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $20 (LDA WA,mem)
	r := disasmWith([]byte{0xB3, 0x20}, 0)
	if r.Text != "LDA WA,(XHL)" {
		t.Errorf("got %q, want LDA WA,(XHL)", r.Text)
	}
}

func TestDisasmDstMemBIT(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $CB (BIT 3,(mem))
	r := disasmWith([]byte{0xB3, 0xCB}, 0)
	if r.Text != "BIT 3,(XHL)" {
		t.Errorf("got %q, want BIT 3,(XHL)", r.Text)
	}
}

func TestDisasmDstMemSET(t *testing.T) {
	// Dst mem prefix $B3 (indirect (XHL)) + op2 $BD (SET 5,(mem))
	r := disasmWith([]byte{0xB3, 0xBD}, 0)
	if r.Text != "SET 5,(XHL)" {
		t.Errorf("got %q, want SET 5,(XHL)", r.Text)
	}
}

func TestDisasmDstMemDisp8(t *testing.T) {
	// Dst mem prefix $B8 (disp8, (XWA+d8)) + disp $04 + op2 $00 (LD.B) + imm $AA
	r := disasmWith([]byte{0xB8, 0x04, 0x00, 0xAA}, 0)
	if r.Text != "LD.B (XWA+4),$AA" {
		t.Errorf("got %q, want LD.B (XWA+4),$AA", r.Text)
	}
}

func TestDisasmDstDirect8(t *testing.T) {
	// Dst mem prefix $F0 (direct $XX) + addr $20 + op2 $00 (LD.B) + imm $55
	r := disasmWith([]byte{0xF0, 0x20, 0x00, 0x55}, 0)
	if r.Text != "LD.B ($20),$55" {
		t.Errorf("got %q, want LD.B ($20),$55", r.Text)
	}
}

func TestDisasmDstDirect16(t *testing.T) {
	// Dst mem prefix $F1 (direct $XXXX) + addr LE $00,$80 + op2 $41 (LD.B (mem),A)
	r := disasmWith([]byte{0xF1, 0x00, 0x80, 0x41}, 0)
	if r.Text != "LD.B ($8000),A" {
		t.Errorf("got %q, want LD.B ($8000),A", r.Text)
	}
}

func TestDisasmUnknown(t *testing.T) {
	r := disasmWith([]byte{0x1F}, 0)
	if r.Text != "DB $1F" {
		t.Errorf("got %q, want DB $1F", r.Text)
	}
}

func TestDisasmLDX(t *testing.T) {
	r := disasmWith([]byte{0xF7, 0x00, 0x20, 0x00, 0xFF, 0x00}, 0)
	if r.Text != "LDX ($20),$FF" {
		t.Errorf("got %q, want LDX ($20),$FF", r.Text)
	}
	if len(r.Bytes) != 6 {
		t.Errorf("len = %d, want 6", len(r.Bytes))
	}
}

func TestDisasmCALR(t *testing.T) {
	// CALR - opcode $1E + displacement LE $FE,$FF (-2)
	// PC after reading = 3, target = 3 + (-2) = 1
	r := disasmWith([]byte{0x1E, 0xFE, 0xFF}, 0)
	if r.Text != "CALR $000001" {
		t.Errorf("got %q, want CALR $000001", r.Text)
	}
}

func TestDisasmPUSHByte(t *testing.T) {
	r := disasmWith([]byte{0x09, 0x42}, 0)
	if r.Text != "PUSH.B $42" {
		t.Errorf("got %q, want PUSH.B $42", r.Text)
	}
}

func TestDisasmPUSHWordImm(t *testing.T) {
	r := disasmWith([]byte{0x0B, 0x34, 0x12}, 0)
	if r.Text != "PUSH.W $1234" {
		t.Errorf("got %q, want PUSH.W $1234", r.Text)
	}
}

func TestDisasmLDF(t *testing.T) {
	r := disasmWith([]byte{0x17, 0x02}, 0)
	if r.Text != "LDF 2" {
		t.Errorf("got %q, want LDF 2", r.Text)
	}
}

func TestDisasmPUSHF(t *testing.T) {
	r := disasmWith([]byte{0x18}, 0)
	if r.Text != "PUSH F" {
		t.Errorf("got %q, want PUSH F", r.Text)
	}
}

func TestDisasmPOPF(t *testing.T) {
	r := disasmWith([]byte{0x19}, 0)
	if r.Text != "POP F" {
		t.Errorf("got %q, want POP F", r.Text)
	}
}

func TestDisasmRETD(t *testing.T) {
	r := disasmWith([]byte{0x0F, 0x04, 0x00}, 0)
	if r.Text != "RETD $0004" {
		t.Errorf("got %q, want RETD $0004", r.Text)
	}
}

func TestDisasmRegPrefixNEG(t *testing.T) {
	r := disasmWith([]byte{0xC9, 0x07}, 0)
	if r.Text != "NEG.B A" {
		t.Errorf("got %q, want NEG.B A", r.Text)
	}
}

func TestDisasmRegPrefixCPL(t *testing.T) {
	r := disasmWith([]byte{0xC9, 0x06}, 0)
	if r.Text != "CPL.B A" {
		t.Errorf("got %q, want CPL.B A", r.Text)
	}
}

func TestDisasmRegPrefixEXTZ(t *testing.T) {
	r := disasmWith([]byte{0xD8, 0x12}, 0)
	if r.Text != "EXTZ WA" {
		t.Errorf("got %q, want EXTZ WA", r.Text)
	}
}

func TestDisasmRegPrefixDJNZ(t *testing.T) {
	// Register prefix D8 (WA, word) + op2 $1C (DJNZ) + disp $FC (-4)
	// PC after reading: 3, target: 3 + (-4) = -1 -> masked to $FFFFFF
	r := disasmWith([]byte{0xD8, 0x1C, 0xFC}, 0)
	if r.Text != "DJNZ.W WA,$FFFFFF" {
		t.Errorf("got %q, want DJNZ.W WA,$FFFFFF", r.Text)
	}
}

func TestDisasmAddrFromStart(t *testing.T) {
	r := disasmWith([]byte{0x00}, 0xFF0010)
	if r.Addr != 0xFF0010 {
		t.Errorf("Addr = %06X, want FF0010", r.Addr)
	}
}

func TestDisasmSrcMemLDI(t *testing.T) {
	r := disasmWith([]byte{0x80, 0x10}, 0)
	if r.Text != "LDI.B (XWA)" {
		t.Errorf("got %q, want LDI.B (XWA)", r.Text)
	}
}

func TestDisasmSrcMemCPI(t *testing.T) {
	r := disasmWith([]byte{0x90, 0x14}, 0)
	if r.Text != "CPI.W (XWA)" {
		t.Errorf("got %q, want CPI.W (XWA)", r.Text)
	}
}

func TestDisasmSrcMemLDAddr(t *testing.T) {
	// Source mem prefix $80 (byte, (XWA)) + op2 $19 (LD (addr),mem) + addr LE $00,$40
	r := disasmWith([]byte{0x80, 0x19, 0x00, 0x40}, 0)
	if r.Text != "LD.B ($4000),(XWA)" {
		t.Errorf("got %q, want LD.B ($4000),(XWA)", r.Text)
	}
}

func TestDisasmSrcMemEX(t *testing.T) {
	r := disasmWith([]byte{0x80, 0x30}, 0)
	if r.Text != "EX.B (XWA),W" {
		t.Errorf("got %q, want EX.B (XWA),W", r.Text)
	}
}
