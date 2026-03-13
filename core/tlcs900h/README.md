# TLCS-900/H CPU

Clean room implementation of the Toshiba TLCS-900/H processor core used
by the Neo Geo Pocket Color (TMP95C061 SoC).


## File Layout

| File | Purpose |
|------|---------|
| bus.go | Bus interface for memory access |
| cpu.go | CPU struct, Reset, Step, fetch helpers, push/pop, register read/write wrappers |
| registers.go | Register bank struct, Registers struct (4 banks + dedicated regs + SR) |
| registers_access.go | Register access methods: 3-bit codes, 4-bit byte codes, full 8-bit extended codes |
| size.go | Size type (Byte/Word/Long) with Mask, MSB, Bits helpers |
| flags.go | Flag constants, condition code evaluation, flag-setting helpers (arith, logic, shift) |
| decode.go | Instruction decoder: baseOps + prefix dispatch to regOps/srcMemOps/dstMemOps |
| ops_arith.go | ADD, ADC, SUB, SBC, CP, INC, DEC, NEG, MUL, MULS, MULA, DIV, DIVS, DAA, EXTZ, EXTS, PAA, MINC, MDEC |
| ops_load.go | LD, LDA, PUSH, POP, EX, LDI, LDD, LDIR, LDDR, LDC, LDX, LINK, UNLK, INCF, DECF, LDF |
| ops_logic.go | AND, OR, XOR |
| ops_bit.go | BIT, SET, RES, CHG, TSET, ANDCF, ORCF, XORCF, LDCF, STCF, BS1F, BS1B, CPL, RCF, SCF, CCF, ZCF |
| ops_shift.go | RLC, RRC, RL, RR, SLA, SRA, SLL, SRL, RLD, RRD |
| ops_branch.go | JP, JR, JRL, CALL, CALR, RET, RETD, RETI, DJNZ, SCC |
| ops_cmp_block.go | CPI, CPIR, CPD, CPDR |
| ops_ctrl.go | NOP, HALT, EI/DI, SWI, MIRR |
| interrupt.go | Interrupt check, service, and vector dispatch |
| disasm.go | Instruction disassembler (used by biosstep tool) |
| serialize.go | Save/restore state for save states |


## Decode Pipeline

Every instruction begins with a single byte fetch at PC. That byte either
encodes a complete standalone instruction or acts as a prefix that resolves
an operand before fetching a second opcode byte.

```
fetch first byte
  |
  +-- standalone (baseOps) --> execute directly
  |
  +-- register prefix -------> set opSize, opReg --> fetch second byte --> regOps[op2]
  |
  +-- source memory prefix ---> set opSize, resolve opAddr --> fetch second byte --> srcMemOps[op2]
  |
  +-- dest memory prefix -----> resolve opAddr --> fetch second byte --> dstMemOps[op2]
```

There are four dispatch tables, each 256 entries:

- **baseOps[256]**: First-byte dispatch. Standalone instructions and prefix handlers.
- **regOps[256]**: Second-byte dispatch after a register prefix.
- **srcMemOps[256]**: Second-byte dispatch after a source memory prefix.
- **dstMemOps[256]**: Second-byte dispatch after a destination memory prefix.


## Prefix Types

### Register Prefixes

Set `opSize` and `opReg`, then dispatch through `regOps`.

| Opcode Range | Size | Encoding |
|--------------|------|----------|
| C8-CF | Byte | 3-bit register code in bits 2-0 of prefix byte |
| D8-DF | Word | 3-bit register code in bits 2-0 of prefix byte |
| E8-EF | Long | 3-bit register code in bits 2-0 of prefix byte |
| C7 | Byte | Extended: full 8-bit register code follows as next byte |
| D7 | Word | Extended: full 8-bit register code follows as next byte |
| E7 | Long | Extended: full 8-bit register code follows as next byte |

### Source Memory Prefixes

Resolve a memory address into `opAddr`, then dispatch through `srcMemOps`.

