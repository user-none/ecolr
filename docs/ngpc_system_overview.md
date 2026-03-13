# Neo Geo Pocket Color System Overview

High-level overview of the chips used by the NGPC and how they interact.
For detailed register maps, timing, and programming constraints, see the
component-specific references:

- [ngpc_system_reference.md](ngpc_system_reference.md) - CPU SoC, memory
  map, timers, interrupts, HDMA, input, cartridge, BIOS
- [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) - SoC peripheral
  register detail (16-bit timers, DMA registers, clock gear, RTC)
- [k2ge_reference.md](k2ge_reference.md) - K2GE video display controller
- [ngpc_sound_integration.md](ngpc_sound_integration.md) - Sound system
  integration
- [ngpc_bios.md](ngpc_bios.md) - BIOS ROM, system calls, boot sequence, HLE
- [ngpc_cartridge_format.md](ngpc_cartridge_format.md) - ROM file format,
  flash command protocol, save data

---

## Chips

| Chip | Role | Clock |
|------|------|-------|
| TLCS-900H (TMP95CS64F) | Main CPU SoC - game logic, system control, DAC output | 6.144 MHz (adjustable via clock gear down to 384 KHz) |
| K2GE (or K1GE for mono NGP) | Graphics engine - sprites, scroll planes, compositing | 6.144 MHz |
| Z80 | Sound CPU - runs sound driver, programs PSG | 3.072 MHz (master / 2, independent of clock gear) |
| T6W28 | PSG - 3 tone + 1 noise, stereo volume per channel | 3.072 MHz (Z80 rate, /16 internal divider) |
| 2x 6-bit DAC | PCM audio output (left + right) | Asynchronous, written by TLCS-900H |
| Flash ROM | Cartridge game code + save data (up to 4 MB) | Asynchronous, 8-bit bus with wait states |

All clocks derive from a single 6.144 MHz crystal. There is no NTSC/PAL
distinction.

---

## System Diagram

```
                          6.144 MHz Crystal
                                |
                +---------------+---------------+
                |                               |
                v               ,-- /2 -------->v
        +===============+      |        +===============+
        |  TLCS-900H    |      |        |     Z80       |
        |  (Main CPU)   |      |        | (Sound CPU)   |
        |  TMP95CS64F   |      |        +===============+
        |               |      |           |         |
        | +----------+  |      |      $4000|    $4001|
        | | Timers   |  |      |      (R)  |    (L)  |
        | | T0-T3    |<-|--H_INT--+        v         v
        | | T4-T7    |  |        |    +===============+
        | +----------+  |        |    |    T6W28      |
        | +----------+  |        |    |    (PSG)      |
        | | HDMA x4  |  |        |    +===============+
        | +----------+  |        |      |           |
        | +----------+  |        |      | PSG Left  | PSG Right
        | | Serial   |  |        |      v           v
        | +----------+  |        |    +---+ +---+ +---+
        +===============+        |    |   | |   | |   |
          |   |   |   |          |    | M | | A | | S |
          |   |   |   |          |    | I | | M | | P |
          |   |   |   +--$00A2-->+ DAC| X | | P | | K |
          |   |   |   +--$00A3-->+ DAC| E | |   | | R |
          |   |   |              |    | R | |   | |   |
          |   |   |              |    +---+ +---+ +---+
          |   |   |              |
     +----|---|---|--------------+
     |    |   |   |
     |    |   |   +-------------------------------+
     |    |   |                                   |
     |    |   v                                   v
     |    | +===============+            +===============+
     |    | |    K2GE       |            | Flash ROM     |
     |    | | (Graphics)    |            | (Cartridge)   |
     |    | +===============+            | CS0: 2 MB     |
     |    |   |          |               | CS1: 2 MB     |
     |    |   | H_INT    | INT4          +===============+
     |    |   | (HBlank) | (VBlank)
     |    |   v          v                  160x152
     |    +-- to Timer0  to INT ctrl         LCD
     |        (TI0)      ($6FCC)              ^
     |                                        |
     +-------- K2GE line buffer renders ------+
```

---

## Bus Connections

```
TLCS-900H <--16-bit bus--> Internal RAM (12 KB)
          <--16-bit bus--> K2GE ($8000-$BFFF, 16 KB window)
          <--8-bit bus---> Shared RAM ($7000-$7FFF, 4 KB) <--- Z80 ($0000-$0FFF)
          <--8-bit bus---> Flash ROM (CS0/CS1, wait states required)
```

### Inter-CPU Communication

| Mechanism | TLCS-900H Address | Z80 Address | Direction |
|-----------|-------------------|-------------|-----------|
| Shared register | $00BC | $8000 | Bidirectional (same physical register) |
| Z80 interrupt trigger | INT5 ($6FD0) | Write to $C000 | Z80 -> TLCS-900H |
| Sound chip enable/disable | $00B8 ($55/$AA) | -- | TLCS-900H -> T6W28 |
| Z80 enable/disable | $00B9 ($55/$AA) | -- | TLCS-900H -> Z80 |
| Shared RAM | $7000-$7FFF | $0000-$0FFF | Bidirectional (TLCS-900H has bus priority) |

---

## Interaction Patterns

### Graphics

The TLCS-900H writes sprite attributes, tile maps, scroll registers, and
character data into the K2GE's memory-mapped region ($8000-$BFFF). The K2GE
independently renders each scanline into a line buffer and outputs to the
LCD. The K2GE signals the CPU via two mechanisms:

- VBlank (INT4, user vector $6FCC) - fired after the last active scanline
- HBlank (H_INT) - 152 pulses per frame, routed to Timer 0's external
  clock input (TI0) for programmable per-N-scanline interrupts

### Sound - PSG Path

The TLCS-900H uploads a sound driver program into shared RAM ($7000-$7FFF),
then enables the Z80 and T6W28. The Z80 executes the driver and writes
directly to the T6W28's two write ports ($4000 right, $4001 left) to
control tone frequencies and per-channel stereo volume. The TLCS-900H
streams music data into shared RAM and sends commands via the communication
register ($00BC). The Z80 can interrupt the TLCS-900H back via a write to
$C000 (fires INT5).

### Sound - DAC Path

The TLCS-900H writes PCM sample values directly to DACL ($00A2) and DACR
($00A3) at a timer-driven rate. The Z80 has no access to the DACs. This
path is used for digitized audio (voice samples, sound effects).

PSG and DAC outputs are mixed in analog hardware before reaching the
speaker or headphone output.

### Cartridge

The cartridge connects via an 8-bit external data bus with 21 address lines.
Two chip selects (CS0, CS1) allow up to 4 MB total ROM. Wait states are
required due to the narrow bus. The same flash chips store both game code
and save data - there is no separate SRAM or EEPROM.
