# Neo Geo Pocket Color System Reference

Technical reference for the Neo Geo Pocket Color hardware architecture. This
document covers the system-on-chip design, CPU integration, memory map, timer
system, interrupt controller, micro DMA, input, cartridge format, BIOS, power
management, RTC, and flash storage.

For component-specific details, refer to:

- [ngpc_system_overview.md](ngpc_system_overview.md) - High-level chip
  overview and interaction diagram
- [k2ge_reference.md](k2ge_reference.md) - K2GE video display controller
  (sprites, scroll planes, palettes, display timing)
- [ngpc_sound_integration.md](ngpc_sound_integration.md) - Sound system
  (T6W28 PSG, DACs, Z80 sound CPU, audio mixing)
- [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) - SoC peripheral
  register-level detail (16-bit timers, DMA registers, clock gear, RTC)
- [ngpc_bios.md](ngpc_bios.md) - BIOS ROM, system calls, boot sequence, HLE

This document does not cover the TLCS-900H CPU instruction set internals,
Z80 CPU internals, or T6W28/SN76489 chip internals. Those are documented
in their respective emulation module source trees.

---

## Table of Contents

- [System Overview](#system-overview)
- [Clock System](#clock-system)
- [TLCS-900H CPU Integration](#tlcs-900h-cpu-integration)
- [Memory Map](#memory-map)
- [CPU I/O Registers (SFR)](#cpu-io-registers-sfr)
- [System RAM Registers](#system-ram-registers)
- [Timer System](#timer-system)
- [Interrupt System](#interrupt-system)
- [Micro DMA (HDMA)](#micro-dma-hdma)
- [Input System](#input-system)
- [Serial Communication](#serial-communication)
- [Power Management](#power-management)
- [Real-Time Clock](#real-time-clock)
- [Cartridge Interface](#cartridge-interface)
- [Flash Memory and Save Data](#flash-memory-and-save-data)
- [BIOS](#bios)
- [Programming Constraints](#programming-constraints)
- [Sources](#sources)

---

## System Overview

The Neo Geo Pocket Color (NGPC) is a handheld console built around a Toshiba
TLCS-900/H CPU with integrated peripherals. A secondary Z80 CPU handles
sound processing. The system uses a single clock domain with no NTSC/PAL
distinction.

### Major Components

| Component | Description |
|-----------|-------------|
| Main CPU | Toshiba TLCS-900/H (TMP95CS64F variant) |
| Sound CPU | Z80-compatible core at 3.072 MHz |
| Video | K2GE (color) / K1GE (monochrome compatibility) graphics engine |
| Sound | T6W28 PSG (SN76489 variant) + 2x 6-bit DACs |
| Crystal | 6.144 MHz master oscillator |

### CPU SoC Integration (TMP95CS64F)

The main CPU is a system-on-chip that integrates the following:

| Feature | Description |
|---------|-------------|
| CPU core | TLCS-900/H, 16-bit internal bus, 24-bit address space |
| Internal RAM | 12 KB (0-wait state, 8/16-bit access) |
| Internal ROM | 64 KB BIOS |
| 8-bit timers | 4 (T0-T3), combinable into 2x 16-bit |
| 16-bit timers | 4 (T4-T7) with comparators and capture |
| Micro DMA | 4 channels (HDMA) |
| Serial | 2 UART channels |
| A/D converter | 10-bit (battery voltage monitoring) |
| Interrupt controller | 24 channels |
| Watchdog timer | System reset if not serviced |
| Clock gear | Configurable CPU clock divider |
| Chip select / wait | Configurable per memory region |

### Discrete Components

| Component | Description |
|-----------|-------------|
| K1GE / K2GE | SNK custom graphics engine (separate chip) |
| T6W28 + Z80 | Sound subsystem (separate from main SoC) |
| Flash ROM | Toshiba flash chips on cartridge (game code + save data) |
| CR2032 battery | Backup for RTC, system settings, resume RAM |

---

## Clock System

All system clocks derive from a single 6.144 MHz crystal.

### Clock Derivation

| Component | Clock | Derivation |
|-----------|-------|------------|
| Master oscillator | 6.144 MHz | Crystal |
| TLCS-900H (max) | 6.144 MHz | Master / 1 (configurable via clock gear) |
| K2GE | 6.144 MHz | Master (shared with CPU) |
| Z80 sound CPU | 3.072 MHz | Master / 2 (independent of clock gear) |
| Prescaler input | 1.536 MHz | Master / 4 (feeds prescaler divider chain) |

### Clock Gear

The CPU clock speed is adjustable through the BIOS system call
VECT_CLOCKGEARSET. The clock gear divides the master oscillator:

| Gear | Divisor | CPU Clock | Notes |
|------|---------|-----------|-------|
| 0 | /1 | 6.144 MHz | Full speed (default) |
| 1 | /2 | 3.072 MHz | |
| 2 | /4 | 1.536 MHz | |
| 3 | /8 | 768 KHz | |
| 4 | /16 | 384 KHz | Minimum speed |

The clock gear does NOT affect the prescaler. The prescaler is fed from
the oscillator's 1/4 divider, independent of the CPU clock path. Timer
frequencies are constant regardless of gear setting. The Z80 sound CPU
clock (3.072 MHz) is also independent of the clock gear.

### Timer Clock Sources

The prescaler produces four derived clock sources for the timer system.
The datasheet "fc/N" notation uses fc = oscillator frequency, not CPU
state rate. The CPU state divisors account for the relationship between
the CPU state rate (fosc/2) and the prescaler divider chain:

| Source | Datasheet | CPU State Divisor | Frequency at 6.144 MHz |
|--------|-----------|-------------------|------------------------|
| T1     | fc/8      | 128               | 48 kHz                 |
| T4     | fc/32     | 512               | 12 kHz                 |
| T16    | fc/128    | 2048              | 3 kHz                  |
| T256   | fc/2048   | 32768             | 187.5 Hz               |

These frequencies are fixed and do not scale with the clock gear setting.

---

## TLCS-900H CPU Integration

### CPU Characteristics

| Parameter | Value |
|-----------|-------|
| Architecture | TLCS-900/H (Toshiba) |
| Internal data bus | 16-bit |
| External data bus | 8-bit (to cartridge ROM) |
| Address bus | 24-bit (16 MB address space) |
| Operating mode | Maximum mode (32-bit registers) |
| Instruction queue | 4 bytes (prefetch) |

The TLCS-900/H is architecturally descended from the TLCS-90 (a Z80
derivative) with enhancements: 32-bit registers, 24-bit linear addressing,
and a richer instruction set.

### Register File

The CPU operates in maximum mode with four banks of 32-bit general purpose
registers. Each 32-bit register can be accessed at three widths:

| 32-bit | 16-bit (low half) | 8-bit (low bytes) |
|--------|-------------------|-------------------|
| XWA | WA | W, A |
| XBC | BC | B, C |
| XDE | DE | D, E |
| XHL | HL | H, L |

Four register banks (0-3) are available. The active bank is selected by
the RFP field in the Status Register. Register bank 3 is reserved for the
BIOS and must not be used by application software.

Dedicated (non-banked) registers:

| Register | Size | Purpose |
|----------|------|---------|
| XIX | 32-bit | Index register X |
| XIY | 32-bit | Index register Y |
| XIZ | 32-bit | Index register Z |
| XSP | 32-bit | Stack pointer |
| PC | 32-bit | Program counter (24-bit effective) |
| SR | 16-bit | Status register |
| F' | 8-bit | Alternate flags register |

The 16-bit low halves of index registers are IX, IY, IZ, SP.

### Status Register (SR)

The 16-bit SR is divided into two bytes:

**Upper byte (system control):**

| Bits | Name | Function |
|------|------|----------|
| 15 | SYSM | System mode (1 = system, 0 = user) |
| 14-12 | IFF2-IFF0 | Interrupt mask level (0-7) |
| 11 | MAX | Maximum mode flag (always 1 on NGPC) |
| 10-8 | RFP2-RFP0 | Register File Pointer (selects bank 0-3) |

**Lower byte (F register - arithmetic flags):**

| Bit | Name | Function |
|-----|------|----------|
| 7 | S | Sign flag |
| 6 | Z | Zero flag |
| 4 | H | Half-carry flag (BCD) |
| 2 | V | Overflow / parity flag |
| 1 | N | Subtract flag (for DAA) |
| 0 | C | Carry flag |

Bits 5 and 3 are undefined.

### Condition Codes

The TLCS-900H supports 16 condition codes for conditional instructions:

| Code | Mnemonic | Condition |
|------|----------|-----------|
| 0x0 | F | Never (false) |
| 0x1 | LT | S xor V (signed less than) |
| 0x2 | LE | (S xor V) or Z (signed less or equal) |
| 0x3 | ULE | C or Z (unsigned less or equal) |
| 0x4 | OV | V (overflow) |
| 0x5 | MI | S (minus / negative) |
| 0x6 | EQ / Z | Z (equal / zero) |
| 0x7 | ULT / C | C (unsigned less than / carry) |
| 0x8 | T | Always (true) |
| 0x9 | GE | not (S xor V) (signed greater or equal) |
| 0xA | GT | not ((S xor V) or Z) (signed greater than) |
| 0xB | UGT | not (C or Z) (unsigned greater than) |
| 0xC | NOV | not V (no overflow) |
| 0xD | PL | not S (plus / positive) |
| 0xE | NE / NZ | not Z (not equal / not zero) |
| 0xF | UGE / NC | not C (unsigned greater or equal / no carry) |

### Instruction Encoding

Instructions use variable-length encoding. Opcodes 0x00-0xDF are
single-byte basic instructions. Opcodes 0xE0-0xFE are prefix bytes that
begin extended (two-byte) instructions. The prefix encodes one operand
(typically a memory addressing mode) and may be followed by displacement
bytes. After the prefix comes the operation opcode byte specifying the
operation and second operand.

---

## Memory Map

The TLCS-900H has a 24-bit address bus providing a 16 MB address space.

| Address Range | Size | Description |
|---------------|------|-------------|
| $000000-$0000FF | 256 bytes | CPU internal I/O registers (SFR area) |
| $004000-$006BFF | 11 KB | Work RAM (battery-backed during resume) |
| $006C00-$006FFF | 1 KB | System-reserved RAM (BIOS variables, interrupt vectors) |
| $007000-$007FFF | 4 KB | Z80 shared RAM (sound CPU program + data) |
| $008000-$00BFFF | 16 KB | K2GE video registers, sprite/scroll/character RAM |
| $200000-$3EFFFF | up to 1984 KB | Cartridge Flash ROM chip 0 (CS0) |
| $3F0000-$3FFFFF | 64 KB | Cartridge system ROM (factory, reserved) |
| $800000-$9FFFFF | 2 MB | Cartridge Flash ROM chip 1 (CS1, for large games) |
| $FF0000-$FFFFFF | 64 KB | Internal BIOS ROM |

### Work RAM Detail

| Address Range | Size | Description |
|---------------|------|-------------|
| $004000-$005FFF | 8 KB | Battery-backed work RAM (preserved during resume) |
| $006000-$006BFF | 3 KB | Additional work RAM |
| $006C00-$006FFF | 1 KB | System RAM (BIOS state, interrupt vector table) |

### K2GE Video Memory

| Address Range | Size | Description |
|---------------|------|-------------|
| $008000-$0087FF | 2 KB | Control registers, palettes, LED control |
| $008800-$0088FF | 256 bytes | Sprite attribute table (64 sprites x 4 bytes) |
| $008C00-$008C3F | 64 bytes | Sprite color palette assignments (K2GE only) |
| $009000-$0097FF | 2 KB | Scroll Plane 1 tile map |
| $009800-$009FFF | 2 KB | Scroll Plane 2 tile map |
| $00A000-$00BFFF | 8 KB | Character RAM (512 tiles x 16 bytes) |

See k2ge_reference.md for register and VRAM details.

---

## CPU I/O Registers (SFR)

The Special Function Registers occupy addresses $000000-$0000FF (256 bytes).
The lower range ($00-$7F) contains timers, serial ports, DMA, interrupts,
I/O ports, A/D converter, watchdog, and chip select/wait state
configuration. The upper range ($80-$FF) contains sound control registers
($A0-$BC) and RTC registers ($90-$97). The $80-$8F range has no known
functional registers on the NGPC despite being present in the TMP95C061
register map.

### I/O Ports

| Address | Register | Description |
|---------|----------|-------------|
| $01 | P1 | I/O Port 1 |
| $04 | P1CR | Port 1 Control Register |
| $06 | P2 | I/O Port 2 |
| $09 | P5 | I/O Port 5 |
| $0D | P6 | I/O Port 6 |
| $0E | P7 | I/O Port 7 |
| $0F | P7CR | Port 7 Control Register |
| $17 | P7FC | Port 7 Function Control |
| $18 | P8 | I/O Port 8 |
| $19 | P9 | I/O Port 9 |
| $1A | P8CR | Port 8 Control Register |
| $1B | P8FC | Port 8 Function Control |
| $1E | PA | I/O Port A |
| $1F | PB | I/O Port B |
| $2C | PACR | Port A Control Register |
| $2D | PAFC | Port A Function Control |

### Timer Registers

| Address | Register | Description |
|---------|----------|-------------|
| $20 | TRUN | Timer Run (start/stop individual timers) |
| $22 | TREG0 | Timer 0 compare/reload value |
| $23 | TREG1 | Timer 1 compare/reload value |
| $24 | T01MOD | Timer 0/1 Mode (clock source, 8/16-bit mode) |
| $25 | TFFCR | Timer Flip-Flop Control |
| $26 | TREG2 | Timer 2 compare/reload value |
| $27 | TREG3 | Timer 3 compare/reload value |
| $28 | T23MOD | Timer 2/3 Mode |
| $29 | TRDC | Timer Reload / Double-buffer Control |

### 16-Bit Timer Registers

| Address | Register | Description |
|---------|----------|-------------|
| $30-$37 | TREG4/5, CAP1/2 | T4/T5 compare/reload and capture registers |
| $38 | T4MOD | Timer 4 mode |
| $39 | T4FFCR | Timer 4 flip-flop control |
| $3A | T45CR | Timer 4/5 control |
| $40-$47 | TREG6/7, CAP3/4 | T6/T7 compare/reload and capture registers |
| $48 | T5MOD | Timer 5 mode (controls T6/T7 channel) |
| $49 | T5FFCR | Timer 5 flip-flop control |

See [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for register-level
detail.

### DMA Registers

DMA source, destination, count, and mode registers are in the CPU control
register (CR) space, accessed via LDC instructions. They are NOT
memory-mapped SFR registers. The DMA start vector registers are
memory-mapped:

| Address | Register | Description |
|---------|----------|-------------|
| $7C | DMA0V | DMA Channel 0 start vector |
| $7D | DMA1V | DMA Channel 1 start vector |
| $7E | DMA2V | DMA Channel 2 start vector |
| $7F | DMA3V | DMA Channel 3 start vector |

See [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for CR space
register offsets and DMAM mode register bit definitions.

### Serial Registers

| Address | Register | Init | Description |
|---------|----------|------|-------------|
| $50 | SC0BUF | $00 | Serial Channel 0 Buffer (used as link cable comm buffer) |
| $51 | SC0CR | $20 | Serial Channel 0 Control |
| $52 | SC0MOD | $69 | Serial Channel 0 Mode |
| $53 | BR0CR | -- | Baud Rate 0 Control |
| $54 | SC1BUF | -- | Serial Channel 1 Buffer |
| $55 | SC1CR | -- | Serial Channel 1 Control |
| $56 | SC1MOD | -- | Serial Channel 1 Mode |
| $57 | BR1CR | -- | Baud Rate 1 Control |

Serial Channel 0 (SC0BUF at $50) is used as the link cable communication
data buffer. SC0CR and SC0MOD are initialized to fixed values but are not
actively configured by emulators beyond initialization.

### Miscellaneous Registers

| Address | Register | Description |
|---------|----------|-------------|
| $5C | ODE | Open Drain Enable |
| $60-$61 | ADREG0L/H | A/D Result Register 0 (low/high) |
| $62-$63 | ADREG1L/H | A/D Result Register 1 (low/high) |
| $64-$65 | ADREG2L/H | A/D Result Register 2 (low/high) |
| $66-$67 | ADREG3L/H | A/D Result Register 3 (low/high) |
| $68 | B0CS | Block 0 Chip Select / Wait Control |
| $69 | B1CS | Block 1 Chip Select / Wait Control |
| $6A | B2CS | Block 2 Chip Select / Wait Control |
| $6B | B3CS | Block 3 Chip Select / Wait Control |
| $6C | BEXCS | External Block Chip Select / Wait Control |
| $6D | ADMOD | A/D Mode Register |
| $6E | WDMOD | Watchdog Timer Mode |
| $6F | WDCR | Watchdog Timer Control |

### Interrupt Enable Registers

| Address | Register | Description |
|---------|----------|-------------|
| $70 | INTE0AD | Interrupt enable (INT0 + A/D) |
| $71 | INTE45 | Interrupt enable (INT4, INT5) |
| $72 | INTE67 | Interrupt enable (INT6, INT7) |
| $73 | INTET01 | Interrupt enable (Timer 0, Timer 1) |
| $74 | INTET23 | Interrupt enable (Timer 2, Timer 3) |
| $75 | INTET45 | Interrupt enable (Timer 4, Timer 5) |
| $76 | INTET67 | Interrupt enable (Timer 6, Timer 7) |
| $77 | INTES0 | Interrupt enable (Serial 0) |
| $78 | INTES1 | Interrupt enable (Serial 1) |
| $79 | INTETC01 | Interrupt enable (Micro DMA 0, 1) |
| $7A | INTETC23 | Interrupt enable (Micro DMA 2, 3) |
| $7B | IIMC | Interrupt Input Mode Control |

### Chip Select / Wait Control

The chip select registers (B0CS-B3CS at $68-$6B, BEXCS at $6C) configure
wait states for each memory region. Cartridge ROM access (CS0, CS1)
requires wait states due to the 8-bit external data bus.

### Sound-Related Registers

| Address | Register | Description |
|---------|----------|-------------|
| $00A0 | -- | T6W28 right channel write port (gated: requires $B8=$55 and $B9=$AA) |
| $00A1 | -- | T6W28 left channel write port (gated: requires $B8=$55 and $B9=$AA) |
| $00A2 | DACL | DAC Left output (unsigned 8-bit, 0x80 = center) |
| $00A3 | DACR | DAC Right output (unsigned 8-bit, 0x80 = center) |
| $00B8 | -- | Sound chip activation ($55 = on, $AA = off) |
| $00B9 | -- | Z80 activation ($55 = on, $AA = off) |
| $00BA | -- | Z80 NMI trigger (any write fires Z80 NMI) |
| $00BC | -- | Z80 <-> TLCS-900H communication register |

See ngpc_sound_integration.md for details on the sound system.

### Power and System Control Registers

| Address | Register | R/W | Description |
|---------|----------|-----|-------------|
| $00B0 | -- | R | Controller input (direct hardware read) |
| $00B1 | -- | R | Power status: bit 0 = power button state, bit 1 = sub-battery OK |
| $00B3 | -- | R/W | NMI enable control: bit 2 enables power button NMI |
| $00B6 | -- | W | Power state: $50 = powering down, $05 = powering up |

The BIOS reads $00B0 for raw controller input during its VBlank handler
and writes the processed result to $6F82. The BIOS reads $00B1 to check
the power button and sub-battery status. Register $00B3 bit 2 must be
set for the power button to generate an NMI. The BIOS writes $00B6
during power state transitions (VECT_SHUTDOWN writes $50).

---

## System RAM Registers

The BIOS reserves the region $006C00-$006FFF for system state and the
user interrupt vector table.

### System State

| Address | Size | Description |
|---------|------|-------------|
| $6C00 | 4 bytes | Cartridge start PC (copied from header offset $1C) |
| $6C04 | 2 bytes | Cartridge software ID (copied from header offset $20) |
| $6C06 | 1 byte | Cartridge sub-code (copied from header offset $22) |
| $6C08 | 12 bytes | Cartridge title (copied from header offset $24) |
| $6C14 | 1 byte | BIOS setup completion flag ($DD = setup done) |
| $6C15 | 1 byte | BIOS setup flag 2 ($00 = setup done) |
| $6C55 | 1 byte | Commercial game flag ($01 = game loaded, $00 = BIOS menu) |
| $6C58 | 1 byte | CS0 flash chip present ($01 = yes, $00 = no) |
| $6C59 | 1 byte | CS1 flash chip present ($01 = yes, $00 = no; set for 4 MB ROMs) |
| $6F80 | 2 bytes | Battery voltage (10-bit A/D result, max $3FF) |
| $6F82 | 1 byte | Controller input status |
| $6F84 | 1 byte | User Boot status flags |
| $6F85 | 1 byte | User Shutdown request flag |
| $6F86 | 1 byte | User Answer |
| $6F87 | 1 byte | Language setting ($00 = Japanese, $01 = English) |
| $6F89 | 1 byte | A/D interrupt status flag (bit 7 monitored by BIOS) |
| $6F91 | 1 byte | System type ($00 = monochrome NGP, $10 = color NGPC) |

### User Boot Status ($6F84)

| Bit | Name | Description |
|-----|------|-------------|
| 7 | Alarm | RTC alarm triggered startup |
| 6 | Power ON | Normal power-on startup |
| 5 | Resume | Resume from sleep (battery-backed RAM preserved) |

### User Interrupt Vector Table ($6FB8-$6FFC)

The BIOS dispatches hardware interrupts to user-defined handlers via this
RAM table. Each entry is a 32-bit (4-byte) handler address.

| Address | Interrupt Source | Micro DMA Vector |
|---------|-----------------|------------------|
| $6FB8 | SWI 3 | -- |
| $6FBC | SWI 4 | -- |
| $6FC0 | SWI 5 | -- |
| $6FC4 | SWI 6 | -- |
| $6FC8 | RTC Alarm | $0A |
| $6FCC | VBlank (INT4) | $0B |
| $6FD0 | Z80 Interrupt (INT5) | $0C |
| $6FD4 | Timer 0 (INTT0) | $10 |
| $6FD8 | Timer 1 (INTT1) | $11 |
| $6FDC | Timer 2 (INTT2) | $12 |
| $6FE0 | Timer 3 (INTT3) | $13 |
| $6FE4 | Serial TX (channel 1) | $18 |
| $6FE8 | Serial RX (channel 1) | $19 |
| $6FEC | (reserved) | -- |
| $6FF0 | Micro DMA 0 End (INTTC0) | -- |
| $6FF4 | Micro DMA 1 End (INTTC1) | -- |
| $6FF8 | Micro DMA 2 End (INTTC2) | -- |
| $6FFC | Micro DMA 3 End (INTTC3) | -- |

User programs write their handler addresses into this table before
enabling interrupts.

---

## Timer System

### Overview

The TMP95C061 contains 4x 8-bit timers (T0-T3) and 4x 16-bit timers
(T4-T7).

| Timer | Width | NGPC Usage |
|-------|-------|------------|
| T0 | 8-bit | HBlank counting (external clock input from K2GE H_INT) |
| T1 | 8-bit | General purpose, can pair with T0 for 16-bit |
| T2 | 8-bit | General purpose |
| T3 | 8-bit | Z80 IRQ source (TO3 flip-flop rising edge triggers Z80 IRQ), can pair with T2 for 16-bit |
| T4-T7 | 16-bit | General purpose, with comparators and capture (see [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for register detail) |

### 8-Bit Timer Operation

Each 8-bit timer counts up from 0. When the counter matches the TREG
compare value, the timer:

1. Generates an interrupt (if enabled)
2. Reloads the counter to 0
3. Optionally toggles the timer flip-flop output

The timer clock source is selected via the mode register (T01MOD or
T23MOD). Available sources include the prescaler outputs (T1, T4, T16,
T256) or an external clock input.

### Timer 0 and HBlank

Timer 0 supports an external clock input (TI0). On the NGPC, TI0 is
connected to the K2GE H_INT signal. This allows Timer 0 to count HBlank
pulses and fire an interrupt after a programmable number of scanlines.

Configuration for per-N-scanline interrupts:
1. Set Timer 0 to count mode with TI0 as the clock source
2. Set TREG0 to the desired scanline interval
3. Enable Timer 0 interrupt
4. Timer 0 fires its interrupt after that many HBlank signals

The Timer 0 interrupt vector is at $6FD4 in the user interrupt table.

### TRUN Register ($20)

Controls which timers are running. Writing a 1 to a timer's bit starts
it; writing 0 stops it.

### 16-Bit Timer Pairing

Timers 0+1 and 2+3 can each be paired into a single 16-bit timer by
setting the appropriate mode in T01MOD or T23MOD. In 16-bit mode, the
lower timer's overflow clocks the upper timer.

---

## Interrupt System

### Interrupt Controller

The TMP95C061 has a 24-channel interrupt controller. Each channel has:

- An interrupt request flip-flop
- A 3-bit priority setting (levels 0-6; level 7 is NMI)
- A micro DMA start vector (for triggering HDMA on interrupt)

Interrupts are masked based on the IFF field in the SR register. Only
interrupts with priority greater than or equal to IFF are accepted.
NMI (level 7) cannot be masked.

When an interrupt is serviced, the CPU pushes PC and SR onto the stack,
then sets IFF to the serviced interrupt's priority level + 1 (capped
at 7). This masks the current and lower priority interrupts while the
handler runs. RETI restores the original SR (and thus the original
IFF) from the stack.

### Hardware Interrupt Vector Table (BIOS ROM at $FFFF00)

| Address | Vector | Priority | Description |
|---------|--------|----------|-------------|
| $FFFF00 | RESET/SWI0 | -- | Power-on reset vector |
| $FFFF04 | SWI1 | -- | Software interrupt 1 (BIOS system call entry) |
| $FFFF08 | SWI2 | -- | Software interrupt 2 |
| $FFFF0C | SWI3 | -- | Software interrupt 3 |
| $FFFF10 | SWI4 | -- | Software interrupt 4 |
| $FFFF14 | SWI5 | -- | Software interrupt 5 |
| $FFFF18 | SWI6 | -- | Software interrupt 6 |
| $FFFF1C | SWI7 | -- | Software interrupt 7 |
| $FFFF20 | NMI | 7 | Non-maskable interrupt (power button) |
| $FFFF24 | INTWD | 6 | Watchdog timer |
| $FFFF28 | INT0 | configurable | External interrupt 0 |
| $FFFF2C | INT4 | 4 | External interrupt 4 (VBlank from K2GE) |
| $FFFF30 | INT5 | configurable | External interrupt 5 (Z80 to TLCS-900H) |
| $FFFF34 | INT6 | configurable | External interrupt 6 |
| $FFFF38 | INT7 | configurable | External interrupt 7 |
| $FFFF40 | INTT0 | configurable | Timer 0 (HBlank driven) |
| $FFFF44 | INTT1 | configurable | Timer 1 |
| $FFFF48 | INTT2 | configurable | Timer 2 |
| $FFFF4C | INTT3 | configurable | Timer 3 |
| $FFFF50 | INTTR4 | configurable | Timer 4 |
| $FFFF54 | INTTR5 | configurable | Timer 5 |
| $FFFF58 | INTTR6 | configurable | Timer 6 |
| $FFFF5C | INTTR7 | configurable | Timer 7 |
| $FFFF60 | INTRX0 | configurable | Serial channel 0 receive |
| $FFFF64 | INTTX0 | configurable | Serial channel 0 transmit |
| $FFFF68 | INTRX1 | configurable | Serial channel 1 receive |
| $FFFF6C | INTTX1 | configurable | Serial channel 1 transmit |
| $FFFF70 | INTAD | configurable | A/D conversion complete |
| $FFFF74 | INTTC0 | 6 | Micro DMA channel 0 end |
| $FFFF78 | INTTC1 | 6 | Micro DMA channel 1 end |
| $FFFF7C | INTTC2 | 6 | Micro DMA channel 2 end |
| $FFFF80 | INTTC3 | 6 | Micro DMA channel 3 end |

### Interrupt Dispatch

The BIOS intercepts hardware interrupts at the ROM vector table, performs
system housekeeping (watchdog service, power management), then dispatches
to user-defined handlers via the RAM vector table at $6FB8-$6FFC.

### Priority Configuration

Registers INTE0AD through INTETC23 ($70-$7A) each contain two 3-bit
priority fields packed into a single byte. Setting a priority to 0
disables that interrupt source.

The VBlank interrupt (INT4) is system-critical. The NGPC specification
states it is "forbidden to prohibit" this interrupt because system
operations depend on it.

---

## Micro DMA (HDMA)

### Overview

The TMP95C061 includes 4 independent micro DMA (HDMA) channels. These
perform small data transfers triggered by interrupt sources without full
CPU context-switch overhead.

### Operation

1. Each micro DMA channel is associated with a specific interrupt source
   via a start vector number
2. When the triggering interrupt fires, the micro DMA channel executes
   a transfer instead of (or before) the normal interrupt handler
3. Transfers execute at priority level 6
4. Transfer completion generates an INTTC0-3 interrupt

### Registers per Channel

| Register | Space | Description |
|----------|-------|-------------|
| DMAS | CR | Source address (32-bit) |
| DMAD | CR | Destination address (32-bit) |
| DMAC | CR | Transfer count (16-bit) |
| DMAM | CR | Mode (transfer size, direction, address increment/decrement) |
| DMAxV | SFR ($7C-$7F) | Start vector (which interrupt triggers this channel) |

DMAS, DMAD, DMAC, and DMAM are in the CPU control register (CR) space,
accessed via LDC instructions. The start vector registers are standard
memory-mapped SFR registers. Supported transfer sizes: byte, word,
longword.

See [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for CR space
offsets, DMAM bit field definitions, and start vector mappings.

### NGPC Usage

| Application | Mechanism |
|-------------|-----------|
| Raster scroll effects | Timer 0 (HBlank) triggers micro DMA to update scroll registers per scanline |
| DAC sample playback | Timer-driven micro DMA writes sample data to DAC registers at a fixed rate |
| VBlank transfers | VBlank interrupt (start vector $0B) triggers frame-synchronized DMA |
| Z80 communication | Z80 interrupt (start vector $0C) triggers data transfer to/from shared RAM |

---

## Input System

### Controller Layout

The NGPC has the following inputs:

| Input | Type |
|-------|------|
| Directional pad | 8-way microswitched thumb stick (Up, Down, Left, Right + diagonals) |
| Button A | Action button |
| Button B | Action button |
| Option | Menu/option button |
| Power | Power switch (triggers NMI, not a normal input) |

### Input Register ($6F82)

Controller state is read from memory address $6F82 (1 byte). The BIOS
updates this register during system interrupts.

| Bit | Input | Active State |
|-----|-------|-------------|
| 0 | Up | Set when pressed |
| 1 | Down | Set when pressed |
| 2 | Left | Set when pressed |
| 3 | Right | Set when pressed |
| 4 | Button A | Set when pressed |
| 5 | Button B | Set when pressed |
| 6 | Option | Set when pressed |
| 7 | -- | Unused |

Diagonal inputs are represented by multiple direction bits being set
simultaneously (e.g., Up + Right for upper-right diagonal).

User programs read this memory location directly. No polling or
register manipulation is required; the BIOS handles input scanning.

---

## Serial Communication

### Hardware

Serial Channel 1 is used for the link cable (5-pin serial port).

| Parameter | Value |
|-----------|-------|
| Baud rate | 19,200 bps |
| Channel | Serial Channel 1 |
| Connector | 5-pin serial port |

### BIOS System Calls

| Address | System Call | Description |
|---------|-------------|-------------|
| $FFFE40 | VECT_COMINIT | Initialize serial communication |
| $FFFE44 | VECT_COMEND | Flush/end link communication |
| $FFFE4C | VECT_COMSENDDATA | Transmit byte (parameter in RB3) |
| $FFFE5C | VECT_COMGETSENDDATACOUNT | Query send queue depth (returns in RWA3) |
| $FFFE60 | VECT_COMGETRECVDATACOUNT | Query receive queue depth (returns in RWA3) |

Note: the full communication vector table ($10-$1A) has additional calls
not listed here. See [ngpc_bios.md](ngpc_bios.md) for the complete list
with parameters. The names above are from ngpctech.txt; names in the BIOS
reference are from HLE implementations and may differ.

Serial interrupt vectors: $6FE4 (TX), $6FE8 (RX).

There is no method to synchronize the vertical blanking periods between
two linked units.

---

## Power Management

### Power Button

The power button triggers an NMI interrupt (vector at $FFFF20). The BIOS
handles initial NMI processing. If the power switch is pressed during
program execution, the BIOS sets the User Shutdown flag at $6F85.

User programs must monitor $6F85 and call VECT_SHUTDOWN when the flag is
set to allow an orderly power-off sequence.

### HALT Restriction

The HALT instruction is prohibited for user programs. Voltage management
is delegated entirely to the system BIOS. Using HALT directly can
interfere with power management.

### Interrupt Mask Restriction

The IFF field in the SR must be kept at <= 2 during normal operation.
Setting it higher interferes with BIOS power management and system
operations.

### Watchdog Timer

| Parameter | Value |
|-----------|-------|
| Mode register | $6E (WDMOD) |
| Control register | $6F (WDCR) |
| Service value | Write $4E to WDCR |
| Timeout | Approximately 100 ms |

The watchdog must be serviced at least once every ~100 ms by writing $4E
to the WDCR register. Failure to service it triggers a system reset. The
BIOS handles watchdog servicing during normal operation.

### Battery Monitoring

The A/D converter monitors battery voltage on analog input AN0. The
10-bit result is available at $6F80 (max value $3FF). The INTAD
interrupt ($FFFF70) signals conversion completion.

For emulation, AN0 should return $3FF (full battery). If the A/D
converter returns a low value, the BIOS may enter a low-battery
shutdown path. The A/D mode register ($6D) controls conversion
start and channel selection. The BIOS configures and triggers A/D
conversions during its VBlank interrupt handler.

---

## Real-Time Clock

### Features

The NGPC includes an integrated RTC maintained by the CR2032 backup
battery. The RTC registers occupy SFR addresses $90-$97. See
[ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for register-level
detail.

| Feature | Description |
|---------|-------------|
| Time tracking | Date and time |
| Alarm | Can wake the system from power-off state |
| Battery | CR2032 lithium cell |

### BIOS Interface

See [ngpc_bios.md](ngpc_bios.md) for parameter details.

| Address | System Call | Description |
|---------|-------------|-------------|
| $FFFE08 | VECT_RTCGET | Read current date/time from the RTC |
| $FFFE14 | VECT_ALARMSET | Set an alarm during game operation |
| $FFFE18 | VECT_ALARMDOWNSET | Set a power-on alarm (system wakes at specified time) |

### Alarm Interrupt

When the RTC alarm fires, it generates an interrupt dispatched to the
user vector at $6FC8 with micro DMA start vector $0A. An alarm can wake
the system from a powered-off state (User Boot bit 7 = Alarm startup).

### Battery Backup Scope

The CR2032 battery maintains:

- RTC time and alarm settings
- System settings (language, date/time configuration)
- Resume RAM ($4000-$5FFF) for sleep/wake functionality

The backup battery does NOT maintain game save data. Saves are stored in
cartridge flash memory.

For emulation purposes, the system RAM at $4000-$6FFF (12 KB) and the
first $20 bytes of the custom I/O registers ($80-$9F) should be persisted
as NVRAM. When this saved state is available on startup, the BIOS uses a
warm boot path at $FF1800 that skips full initialization and restores from
the persisted state. On a cold boot (no saved state), $6C00-$6FFF is
re-initialized from the cartridge ROM header data. See
[ngpc_bios.md](ngpc_bios.md) for details on the warm boot mechanism.

---

## Cartridge Interface

### Physical Connector

The cartridge slot is a 40-pin connector:

| Signal | Description |
|--------|-------------|
| A0-A20 | 21 address lines (up to 2 MB per chip select) |
| D0-D7 | 8 data lines (8-bit bus) |
| CS0 | Chip select 0 (primary ROM) |
| CS1 | Chip select 1 (secondary ROM for large games) |
| RD/OE | Read enable |
| WE | Write enable |
| RESET | Reset line |
| VCC | 3.3V logic |
| GND | Ground |

### Memory Mapping

| Chip Select | Address Range | Maximum Size |
|------------|---------------|-------------|
| CS0 | $200000-$3EFFFF | ~2 MB |
| CS1 | $800000-$9FFFFF | 2 MB |

The region $3F0000-$3FFFFF (64 KB) at the top of CS0 space is reserved
for the cartridge system ROM and is not available for game code or data.

Maximum total cartridge ROM: 4 MB (32 Mbit) across both chip selects.

### Wait States

Cartridge ROM access requires wait states configured via the B0CS ($7C)
and B1CS ($7D) registers. The external data bus is 8-bit, so 16-bit
accesses require two bus cycles.

### ROM Header and File Format

See [ngpc_cartridge_format.md](ngpc_cartridge_format.md) for the complete ROM
header layout, file format details, flash command protocol, and save data
format.

The system code at header offset $23 determines which graphics mode the BIOS
selects:
- $00: BIOS sets K1GE compatibility mode (monochrome)
- $10: BIOS sets K2GE color mode

---

## Flash Memory and Save Data

See [ngpc_cartridge_format.md](ngpc_cartridge_format.md) for the flash command
protocol (AMD-style state machine), save data file format, and ROM loading
procedure.

### Architecture

The NGPC uses the cartridge flash ROM for persistent save data storage.
There is no separate SRAM or EEPROM. The same flash chips that hold game
code also hold save data.

### Flash Chip Variants

Cartridges use flash chips from Toshiba, Sharp, or Samsung. The BIOS
supports three manufacturer IDs and three device IDs:

| Manufacturer | ID |
|--------------|------|
| Toshiba | $98 |
| Samsung | $EC |
| Sharp | $B0 |

| Size | Device ID | Main Blocks | Boot Blocks (top of address space) |
|------|-----------|-------------|-----------------------------------|
| 4 Mbit (512 KB) | $AB | 7 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |
| 8 Mbit (1 MB) | $2C | 15 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |
| 16 Mbit (2 MB) | $2F | 31 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |

32 Mbit (4 MB) cartridges use two 16 Mbit chips, one per chip select
(CS0 at $200000, CS1 at $800000).

The flash chips use AMD-style command sequences. The chip identification
command (0x5555=AA, 0x2AAA=55, 0x5555=90) places the manufacturer ID at
offset 0 and device ID at offset 1 within the chip address space.

The last block of each chip is system-reserved. The BIOS tests this block
on every boot to verify flash functionality.

### BIOS Flash System Calls

See [ngpc_bios.md](ngpc_bios.md) for parameter details.

| Address | System Call | Description |
|---------|-------------|-------------|
| $FFFE1C | VECT_FLASHWRITE | Write data to flash |
| $FFFE20 | VECT_FLASHALLERS | Erase all flash blocks |
| $FFFE24 | VECT_FLASHERS | Erase specified flash block(s) |
| $FFFE28 | VECT_FLASHPROTECT | Protect specified flash block(s) |

### Flash Constraints

- Flash can only clear bits to 0. Block erase sets all bits to 1.
  Erase is required before writing to a previously written region.
- Flash has finite erase/write endurance (typically ~100,000 cycles
  per block)
- VECT_FLASHERS supports blocks 0-32 only. Higher block numbers
  produce undefined behavior.
- VECT_FLASHPROTECT is irreversible. Once a block is protected, it
  cannot be unprotected.
- The last 16 KB block of each flash chip is system-reserved and must
  not contain user code, data, or save areas.

### Resume / Battery-Backed RAM

Separately from flash saves, work RAM at $4000-$5FFF (8 KB) is
battery-backed by the CR2032. This is preserved during a resume startup
(sleep mode) but is cleared when a different cartridge is inserted.

---

## BIOS

The 64 KB BIOS ROM is mapped at $FF0000-$FFFFFF. The reset vector at
$FFFF00 points into this region. The BIOS handles boot initialization,
system calls (via SWI 1 through a jump table at $FFFE00), and interrupt
dispatch.

See [ngpc_bios.md](ngpc_bios.md) for the complete BIOS reference including
the full system call table with parameters, boot sequence details, interrupt
dispatch mechanism, and HLE considerations.

### User Program Startup

After the BIOS transfers control, the user program must:

1. Configure interrupt vectors in RAM ($6FB8-$6FFC)
2. Set the display window (minimized on startup)
3. Enable interrupts with EI (IFF set to 0-2)
4. Begin main loop

---

## Programming Constraints

### System Restrictions

| Restriction | Description |
|-------------|-------------|
| Register bank 3 | Reserved for BIOS system calls. Must not be used by application software. |
| HALT instruction | Prohibited. Voltage management is handled by the BIOS. |
| Watchdog timer | Must write $4E to $6E at least once every ~100 ms |
| Interrupt mask | IFF must be kept at <= 2 during normal operation |
| VBlank interrupt | Must not be disabled. BIOS depends on it for power management. |
| User Shutdown | Must monitor $6F85 and call VECT_SHUTDOWN when set |

### Display Initialization

At user program startup, the K2GE window size is minimized. Before
rendering, the application must set the window registers. See
k2ge_reference.md for details.

### Palette RAM Access

Color palette RAM ($8200-$83FF) must be accessed with 16-bit word reads
and writes only. 8-bit access produces unreliable values.

---

---

## Sources

- [Neo Geo Pocket Technical Data (devrs.com)](https://www.devrs.com/ngp/files/ngpctech.txt) - Memory map, interrupt vectors, Z80 control registers, I/O register addresses
- [Neo Geo Pocket Specification (devrs.com)](http://devrs.com/ngp/files/DoNotLink/ngpcspec.txt) - CPU clocks, timer system, display timing, Z80 RAM, BIOS system calls, programming constraints, boot sequence, serial communication
- [SNK Neo Geo Pocket Hardware Information (Data Crystal)](https://datacrystal.tcrf.net/wiki/SNK_Neo_Geo_Pocket/Hardware_information) - System specifications, memory sizes, ROM header format
- [Neo Geo Pocket Color (Game Tech Wiki)](https://www.gametechwiki.com/w/index.php/Neo_Geo_Pocket_Color) - Hardware overview, system identification
- [NeoGeo Pocket Dev'rs Documentation Page](https://www.devrs.com/ngp/docs.php) - Development resource index
- [TMP95C061 Datasheet (bitsavers.org)](http://www.bitsavers.org/components/toshiba/_dataSheet/TMP95c061-ds.pdf) - Base CPU SFR register map, timer system, DMA, serial, interrupt controller, A/D converter
- [1994 Toshiba TLCS-900 16-Bit Microcontroller Databook (bitsavers.org)](http://bitsavers.trailing-edge.com/components/toshiba/_dataBook/1994_Toshiba_TLCS-900_16_Bit_Microcontroller.pdf) - CPU architecture, register file, instruction set encoding, condition codes, addressing modes
- [Neo Geo Pocket Emulation Thread (NESdev Forums)](https://forums.nesdev.org/viewtopic.php?t=18579) - Community hardware research including I/O register behavior, timer configuration, prescaler details
- [NGPC Flash Board (NeoGeo Development Wiki)](https://wiki.neogeodev.org/index.php?title=NGPC_flash_board) - Cartridge flash memory information, connector pinout