| Opcode Range | Size | Addressing Mode |
|--------------|------|-----------------|
| 80-87 | Byte | (R) register indirect |
| 88-8F | Byte | (R+d8) register + 8-bit displacement |
| 90-97 | Word | (R) register indirect |
| 98-9F | Word | (R+d8) register + 8-bit displacement |
| A0-A7 | Long | (R) register indirect |
| A8-AF | Long | (R+d8) register + 8-bit displacement |
| C0 / D0 / E0 | B/W/L | (#8) 8-bit immediate address |
| C1 / D1 / E1 | B/W/L | (#16) 16-bit immediate address |
| C2 / D2 / E2 | B/W/L | (#24) 24-bit immediate address |
| C3 / D3 / E3 | B/W/L | Register indirect sub-modes (see below) |
| C4 / D4 / E4 | B/W/L | Pre-decrement |
| C5 / D5 / E5 | B/W/L | Post-increment |

### Destination Memory Prefixes

Resolve a memory address into `opAddr`, then dispatch through `dstMemOps`.
Size is determined by the second opcode byte, not the prefix.

| Opcode Range | Addressing Mode |
|--------------|-----------------|
| B0-B7 | (R) register indirect |
| B8-BF | (R+d8) register + 8-bit displacement |
| F0 | (#8) 8-bit immediate address |
| F1 | (#16) 16-bit immediate address |
| F2 | (#24) 24-bit immediate address |
| F3 | Register indirect sub-modes |
| F4 | Pre-decrement |
| F5 | Post-increment |

### Register Indirect Sub-modes (x3 prefix)

The byte following the x3 prefix selects the sub-mode:

| code & 0x03 | Mode |
|-------------|------|
| 0x00 | (R) - register value as address |
| 0x01 | (R+d16) - register + 16-bit signed displacement |
| 0x03 | Sub-sub modes: 0x03=(R+r8), 0x07=(R+r16), 0x13=(PC+d16) |

### Pre-decrement / Post-increment

The register code byte encodes the 32-bit register in bits 7-2 and the
step size in bits 1-0: 0=1 byte, 1=2 bytes, 2=4 bytes.


## Register Encoding

### 3-bit Register Codes (R)

Used in first-byte opcodes and regOps second bytes. Maps to the current
bank (selected by RFP in SR) for codes 0-3, or dedicated registers for 4-7.

| Code | 8-bit | 16-bit | 32-bit |
|------|-------|--------|--------|
| 000 | W | WA | XWA |
| 001 | A | BC | XBC |
| 010 | B | DE | XDE |
| 011 | C | HL | XHL |
| 100 | D | IX | XIX |
| 101 | E | IY | XIY |
| 110 | H | IZ | XIZ |
| 111 | L | SP | XSP |

Note: 8-bit codes use a remapping table (`r8From3bit`) because the 3-bit
code interleaves high/low bytes: bit 0 selects high(0)/low(1) byte of the
16-bit word, bits 2-1 select the register pair.

### Extended Register Codes (r)

Used after C7/D7/E7 prefixes. Full 8-bit code addresses any register in
any bank. Bits 7-2 identify the 32-bit register, bits 1-0 select the
sub-register:

- **Byte access**: bits 1-0 select byte 0-3 within the 32-bit register
- **Word access**: bit 1 selects low(0) or high(1) word
- **Long access**: bits 1-0 ignored

Code ranges for bits 7-2:

| Range | Register Set |
|-------|-------------|
| 0x00-0x0C | Bank 0: XWA0, XBC0, XDE0, XHL0 |
| 0x10-0x1C | Bank 1: XWA1, XBC1, XDE1, XHL1 |
| 0x20-0x2C | Bank 2: XWA2, XBC2, XDE2, XHL2 |
| 0x30-0x3C | Bank 3: XWA3, XBC3, XDE3, XHL3 |
| 0xD0-0xDC | Previous bank (RFP-1) |
| 0xE0-0xEC | Current bank (RFP) |
| 0xF0 | XIX |
| 0xF4 | XIY |
| 0xF8 | XIZ |
| 0xFC | XSP |

Within each 4-register group, bits 3-2 select: 0=XWA, 1=XBC, 2=XDE, 3=XHL.


## Transient Operand State

The prefix handlers store decoded operand info in CPU fields that are
consumed by the second-byte handler within the same instruction. These
are not serialized.

| Field | Set By | Used By |
|-------|--------|---------|
| opSize | Register and source memory prefixes | regOps and srcMemOps handlers |
| opReg | Register prefixes | regOps handlers (via readOpReg/writeOpReg) |
| opRegEx | Register prefixes (true for C7/D7/E7) | readOpReg/writeOpReg to select 3-bit vs extended access |
| opAddr | Memory prefixes | srcMemOps/dstMemOps handlers |
| opMemReg | (R) indirect source prefix | LDI/LDD/LDIR/LDDR to select pointer registers |

For destination memory prefixes, `opSize` is set to 0 by the prefix.
The second-byte handler determines operand size from its own encoding.


## Flags

Lower 8 bits of SR. Bits 5 and 3 are reserved (always 0).

| Bit | Flag | Description |
|-----|------|-------------|
| 7 | S | Sign |
| 6 | Z | Zero |
| 4 | H | Half-carry |
| 2 | V | Overflow / Parity |
| 1 | N | Subtract |
| 0 | C | Carry |

Flag-setting is grouped by operation type:
- **Arithmetic** (setFlagsArith): S, Z, H, V(overflow), N, C all set
- **Logic** (setFlagsLogic): S, Z set; H=1 for AND, 0 for OR/XOR; V=parity; N=0, C=0
- **Shift** (setFlagsShift): S, Z set; H=0; V=parity; N=0; C=last bit shifted out
- **INC/DEC**: Same as arithmetic but C is preserved
- **INC/DEC word/long**: No flag changes (databook CPU900H-70/83)

Parity is computed for byte and word sizes. For long, parity is undefined
and left as 0.


## Condition Codes

16 condition codes (4-bit encoding) used by JR, JRL, JP cc, CALL cc, RET cc, SCC:

| Code | Mnemonic | Condition |
|------|----------|-----------|
| 0 | F | Always false |
| 1 | LT | S xor V |
| 2 | LE | (S xor V) or Z |
| 3 | ULE | C or Z |
| 4 | OV | V |
| 5 | MI | S |
| 6 | EQ | Z |
| 7 | ULT | C |
| 8 | T | Always true |
| 9 | GE | S == V |
| A | GT | (S == V) and not Z |
| B | UGT | not C and not Z |
| C | NOV | not V |
| D | PL | not S |
| E | NE | not Z |
| F | UGE | not C |


## Interrupts

- `RequestInterrupt(level, vector)` queues an interrupt
- `checkInterrupt()` runs before each Step; accepts if level >= IFF (or level 7 NMI)
- On service: push SR + PC (6 bytes), set IFF = level+1 (capped at 7), read handler
  address from vector table at 0xFFFF00 + vector*4, increment intNest
- RETI pops SR + PC, decrements intNest


## Custom Opcode Hooks

External code (HLE BIOS) can register handlers for any primary opcode byte
via `RegisterOp(opcode, handler)`. Custom handlers take precedence over
baseOps during execute.


## Bus Interface

- **Bus**: `Read(Size, addr)` / `Write(Size, addr, val)` / `Reset()`

All internal reads/writes go through `readBus`/`writeBus` which mask
addresses to 24 bits. Peripherals are ticked at instruction boundaries
by the caller via the Memory.Tick method.


## Cycle Budgeting

`StepCycles(budget)` executes one instruction and tracks overshoot via a
`deficit` field. If an instruction costs more than the budget, the excess
is carried forward and consumed from the next budget before executing
another instruction.


## Not Implemented

- **LDAR** (F3:13:d16:20+zz+R): Load Address Relative. Encoding conflicts
  with prefixDstPreDec (0xF3) and would require special-case detection in
  the F3 prefix handler. Not used by NGPC software due to fixed memory map.
- **DMA control registers**: The TLCS-900/H has a 4-channel DMA controller
  with source (DMAS0-3, cr 0x00-0x0C), destination (DMAD0-3, cr 0x10-0x1C),
  count/mode (DMAC0-3/DMAM0-3, cr 0x20-0x2C) registers accessible via
  LDC instructions. These are not implemented because the NGPC does not use
  the DMA controller. LDC reads to these registers return 0 and writes are
  silently discarded.
