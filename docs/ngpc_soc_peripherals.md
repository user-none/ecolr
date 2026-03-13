# TMP95CS64F SoC Peripheral Register Reference

Register-level detail for the TMP95CS64F SoC peripherals used by the Neo Geo
Pocket Color. This document covers the 16-bit timers, micro DMA control
registers, clock gear control, and RTC registers.

For system-level descriptions of how these peripherals are used (timer
purposes, DMA usage patterns, interrupt dispatch), see
[ngpc_system_reference.md](ngpc_system_reference.md).

The TMP95CS64F is a custom SNK variant closely related to the Toshiba
TMP95C061. Register addresses and behavior are derived from the TMP95C061
datasheet and cross-referenced against MAME, Mednafen, RACE, and Neopop-SDL
emulator implementations.

---

## Table of Contents

- [16-Bit Timers (T4-T7)](#16-bit-timers-t4-t7)
- [Micro DMA (HDMA) Registers](#micro-dma-hdma-registers)
- [Clock Gear Control](#clock-gear-control)
- [Real-Time Clock Registers](#real-time-clock-registers)
- [Interrupt Priority Registers](#interrupt-priority-registers)
- [Sources](#sources)

---

## 16-Bit Timers (T4-T7)

### Architecture

The TMP95CS64F contains two 16-bit timer channels in addition to the four
8-bit timers (T0-T3) documented in the system reference. Each 16-bit channel
consists of a pair of compare/reload registers and a pair of capture registers:

| Channel | Compare Registers | Capture Registers | Mode Register | Flip-Flop Control |
|---------|-------------------|-------------------|---------------|-------------------|
| T4/T5 | TREG4, TREG5 | CAP1, CAP2 | T4MOD ($38) | T4FFCR ($39) |
| T6/T7 | TREG6, TREG7 | CAP3, CAP4 | T5MOD ($48) | T5FFCR ($49) |

Note: The naming follows the TMP95C061 convention. T4MOD controls the T4/T5
pair. T5MOD controls the T6/T7 pair.

### TRUN Register ($20) - Timer Run Control

| Bit | Name | Description |
|-----|------|-------------|
| 7 | PRESCALE | Prescaler enable. Must be set for the prescaler to run. |
| 6 | -- | Reserved |
| 5 | T5RUN | Timer T6/T7 channel run. Clearing resets the internal counter. |
| 4 | T4RUN | Timer T4/T5 channel run. Clearing resets the internal counter. |
| 3 | T3RUN | Timer 3 run (8-bit) |
| 2 | T2RUN | Timer 2 run (8-bit) |
| 1 | T1RUN | Timer 1 run (8-bit) |
| 0 | T0RUN | Timer 0 run (8-bit) |

Reset value: $00 (all timers stopped, prescaler off).

### T4/T5 Channel Registers ($30-$3A)

| Address | Register | Size | R/W | Description |
|---------|----------|------|-----|-------------|
| $30 | TREG4L | Byte | W | Timer 4 compare/reload value (low byte) |
| $31 | TREG4H | Byte | W | Timer 4 compare/reload value (high byte) |
| $32 | TREG5L | Byte | W | Timer 5 compare/reload value (low byte) |
| $33 | TREG5H | Byte | W | Timer 5 compare/reload value (high byte) |
| $34 | CAP1L | Byte | R | Capture register 1 (low byte) |
| $35 | CAP1H | Byte | R | Capture register 1 (high byte) |
| $36 | CAP2L | Byte | R | Capture register 2 (low byte) |
| $37 | CAP2H | Byte | R | Capture register 2 (high byte) |
| $38 | T4MOD | Byte | R/W | Timer 4 mode register |
| $39 | T4FFCR | Byte | R/W | Timer 4 flip-flop control register |
| $3A | T45CR | Byte | R/W | Timer 4/5 control register |

### T6/T7 Channel Registers ($40-$49)

| Address | Register | Size | R/W | Description |
|---------|----------|------|-----|-------------|
| $40 | TREG6L | Byte | W | Timer 6 compare/reload value (low byte) |
| $41 | TREG6H | Byte | W | Timer 6 compare/reload value (high byte) |
| $42 | TREG7L | Byte | W | Timer 7 compare/reload value (low byte) |
| $43 | TREG7H | Byte | W | Timer 7 compare/reload value (high byte) |
| $44 | CAP3L | Byte | R | Capture register 3 (low byte) |
| $45 | CAP3H | Byte | R | Capture register 3 (high byte) |
| $46 | CAP4L | Byte | R | Capture register 4 (low byte) |
| $47 | CAP4H | Byte | R | Capture register 4 (high byte) |
| $48 | T5MOD | Byte | R/W | Timer 5 mode register (controls T6/T7 channel) |
| $49 | T5FFCR | Byte | R/W | Timer 5 flip-flop control register |

### T4MOD / T5MOD - Timer Mode Registers ($38, $48)

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | T4xMOD | Operating mode select |
| 5-4 | T4xCAPB | Capture trigger B select (bit 5 forced to 1 on write) |
| 3-2 | T4xCAPA | Capture trigger A select |
| 1-0 | T4xCLK | Clock source select |

Clock source (bits 1-0):

| Value | Source |
|-------|--------|
| 00 | External input (TI4 or TI6) |
| 01 | T1 (prescaler oT1, 128 CPU state divisor) |
| 10 | T4 (prescaler oT4, 512 CPU state divisor) |
| 11 | T16 (prescaler oT16, 2048 CPU state divisor) |

Reset value: $20 (bit 5 forced set).

### T4FFCR / T5FFCR - Flip-Flop Control Registers ($39, $49)

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | FF5C | Flip-flop 5 (or 7) control (forced to 11 on write) |
| 5-4 | FF5 | Inversion trigger select for FF5 (or FF7) |
| 3-2 | FF4 | Inversion trigger select for FF4 (or FF6) |
| 1-0 | FF4C | Flip-flop 4 (or 6) control (forced to 11 on write) |

Flip-flop control values (bits 7-6 and 1-0):

| Value | Action |
|-------|--------|
| 00 | Invert |
| 01 | Set |
| 10 | Clear |
| 11 | Don't change (written on store) |

Reset value: $00. On write, bits 7-6 and 1-0 are forced to 11, so the
effective written value is always OR'd with $C3.

### T45CR - Timer 4/5 Control Register ($3A)

Controls PWM output and capture input selection for the T4/T5 16-bit timer
pair. Reset value: $00.

### Compare/Reload Register Access

The 16-bit compare registers (TREG4-TREG7) are written byte-by-byte through
two consecutive SFR addresses. The even address writes the low byte and the
odd address writes the high byte of the same 16-bit register:

- TREG4 = (TREG4H << 8) | TREG4L
- TREG5 = (TREG5H << 8) | TREG5L
- TREG6 = (TREG6H << 8) | TREG6L
- TREG7 = (TREG7H << 8) | TREG7L

### Capture Register Access

The capture registers (CAP1-CAP4) are read-only. Each is read byte-by-byte
through two consecutive SFR addresses in the same low/high byte pattern.

### 16-Bit Timer Interrupts

| Interrupt | Vector Address | Priority Register | Priority Bits |
|-----------|---------------|-------------------|---------------|
| INTTR4 | $FFFF50 | INTET45 ($75) | Bits 2-0 |
| INTTR5 | $FFFF54 | INTET45 ($75) | Bits 6-4 |
| INTTR6 | $FFFF58 | INTET67 ($76) | Bits 2-0 |
| INTTR7 | $FFFF5C | INTET67 ($76) | Bits 6-4 |

### Implementation Status

Neither MAME nor Mednafen fully implements the 16-bit timer counting and
interrupt generation logic for the TMP95C061. MAME has the register
read/write infrastructure but the timer tick logic in the execution loop
only covers T0-T3. Mednafen does not handle the $30-$49 address range at
all. Games may not exercise these timers heavily since the 8-bit timers
cover most NGPC timing needs.

---

## Micro DMA (HDMA) Registers

### Register Address Space

The micro DMA registers use two different address spaces:

1. **Control Register (CR) space** - Source, destination, count, and mode
   registers. Accessed via LDC instructions (not standard memory-mapped I/O).
2. **SFR space** - Start vector registers at $7C-$7F (standard
   memory-mapped I/O).

### CR Space Registers

Accessed via `LDC cr,r` (write) and `LDC r,cr` (read) instructions.

#### Source Address (DMAS) - 32-bit

| Register | CR Offset | Description |
|----------|-----------|-------------|
| DMAS0 | $00 | Channel 0 source address |
| DMAS1 | $04 | Channel 1 source address |
| DMAS2 | $08 | Channel 2 source address |
| DMAS3 | $0C | Channel 3 source address |

#### Destination Address (DMAD) - 32-bit

| Register | CR Offset | Description |
|----------|-----------|-------------|
| DMAD0 | $10 | Channel 0 destination address |
| DMAD1 | $14 | Channel 1 destination address |
| DMAD2 | $18 | Channel 2 destination address |
| DMAD3 | $1C | Channel 3 destination address |

#### Transfer Count (DMAC) - 16-bit

| Register | CR Offset | Description |
|----------|-----------|-------------|
| DMAC0 | $20 | Channel 0 transfer count |
| DMAC1 | $24 | Channel 1 transfer count |
| DMAC2 | $28 | Channel 2 transfer count |
| DMAC3 | $2C | Channel 3 transfer count |

#### Mode (DMAM) - 8-bit

| Register | CR Offset | Description |
|----------|-----------|-------------|
| DMAM0 | $22 | Channel 0 mode |
| DMAM1 | $26 | Channel 1 mode |
| DMAM2 | $2A | Channel 2 mode |
| DMAM3 | $2E | Channel 3 mode |

Note the interleaving: DMAC and DMAM share a 4-byte slot per channel. DMAC
occupies the first 2 bytes and DMAM occupies byte 2 within each slot.

### DMAM - Mode Register Bit Fields

| Bits | Field | Description |
|------|-------|-------------|
| 7-5 | -- | Reserved |
| 4-2 | MODE | Transfer mode (address behavior) |
| 1-0 | SIZE | Transfer size |

#### Transfer Size (bits 1-0)

| Value | Size |
|-------|------|
| 00 | Byte (8-bit) |
| 01 | Word (16-bit) |
| 10 | Long (32-bit) |
| 11 | Reserved |

#### Transfer Mode (bits 4-2)

| Value | Source Address | Destination Address | Description |
|-------|---------------|---------------------|-------------|
| 000 | Fixed | Increment | I/O to memory (destination increments) |
| 001 | Fixed | Decrement | I/O to memory (destination decrements) |
| 010 | Increment | Fixed | Memory to I/O (source increments) |
| 011 | Decrement | Fixed | Memory to I/O (source decrements) |
| 100 | Fixed | Fixed | Fixed address transfer |
| 101 | Increment | -- | Counter mode (no transfer, source increments as counter) |

### Start Vector Registers (SFR Space)

These are standard memory-mapped I/O registers:

| Address | Register | Description |
|---------|----------|-------------|
| $7C | DMA0V | Channel 0 start vector |
| $7D | DMA1V | Channel 1 start vector |
| $7E | DMA2V | Channel 2 start vector |
| $7F | DMA3V | Channel 3 start vector |

The start vector value determines which interrupt source triggers the DMA
channel. The value stored is the hardware interrupt vector divided by 4:
`DMA_V = interrupt_vector >> 2`.

Common start vector assignments on the NGPC:

| Start Vector | Interrupt Source | Typical Usage |
|--------------|-----------------|---------------|
| $0A | INT0 | RTC alarm |
| $0B | INT4 (VBlank) | Frame-synchronized transfers |
| $0C | INT5 (Z80) | Sound data transfer |
| $10 | INTT0 (Timer 0) | HBlank-driven raster effects |
| $11 | INTT1 (Timer 1) | General purpose |
| $12 | INTT2 (Timer 2) | DAC sample streaming |
| $13 | INTT3 (Timer 3) | Sound timing |
| $14 | INTTR4 | 16-bit timer 4 |
| $15 | INTTR5 | 16-bit timer 5 |

### DMA Execution Behavior

1. HDMA is checked after every CPU instruction
2. HDMA only executes if IFF is not at maximum mask level
3. Channels are checked in priority order: 0, 1, 2, 3
4. One transfer step occurs per check (single-step, not burst)
5. After each step, DMAC is decremented by 1
6. When DMAC reaches 0:
   - The start vector register (DMA0V-DMA3V) is cleared to 0 (disabling
     further triggers until reprogrammed)
   - The corresponding INTTC interrupt is generated (INTTC0-INTTC3)
7. The interrupt flip-flop that triggered the DMA is cleared after the
   transfer step

### Transfer Cycle Costs

| Transfer Size | Cycles |
|---------------|--------|
| Byte | 8 |
| Word | 8 |
| Long | 12 |
| Counter mode | 5 |

---

## Clock Gear Control

### Overview

The CPU clock speed is adjustable via the clock gear system. This divides
the 6.144 MHz master oscillator to reduce CPU speed and power consumption.

| Gear | Divisor | CPU Clock |
|------|---------|-----------|
| 0 | /1 | 6.144 MHz |
| 1 | /2 | 3.072 MHz |
| 2 | /4 | 1.536 MHz |
| 3 | /8 | 768 kHz |
| 4 | /16 | 384 kHz |

### Hardware Register

The clock gear control register address and bit definitions are unknown.
Neither the TMP95C061 nor TMP95CS64F datasheets contain a SYSCR or clock
gear register. The newer TLCS-900/H1 series (32-bit TMP92C parts) has
SYSCR1 with GEAR bits, but this is a different chip family.

All four reference emulators (MAME, Mednafen, RACE, Neopop-SDL) implement
clock gear via BIOS high-level emulation rather than hardware register
emulation. Since our emulator runs the real BIOS ROM, the BIOS will write
to whatever hardware register controls the clock gear. Identifying this
register requires reverse-engineering the BIOS ROM's VECT_CLOCKGEARSET
routine.

### BIOS Interface

Games change the clock gear through the BIOS system call
`VECT_CLOCKGEARSET` at $FFFE04. The gear value (0-4) is passed in CPU
register RB3.

### Effect on Timers

The prescaler is fed from the oscillator's 1/4 divider, independent of the
clock gear. All prescaler-derived timer clock sources (T1, T4, T16, T256)
run at fixed rates regardless of the CPU clock gear setting:

| Source | Datasheet | CPU State Divisor | Frequency (6.144 MHz osc) |
|--------|-----------|-------------------|---------------------------|
| T1     | fc/8      | 128               | 48 kHz                    |
| T4     | fc/32     | 512               | 12 kHz                    |
| T16    | fc/128    | 2048              | 3 kHz                     |
| T256   | fc/2048   | 32768             | 187.5 Hz                  |

The Z80 sound CPU clock (3.072 MHz) is also independent of the clock gear.

---

## Real-Time Clock Registers

### Overview

The RTC registers occupy SFR addresses $90-$97. These are specific to the
TMP95CS64F custom variant and are not part of the standard TMP95C061 SFR
map (which ends at $7F).

### Register Map

| Address | Name | Contents | Format | R/W |
|---------|------|----------|--------|-----|
| $90 | -- | Control / latch trigger | -- | See notes |
| $91 | RTC_YEAR | Year (00-99) | BCD | R/W |
| $92 | RTC_MONTH | Month (01-12) | BCD | R/W |
| $93 | RTC_DAY | Day of month (01-31) | BCD | R/W |
| $94 | RTC_HOUR | Hour (00-23, 24-hour format) | BCD | R/W |
| $95 | RTC_MIN | Minute (00-59) | BCD | R/W |
| $96 | RTC_SEC | Second (00-59) | BCD | R/W |
| $97 | RTC_LDOW | Leap year offset / day of week | Packed nibble | R |

### Register Details

**$90 - RTC Control**

BIOS disassembly confirms the BIOS writes to this register during boot:
after setting the default time values ($91-$97), it clears bit 1
(`AND $90,$FD`) then sets bit 0 (`OR $90,$01`). This suggests:

| Bit | Function |
|-----|----------|
| 0 | RTC enable/run (set after writing time values) |
| 1 | Unknown mode bit (cleared before enabling) |
| 7-2 | Unknown |

Mednafen uses a read of $91 (not $90) as a latch trigger to atomically
snapshot all time values. MAME does not handle $90 specially. RACE and
Neopop-SDL access the address range directly without latching. This
register returns 0 on read in all examined implementations.

**$91 - Year (BCD)**

Two-digit year in BCD. Interpretation: values 00-90 represent 2000-2090,
values 91-99 represent 1991-1999. Derived from the C `tm_year` field as
`(tm_year - 100)` converted to BCD.

**$92 - Month (BCD)**

Month of year in BCD. Range: $01-$12.

**$93 - Day (BCD)**

Day of month in BCD. Range: $01-$31.

**$94 - Hour (BCD)**

Hour in 24-hour format, BCD. Range: $00-$23.

**$95 - Minute (BCD)**

Minute in BCD. Range: $00-$59.

**$96 - Second (BCD)**

Second in BCD. Range: $00-$59.

**$97 - Leap Year Offset / Day of Week**

This register is NOT BCD. It contains two independent 4-bit fields:

| Bits | Field | Description |
|------|-------|-------------|
| 7-4 | LEAP | Years since last leap year (0-3). Computed as year % 4. |
| 3-0 | DOW | Day of week. 0=Sunday, 1=Monday, ... 6=Saturday. |

This register appears to be read-only (derived from the other time
registers). No emulator implements a write path for it. MAME's RTC callback
does not update this register at all, suggesting it may be computed by the
BIOS rather than maintained by hardware.

### Latch Behavior

Mednafen implements an atomic latch: reading address $91 snapshots all
time values into an internal buffer, and reads of $92-$97 return values
from that snapshot. This prevents time fields from rolling over mid-read
(e.g., reading 23:59:59 where the seconds roll to 00 between reads).

Whether the real hardware has a similar latch mechanism is unverified. The
BIOS VECT_RTCGET call reads $91 through $97 sequentially, which would
benefit from atomic latching.

### BIOS Access (VECT_RTCGET)

The BIOS system call at $FFFE08 reads addresses $91-$97 and copies the 7
bytes to a buffer address specified in register XHL3:

| Buffer Offset | Source | Contents |
|---------------|--------|----------|
| +0 | $91 | Year (BCD) |
| +1 | $92 | Month (BCD) |
| +2 | $93 | Day (BCD) |
| +3 | $94 | Hour (BCD) |
| +4 | $95 | Minute (BCD) |
| +5 | $96 | Second (BCD) |
| +6 | $97 | Leap offset / day of week |

### Alarm

The NGPC supports RTC alarm functionality (INT0 is used as the alarm
interrupt). However, none of the examined emulators implement the alarm
registers. The BIOS calls VECT_ALARMSET ($FFFE14) and VECT_ALARMDOWNSET
($FFFE18) are stubbed in all implementations. Alarm register addresses and
bit definitions are unknown.

---

## Interrupt Priority Registers

Detailed bit layout for the interrupt priority/enable registers at $70-$7B.
Each register contains two 3-bit priority fields and two interrupt pending
flags packed into a single byte.

### Register Bit Layout

Each interrupt priority register has the same format:

| Bits | Field | Description |
|------|-------|-------------|
| 7 | PEND_H | High interrupt pending flag (write 0 to clear) |
| 6-4 | PRI_H | High interrupt priority (0 = disabled, 1-6 = priority, 7 = NMI) |
| 3 | PEND_L | Low interrupt pending flag (write 0 to clear) |
| 2-0 | PRI_L | Low interrupt priority (0 = disabled, 1-6 = priority, 7 = NMI) |

### Priority Register Assignments

| Address | Register | Low (bits 2-0) | High (bits 6-4) |
|---------|----------|----------------|-----------------|
| $70 | INTE0AD | INT0 | INTAD |
| $71 | INTE45 | INT4 (VBlank) | INT5 (Z80) |
| $72 | INTE67 | INT6 | INT7 |
| $73 | INTET01 | INTT0 (Timer 0) | INTT1 (Timer 1) |
| $74 | INTET23 | INTT2 (Timer 2) | INTT3 (Timer 3) |
| $75 | INTET45 | INTTR4 (Timer 4) | INTTR5 (Timer 5) |
| $76 | INTET67 | INTTR6 (Timer 6) | INTTR7 (Timer 7) |
| $77 | INTES0 | INTRX0 (Serial 0 RX) | INTTX0 (Serial 0 TX) |
| $78 | INTES1 | INTRX1 (Serial 1 RX) | INTTX1 (Serial 1 TX) |
| $79 | INTETC01 | INTTC0 (DMA 0 end) | INTTC1 (DMA 1 end) |
| $7A | INTETC23 | INTTC2 (DMA 2 end) | INTTC3 (DMA 3 end) |
| $7B | IIMC | Interrupt Input Mode Control (different format) |

### IIMC - Interrupt Input Mode Control ($7B)

This register controls the edge/level sensitivity of external interrupts.
Bit definitions vary from the standard priority register format. Consult
the TMP95C061 datasheet for details.

### DMA Start Vector to Interrupt Mapping

The full mapping from DMA start vector values to interrupt sources. The
start vector is the hardware interrupt vector address divided by 4.

| Start Vector | Hardware Vector | Interrupt Source |
|--------------|----------------|-----------------|
| $0A | $28 | INT0 |
| $0B | $2C | INT4 (VBlank) |
| $0C | $30 | INT5 (Z80) |
| $0D | $34 | INT6 |
| $0E | $38 | INT7 |
| $10 | $40 | INTT0 |
| $11 | $44 | INTT1 |
| $12 | $48 | INTT2 |
| $13 | $4C | INTT3 |
| $14 | $50 | INTTR4 |
| $15 | $54 | INTTR5 |
| $16 | $58 | INTTR6 |
| $17 | $5C | INTTR7 |
| $18 | $60 | INTRX0 |
| $19 | $64 | INTTX0 |
| $1A | $68 | INTRX1 |
| $1B | $6C | INTTX1 |
| $1C | $70 | INTAD |

INTTC0-INTTC3 (DMA completion interrupts) cannot trigger DMA.

---

## Sources

- [TMP95C061 Datasheet (bitsavers.org)](http://www.bitsavers.org/components/toshiba/_dataSheet/TMP95c061-ds.pdf) -
  SFR register map, timer system, DMA control registers, interrupt
  controller, clock gear
- [1994 Toshiba TLCS-900 16-Bit Microcontroller Databook (bitsavers.org)](http://bitsavers.trailing-edge.com/components/toshiba/_dataBook/1994_Toshiba_TLCS-900_16_Bit_Microcontroller.pdf) -
  CPU architecture, control register space, LDC instruction
- MAME TMP95C061 implementation (register addresses, bit field behavior,
  DMA execution model)
- Mednafen NGP implementation (DMA handling, RTC latch mechanism, timer
  usage)
- RACE implementation (DMA vector mapping, RTC register access, clock gear
  multiplier)
- Neopop-SDL implementation (RTC register formatting)
