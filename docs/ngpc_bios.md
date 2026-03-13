# Neo Geo Pocket Color BIOS Reference

Technical reference for the NGPC BIOS ROM, covering both real BIOS execution
and high-level emulation (HLE) as an alternative. The BIOS is a 64 KB ROM
mapped at $FF0000-$FFFFFF that initializes hardware, manages system services,
and provides a system call interface to application software.

For related hardware details, refer to:

- [ngpc_system_reference.md](ngpc_system_reference.md) - Memory map, interrupt
  system, flash storage, serial communication, power management
- [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) - SoC peripheral registers
  (timers, DMA, clock gear, RTC)
- [k2ge_reference.md](k2ge_reference.md) - K2GE video display controller

---

## Table of Contents

- [Overview](#overview)
- [Real BIOS Execution](#real-bios-execution)
  - [BIOS ROM File](#bios-rom-file)
  - [Initial CPU State](#initial-cpu-state)
  - [Boot Path Overview](#boot-path-overview)
  - [Cold Boot Flow](#cold-boot-flow)
  - [Warm Boot Flow](#warm-boot-flow)
  - [NVRAM Persistence](#nvram-persistence)
- [High-Level Emulation Alternative](#high-level-emulation-alternative)
  - [SWI 1 Dispatch](#swi-1-dispatch)
  - [HLE Interrupt Dispatch](#hle-interrupt-dispatch)
- [HLE Initialization Values](#hle-initialization-values)
- [System Call Interface](#system-call-interface)
- [System Call Details](#system-call-details)
- [Interrupt Dispatch](#interrupt-dispatch)
- [System RAM Layout](#system-ram-layout)
- [Hardware the BIOS Touches](#hardware-the-bios-touches)
---

## Overview

The BIOS serves three roles:

1. **Boot initialization** - hardware setup, user setup flow (language/clock),
   boot animation, cartridge validation, jump to game entry point
2. **Runtime services** - system calls for flash access, RTC, clock gear,
   interrupt priority, serial communication, power management
3. **Interrupt housekeeping** - the BIOS intercepts all hardware interrupts
   at the ROM vector table ($FFFF00), performs system tasks (watchdog service,
   input scanning, power monitoring), then dispatches to user handlers via the
   RAM vector table ($6FB8-$6FFC)

### Two Emulation Approaches

**Real BIOS**: Load and execute the actual BIOS ROM. Requires a complete
TLCS-900H CPU implementation and accurate hardware register emulation for
everything the BIOS accesses. This is the more accurate approach and avoids
needing to reverse-engineer every system call's behavior. MAME uses this
approach.

**HLE**: Replace BIOS functions with native emulator code. Avoids the need
for a BIOS ROM file and reduces hardware accuracy requirements. Mednafen,
NeoPop, and RACE all use this approach. Requires detailed knowledge of every
system call's parameters, behavior, and side effects.

Both approaches require a working TLCS-900H CPU for game code execution.

---

## Real BIOS Execution

### Requirements

Running the real BIOS ROM requires:

1. **TLCS-900H CPU** - full instruction set including SWI (software interrupt)
2. **SoC registers** - all SFR registers the BIOS accesses during boot and
   runtime (see [Hardware the BIOS Touches](#hardware-the-bios-touches))
3. **BIOS ROM image** - 64 KB, mapped at $FF0000-$FFFFFF (see
   [BIOS ROM File](#bios-rom-file) for the reference file used in
   this documentation)
4. **Flash controller** - the BIOS probes flash chip IDs and tests the last
   block on every boot
5. **K2GE registers** - the BIOS writes display mode, window size, and
   palette defaults during initialization
6. **RTC registers** - the BIOS reads/writes $91-$96 for time operations
   (6 registers: year, month, day, hour, minute, second in BCD)
7. **Interrupt controller** - priority registers $70-$7A, vector dispatch

### BIOS ROM File

All addresses, disassembly, and behavioral analysis in this document
are based on the following BIOS ROM:

| Property | Value |
|----------|-------|
| Filename | Neo Geo Pocket Color BIOS (JE).bin |
| Size | 65536 bytes (64 KB) |
| SHA-256 | 8fb845a2f71514cec20728e2f0fecfade69444f8d50898b92c2259f1ba63e10d |
| Variant | Japanese + English (two language options) |

The "JE" designation indicates this BIOS supports both Japanese and
English. Other BIOS variants (Japanese-only, European) may have
different language options or code paths.

### Initial CPU State

On reset, the TLCS-900/H initializes the following registers before
executing the first instruction:

| Register | Value | Meaning |
|----------|-------|---------|
| PC | read from $FFFF00-$FFFF02 | 24-bit reset vector (little-endian) |
| SR | $F800 | System mode, IFF=7 (all interrupts masked), MAX mode, bank 0 |
| XSSP | $0100 | System stack pointer initial value |

The IFF=7 value means all maskable interrupts are initially blocked.
The BIOS will lower the interrupt mask level after initialization is
complete.

### Boot Path Overview

The BIOS has two entry points and two distinct boot paths.

**Entry Points:**

| Entry | Address | Trigger |
|-------|---------|---------|
| Cold boot | $FF204A (via reset vector $FFFF00) | First power-on or battery-backed RAM lost |
| Warm boot | $FF1800 | Subsequent power-on with battery-backed RAM intact |

**Path to convergence:**

- Cold boot ($FF204A): full hardware init, clear all RAM, set $6F83
  bit 4, write $6E95=$4E50, run INT0 handler for cart validation,
  then jump to $FF1AB2
- Warm boot ($FF1800): minimal SFR init, re-probe flash, jump to
  $FF1A73 -> $FF1AB2

**Shared boot sequence ($FF1AB2):**

Both paths converge at $FF1AB2, which handles configuration
validation through cart handoff:

| Step | Address | Description |
|------|---------|-------------|
| 1 | $FF1AB2 | Configuration validation ($6E95 + checksum at $6C14) |
| 2 | $FF1B28 | Pre-animation setup (resume check, K2GE init, battery checks, interrupt priority init via $FF23F3) |
| 3 | $FF361E | Setup screen - only if $6F83 bit 4 set (config invalid) |
| 4 | $FF530C | SNK logo animation |
| 5 | $FF6840 | Startup screen |
| 6 | | Cart handoff |

The resume path ($6F86 bit 7 set) branches at $FF1B28, skipping
steps 3-4 and jumping directly to the startup screen (step 5).

**Boot Paths:**

1. **Cold boot** ($FF204A via reset vector):
   Hardware power-on with no prior configuration, or battery-backed
   RAM was lost. The BIOS performs full hardware initialization,
   resets the RTC to defaults (1999-01-01 00:00:00), clears all
   system state, and runs the interactive setup screen for language
   and color palette configuration. After setup, it runs the SNK
   logo animation, startup screen, then hands off to the cartridge.
   The boot process writes $6E95=$4E50 and computes a checksum into
   $6C14 so warm boot recognizes the configuration as valid.

2. **Warm boot** ($FF1800):
   Subsequent power-on with battery-backed RAM intact. Performs
   minimal SFR initialization, re-probes flash chips, then reaches
   the shared decision point at $FF1AB2. The configuration marker
   ($6E95) and checksum ($6C14) determine what happens next:
   - **Config valid** ($6E95==$4E50 + checksum passes): skips the
     setup screen, runs the SNK logo animation and startup screen,
     validates the cartridge, then hands off. This is the typical
     boot path for a system that has been set up at least once.
   - **Config invalid** ($6E95!=$4E50 or checksum fails): runs
     the full setup screen flow (language + color palette), then
     the SNK logo animation, startup screen, and cart handoff.

Additionally, within the warm boot flow there is a **resume path**
($6F86 bit 7 set) that skips the setup screen and SNK logo,
running only the startup screen before cart handoff. This sets
User Boot = Resume ($6F84 bit 5).

**Key state variables:**

| Address | Name | Role |
|---------|------|------|
| $6E95 | Config marker | $4E50 ("NP") when system has been configured |
| $6C14 | Config checksum | Sum of $6F87 + $6C25-$6C2B + $6F94; validated by $FF3341 |
| $6C47 | Setup state | $02 = setup complete (set by game alarm calls, not boot) |
| $6C7C | Boot cycle marker | $5AA5 after INT0 completes cart validation |
| $6F86 bit 7 | Resume flag | Set when resuming from sleep; skips setup + SNK logo |

**First-time setup screen configuration:**

The setup screen presents two user choices across 6 pages. Pages
auto-advance with timers between interactive pages. The page
sequence ($5C00 values 01-06) is: intro -> language selection ->
confirmation -> color palette selection -> confirmation -> done.

*Language* ($6F87, written at $FF3BA1/$FF3BAD):

| Value | Language |
|-------|----------|
| $00 | Japanese |
| $01 | English |

The language selection uses a binary left/right cursor ($5C12
toggles between $00 and $FF). Default is $00 (Japanese) when
$5C01=$FF, or $01 (English) when $5C01=$00. On warm boot with
invalid config, the setup screen pre-selects the current
language from $6F87.

*Color palette* ($6F94, written at $FF3BC0 from cursor at $5C13).
This selects the color gradient used when running monochrome (non-color)
games on the NGPC:

| Value | Palette | Gradient (K2GE $0BGR: bits 0-3=R, 4-7=G, 8-11=B) |
|-------|---------|---------------------------------------------------|
| $00 | Black & White | $0FFF $0DDD $0BBB $0999 $0777 $0444 $0333 $0000 |
| $01 | Red | $0FFF $0CCF $099F $055F $011D $0009 $0006 $0000 |
| $02 | Green | $0FFF $0BFB $07F7 $03D3 $00B0 $0080 $0050 $0000 |
| $03 | Blue | $0FFF $0FCC $0FAA $0F88 $0E55 $0B22 $0700 $0000 |
| $04 | Classic | $0FFF $0ADE $08BD $059B $0379 $0157 $0034 $0000 |

The color selection uses a 5-position left/right cursor (wraps
at boundaries). The palette data is stored inline at $FF50B5
(16 bytes per entry, 8 words of 12-bit RGB). The selection
calls $FF5086 to apply the palette in real-time as the user
navigates. The value is applied via $FF265C during the boot
color effect at step 43.

The RTC is initialized to a default (1999-01-01 00:00:00) during
first-time boot but is not user-configurable through the setup
screen.

### Cold Boot Flow

On cold boot, the CPU reads a 24-bit (3-byte, little-endian)
address from $FFFF00-$FFFF02 and begins executing BIOS code.
The reset vector points to **$FF204A** (BIOS ROM offset $204A).
This path performs full hardware initialization, clears system
state, and always runs the setup screens.

The BIOS boot sequence proceeds as follows:

1. Disables interrupts (DI)
2. Initializes watchdog (WDMOD=$04, WDCR=$B1)
3. Sets interrupt input mode (IIMC=$04, rising edge for INT0)
4. Cycles custom I/O register $80 through values $03,$02,$01,$00,
   writing $00 to $6C2C after each. The register has no known
   functional effect and is not read back by the BIOS
5. Configures SoC peripherals: port direction/function registers,
   timer mode registers, flip-flop control
6. Resets stack pointer (XSP=$6C00)
7. Switches to register bank 3 (LDF 3)
8. Initializes K2GE: unlocks $87F0=$AA, writes $53 to $87E0,
   executes a NOP, writes $47 to $87E0, re-locks $87F0=$55
9. Clears system state variables in RAM. The clearing is not
   contiguous - individual byte writes at $FF247D-$FF253C clear:
   $6F84-$6F86, $6C3D-$6C46, $6C62-$6C69, $6C6B-$6C6C,
   $6C47-$6C48, $6F87, $6F83, $6F80-$6F81, $6F8B-$6F8F,
   $6E89-$6E8A, $6F94. Sets $6F91=$10 (color mode) and $6E88=$01
10. Clears input/comm/power/A-D state at $FF253D-$FF25AB:
    $6F82 (input state), $6C5F-$6C60 (input edge detection),
    $6F88, $6F89 (A/D flag), $6F8A (LED flag), $6C61 (A/D
    in-progress), $6C16 (auto power-off counter), $6C18 (A/D
    counter), $6C1A (A/D interval), $6C1C (sleep counter),
    $6C1E, $6C20-$6C22 (power button state), $6C57, $6C5E,
    $6E80 (flash ready), $6F95 (color mode). Writes $FF to
    $9FFFFE (flash address). Sets RTS disabled (OR $B2 bit 0)
11. Initializes RTC registers $90-$9B: sets $90=$00 (clear
    control), $91=$98 (year), $92=$01 (month), $93=$01 (day),
    $94-$96=$00 (hour/min/sec), $97-$9B=$00. Then configures
    RTC control register $90 (AND $FD clear bit 1, OR $01
    set bit 0)
12. Writes setup checksum input values ($6C24=$0A, $6C25=$DC,
    $6C26-$6C2B=$00)
13. Initializes RAM vector table ($6FB8-$6FFC): fills all 18
    entries with the default handler address $FF23DF using
    `LD.L (XIX+),XIY` in a DJNZ loop (XIX=$6FB8, XIY=$FF23DF,
    BC=$12). Then overwrites the VBlank vector ($6FCC) via a
    table lookup at $FF23C3 indexed by A (A=0 during cold boot)
14. Sets bit 4 of $6F83 (purpose unknown)
15. Computes setup completion checksum (see below)
16. Updates watchdog (WDMOD=$14, WDCR=$B1)
17. Jumps to shared boot path at $FF1074:
    - RES 6,($6F83)
    - Re-initializes port/timer SFR registers and IIMC (second
      pass, same values as step 5 plus $7B=$04 from step 3)
18. Writes boot marker $A55A to $6C7C
19. Reads $6F88, masks to bits 0-2, stores result to $6C57
20. Sound init (CALR $FF3384):
    - Stops prescaler: AND ($20),$F7
    - Disables Z80 and T6W28 ($B8=$AAAA)
    - NOP delays (6 NOPs)
    - Re-enables T6W28 only ($B8=$AA55, Z80 stays disabled)
    - Mutes all 4 T6W28 channels via registers $A0 and $A1:
      writes $9F, $BF, $DF, $FF to each (volume-off commands,
      with NOP delays between writes)
    - Re-disables both T6W28 and Z80 ($B8=$AAAA)
    - Clears sound state ($6DA1=$00000000, $BC=$00, $6DA4=$00)
21. Writes $80=$04 (no known effect), $6C2C=$00, 4 NOPs, then
    reads/modifies WDMOD (AND $F2, OR $14)
22. Display init (CALR $FF115A):
    - Disables interrupts (DI)
    - Checks power state: CP.W ($B6),$0050
    - **Path 1** ($B6==$0050, used on boot cycle repeats):
      Sets K2GE registers: $83F0=$0FFF, $8012=$00, $8004=$00,
      $8005=$00, $8002=$00, $8003=$00. Calls $FF25DF (clear
      tile/sprite RAM), $FF263C (clear OAM). K2GE unlock/init
      ($87F0=$AA, $87E0=$53, NOP, $87E0=$47, $87F0=$55).
      LED off ($8400=$00), flash cycle ($8402=$FF). RET (no
      frame delays)
    - **Path 2** ($B6!=$0050, used on first cold boot pass):
      Checks K2GE status: CP.B ($87E0),$01. If not ready:
      writes K2GE mode ($87F0=$AA, $87E0=$47, $87F0=$55).
      Frame delay x1 via $FF112F. Sets K2GE registers:
      $83F0=$0FFF, $8012=$00, $8004=$00, $8005=$00, $8002=$00,
      $8003=$00. Frame delay x4. Writes $B6=$0050. Calls
      $FF25DF (clear tile/sprite RAM), $FF263C (clear OAM).
      K2GE unlock/init ($87F0=$AA, $87E0=$53, NOP, $87E0=$47,
      $87F0=$55). LED off ($8400=$00), flash cycle ($8402=$FF).
      Frame delay x4. RET
23. Sets interrupt priorities:
    - $70=$0D (INT0 priority=5, rising edge select)
    - $71=$00, $72=$00, $73=$00, $74=$00, $77=$00, $79=$00,
      $7A=$00 (all other interrupts disabled)
24. Clears additional state:
    - $6C16=$0000, $6C1C=$0000, $6C1E=$0000
    - $6F85=$00 (shutdown request), $6F84=$00 (user boot)
    - RES 3,($6F83)
    - $6E80=$00
25. Frame delay via $FF112F
26. Resets stack pointer (XSP=$6C00)
27. Enables interrupts: EI 5 (IFF=7 -> IFF=5)
28. Enables power button NMI: SET 2,($B3)
29. Writes markers: $6C7A=$A5A5, $6E95=$4E50
30. Writes $B4=$00A0 (purpose unknown, may configure interrupt
    source)
31. Executes HALT at $FF1127 - CPU stops and waits for an
    interrupt. Only INT0 (priority 5) is enabled at this point.
    The CPU will wake when INT0 fires.
32. (After INT0 fires) Clears bit 7 of $6F86 (RES 7,($6F86) at
    $FF1128). Note: the INT0 handler runs before returning here
    because the CPU services the interrupt immediately after HALT
    resumes. After RETI, execution continues at $FF1128, then
    unconditionally jumps back to the shared boot path at
    $FF1074 (JRL T,$FF1074 at $FF112C). This creates a boot
    cycle: HALT -> INT0 handler -> RETI -> $FF1128 -> $FF1074 ->
    re-init -> HALT. The cycle repeats identically until the
    battery voltage check passes.

#### INT0 Handler ($FF2856)

The INT0 handler at $FF2856 (ROM vector $FFFF28):

1. Stack validation: checks $4800 <= XSP < $6C01. If out of
   range, jumps to $FF284D (stack recovery).
2. Boot state check: compares $6C7C against $5AA5. Step 18
   writes $A55A (not $5AA5), so this fails on the first boot cycle.

**First boot cycle path** ($FF2898):

3. Calls $FF1E7A - A/D converter battery voltage read (see below)
4. Compares $6F80 >= $016E (battery voltage threshold 1)
5. If voltage < threshold: takes low battery path at $FF2A1E
6. If voltage >= threshold ($FF28A4): second A/D read via $FF1E7A
7. Compares $6F80 >= $016E again (second check)
8. If voltage < threshold: takes low battery path at $FF2A1E
9. If voltage >= threshold ($FF28B0): SFR hardware init:
   - WDMOD=$14, WDCR=$B1, IIMC=$04 ($7B)
   - Port/timer registers: same set as boot step 5 except
     $0D=$09 (P6, vs $08 in step 5) and $1E=$0F (PA, vs
     $03 in step 5)
   - Writes $B4=$000A, then NOP delay loop (250 iterations of
     10 NOPs + DJNZ at $FF2924-$FF292E)
10. Third A/D read via $FF1E7A
11. Compares $6F80 >= $0210 (battery voltage threshold 2, higher)
12. If voltage < threshold: takes low battery path at $FF2A1E
13. If voltage >= threshold ($FF293E): DI, then $80 cycling
    ($03,$02,$01,$00 with $6C2C=$00 after each, no known effect)
14. Checks $6C47 == $02 (language setup done): if yes, jumps
    to $FF2A25 (setup-complete path, see below)
15. Checks $6C46 != $00 (cart type flag): if nonzero, continues
    to cartridge validation. If $6C46 == $00: falls through
    to $FF2A1E (same as low battery path)
16. Calls $FF309C - flash chip ID probing (see
    [Flash Chip ID Probing](#flash-chip-id-probing-ff309c))
17. Checks $6C59 (CS1 present): if nonzero, checks $6C5B
    (CS1 protection) == $01; if so, restarts boot at $FF1074.
    If $6C59 == $00, checks $6C58 (CS0 present) == $00; if
    so, takes low battery path at $FF2A1E
18. Calls $FF326C - cart title match against previous session
    (see [Cart Title Match](#cart-title-match-ff326c)). If
    nonzero (mismatch): clears bit 7 of $6F86
19. Calls $FF314C - license validation and header copy (see
    [License Validation and Header Copy](#license-validation-and-header-copy-ff314c))
20. Copies $6C04 to $6E82 (software ID duplicate), $6C06 to
    $6E84 (sub-code duplicate)
21. Calls $FF32DA - cart title match against saved state (see
    [Cart Saved State Match](#cart-saved-state-match-ff32da)).
    If nonzero (mismatch): takes low battery path at $FF2A1E
22. K2GE unlock/init ($87F0=$AA, $87E0=$53, NOP, $87E0=$47,
    $87F0=$55), LED init ($8400=$FF, $8402=$00), sets
    $B6=$0005
23. Fourth A/D read via $FF1E7A, compares $6F80 >= $0210;
    if voltage < threshold: takes low battery path at $FF2A1E
24. DI, calls $FF2462, sets A=$06, calls $FF239D (RAM vector
    table init with A=6), calls $FF25AC
25. Sets $6E8B=$07, EI 0, calls $FF1F76 (frame delay)
26. DI, calls $FF23E0, calls $FF253D, calls $FF25DF (clear
    tile/sprite RAM), calls $FF263C (clear OAM)
27. Sets $6E8B=$08, EI 0, calls $FF1F76 (frame delay)
28. DI, sets bit 7 of $6F84 (User Boot = Alarm)
29. Writes $6C7C=$5AA5 (boot cycle marker - note this is the
    byte-swapped value that the boot state check at step 2
    expects)
30. Calls $FF2AC5, pops WA, jumps to $FF1AB2

**Setup-complete path** ($FF2A25):

When $6C47=$02 (language setup already done), the handler
jumps here. This path bypasses the initial cart-present check
and flash ID probing but re-probes flash later:

1. K2GE unlock/init ($87F0=$AA, $87E0=$53, NOP, $87E0=$47,
   $87F0=$55), LED init ($8400=$FF, $8402=$00)
2. Sets $B6=$0005
3. A/D battery read via $FF1E7A, compares $6F80 >= $0210;
   if voltage < threshold: takes low battery path at $FF2A1E
4. DI, calls $FF2462, sets A=$06, calls $FF239D, calls $FF25AC
5. Sets $6E8B=$07, EI 0, calls $FF1F76 (frame delay)
6. DI, calls $FF23E0 (sound system startup), calls $FF5001
   (animation init)
7. Calls $FF253D (clear input/comm), $FF25DF (clear tile/sprite
   RAM), $FF263C (clear OAM)
8. Sets $6E8B=$08, EI 0, calls $FF1F76 (frame delay)
9. DI, calls $FF309C (flash chip ID probing)
10. Checks $6C59 (CS1 present): if nonzero, checks $6C5B
    == $01 (CS1 protection); if so, restarts boot at $FF1074
11. Calls $FF2AC5, writes $6C7C=$5AA5
12. Re-checks CS1 ($6C59/$6C5B): if CS1 not present or not
    protected, services watchdog ($6F=$4E, $6E=$F0)
13. Sets A=$05, calls $FF239D (RAM vector table init with
    A=5), calls $FF476C
14. DI, pops WA, jumps to $FF1074 (restart boot cycle)

**Low battery / no-cart path** ($FF2A1E):

16. Calls $FF2AC5 - checks user setup state ($6C47 and $6C48)
17. If both $6C47 and $6C48 are 0 (no setup): jumps to $FF2B1B
    which configures $90 (AND $FD, OR $01 - RTC control)
18. Returns, pops WA, then JRL T,$FF1074 (restart boot cycle)

**Subsequent boot cycle path** (marker matches $5AA5): checks
$6C47==$02, $6F83 bit 6 set, and $6C46==$FF. If all three pass:
calls $FF2AC5, pops WA, dispatches to user INT0 handler via RAM
vector table entry at $6FC8/$6FCA. If any check fails: calls
$FF2AC5, pops WA, RETI.

#### A/D Converter Read ($FF1E7A)

The battery voltage read subroutine at $FF1E7A:

1. Pushes SR, WA, XBC
2. Sets $6C61=$FF (A/D conversion in progress flag)
3. Calls $FF2E51 (A/D configuration):
   - Reads $6C24, masks low nibble, ORs with $60
   - Writes result to $6C24 and $70 (INTE0AD - reconfigures INT0
     priority for A/D trigger mode)
   - Sets $6D=$04 (ADMOD - starts A/D conversion, START bit)
   - Sets bit 7 of $6F89 (A/D conversion started flag)
4. Calls $FF1ED7 (watchdog service):
   - If $6C59==$00, writes $6F=$4E (service watchdog)
5. EI 6 (allow priority 6+ interrupts during wait)
6. Polls $6F89 bit 7 with $2FFFF (196,607) iteration timeout,
   waiting for A/D conversion complete interrupt to clear the bit
7. After timeout: sets $6F80=$0000, $6C61=$00
8. Restores XBC, WA; DI; restores SR from stack

The A/D conversion is handled by the ADC peripheral component
(emu/adc.go). The INTAD ISR at $FF2DCE reads the ADREG result
and stores the 10-bit value to $6F80 as a word. With AN0
defaulting to $3FF (full battery), the BIOS passes both voltage
thresholds ($016E and $0210).

#### INTAD ISR ($FF2DCE)

The A/D conversion complete interrupt handler at $FF2DCE (vector
offset $70, vector index 28):

1. PUSH SR, LDF 3, PUSH WA
2. AND ($6C24),$0F - clears upper nibble of priority config
3. LD ($70),($6C24) - restores INTE0AD to INT0-only priority
4. LD.W WA,($60) - reads ADREG0 result (word read: $60=low, $61=high)
5. SRL.W 6,WA - shifts right by 6 to get 10-bit value from
   ADREG format (low byte has result<<6 in upper bits)
6. LD.W ($6F80),WA - stores battery voltage result
7. LD.W ($6C1A),$0258 - sets conversion interval timer
8. LD.W ($6C18),$0000 - clears conversion counter
9. CP.W WA,$0200 - branches on voltage level
10. **Voltage >= $0200** (good): if $6F8A bit 7 clear and $B3
    bit 2 set, writes LED registers ($8400=$FF, $8402=$00).
    Then RES 7,($6F89), POP WA, DI, POP SR, RETI
11. **Voltage < $0200** (low): if $6F8A bit 7 clear and $B3
    bit 2 set, writes LED warning ($8400=$17, $8402=$30).
    Then checks second threshold CP.W WA,$01E2:
    - If >= $01E2: sets $6C1A=$003C (shorter A/D interval),
      SET 6,($6F89)
    - If < $01E2: checks $6C61!=$FF (not mid-conversion),
      if so SET 5,($6F85) (triggers shutdown)
    Then RES 7,($6F89), POP WA, DI, POP SR, RETI

#### Display Init ($FF115A)

See Reset Boot Flow step 22 for the full two-path description
of $FF115A. Path 1 ($B6==$0050) is a short reinit with no
frame delays. Path 2 ($B6!=$0050) is the full init with frame
delays, K2GE mode check, and $B6 write.

The following steps occur after the boot cycle completes (battery
voltage must pass the $016E threshold to exit the HALT loop):

33. Configuration validation at $FF1AB2: checks $6E95 config
    marker and $FF3341 checksum. On first-time boot (fail):
    resets RTC, clears state, sets $6F83 bit 4 (setup needed).
    On config valid (pass): skips to $FF1B28
34. Pre-animation setup: battery checks, boot markers, interrupt
    priority init, sound system startup, animation init
35. If $6F83 bit 4 set (first-time): runs setup screen ($FF361E)
    for language and time/date configuration
36. Runs SNK logo animation ($FF530C)
37. Runs startup screen ($FF6840)
38. Re-probes flash, validates license, verifies software ID
39. Sets User Boot flag, jumps to cart entry point from ($6C00)

Note: steps 1-32 have been verified via BIOS stepping trace.
The INT0 handler first boot cycle path has been fully traced
including the low battery path, the boot progression path
(voltage >= $016E), and the cartridge validation path (steps
16-30). The BIOS performs three battery voltage reads with two
thresholds ($016E and $0210), then hardware init, then checks
cart/setup state. Without a cartridge ($6C46=$00) or completed
setup ($6C47!=$02), the BIOS loops back to $FF1074 and repeats
the boot cycle. With a cartridge present, the BIOS probes flash
chip IDs, validates the license string and header, performs a
fourth battery voltage read, then proceeds to boot animation
setup before jumping to $FF1AB2. Steps 33-39 correspond to the
post-animation sequence documented in the
[Post-Animation Boot Sequence](#post-animation-boot-sequence-ff1ab2)
section below.

#### Post-Animation Boot Sequence ($FF1AB2)

After the INT0 first boot cycle path completes cart validation
(step 30), control jumps to $FF1AB2. The warm boot path also
reaches this address via $FF1A73. The routine handles
configuration validation, the boot animation, and post-animation
re-initialization before handing off to the cartridge.

##### Configuration Validation ($FF1AB2-$FF1B27)

This is the shared decision point. Both the cold boot flow (via
INT0 step 30) and the warm boot path (via $FF1A73) converge here.

1. RES 3,($6F83)
2. Checks $6E95==$4E50 (config marker "NP"). If match: calls
   $FF3341 (config checksum validation - sums $6F87 + $6C25-$6C2B
   + $6F94, compares against $6C14). If checksum passes (A==0):
   jumps to $FF1B28 (skip setup)
3. If marker mismatch or checksum fail (config invalid): initializes
   RTC registers ($90=$00, $91=$99, $92-$9B=$01,$01,$00,...,$00),
   configures RTC control ($90 AND $FD, OR $01)
4. Writes setup checksum input values ($6C24=$0A, $6C25=$DC,
   $6C26-$6C2B=$00)
5. Calls $FF247D (clear system state), $FF253D (clear
   input/comm state), $FF239D with A=$00 (RAM vector table
   init)
6. SET 4,($6F83)

##### Pre-Animation Setup ($FF1B28-$FF1B86)
7. Checks bit 7 of $6F86. If set: jumps to $FF1D07
   (resume path, see below)
8. K2GE unlock/init ($87F0=$AA, $87E0=$53, NOP, $87E0=$47,
   $87F0=$55)
9. LED init ($8400=$FF, $8402=$00)
10. Battery voltage read via $FF1E7A, compare $6F80 >= $0210.
    If below: jumps to $FF1E5C (low battery handler)
11. DI, writes $B6=$0005
12. Second battery voltage read via $FF1E7A, compare >= $0210.
    If below: jumps to $FF1E5C
13. DI, writes $6C7C=$5AA5, $6C7A=$A5A5
14. Calls $FF23F3 (interrupt priority + SoC peripheral init:
    sets $70=$0A INT0 priority=2, $71=$DC INT4=4/INT5=5,
    $72-$74/$77/$79/$7A=$00 disable others. Mirrors to
    $6C24-$6C2B. Configures timers $52=$49/$51=$20/$53=$15,
    DMA $24=$03/$28=$05/$25=$B0, serial $22=$01/$23=$90/
    $26=$90/$27=$62, prescaler $38=$30/$48=$30, enables
    DMA $20=$80, sets $52 OR $20)
15. Checks $6C55 == $02. If not $02: calls $FF23E0 (sound system
    startup: stops prescaler, disables sound/Z80, clears $6DA2
    bit 7, copies BIOS Z80 sound driver from $FF0000 to $7000
    (4KB), then enables both sound chip and Z80 via $B8=$5555)

##### Boot Animation ($FF1B87)

16. CALR $FF5001 - animation initialization. $FF5001 is not
    an animation loop. It clears animation state at $6DD8
    (28 longs set to 0), fills palette data at $6E48 (8 words
    set to $0FFF), initializes $6DC6 with $FFFFFFFF (18 bytes),
    sets $6DC5=$00, and returns.

The boot animation is driven by VBlank (INT4) interrupts.
The BIOS VBlank handler at $FF2163 performs system tasks
(input scanning, sound DMA, power monitoring) then dispatches
to a user-configurable handler via ($6FCC/$6FCE). During
boot, $FF239D configures this vector to point to the
animation frame advance code. The code between $FF1B87 and
the post-animation sequence uses EI/HALT or frame delay
calls to let VBlank interrupts fire.

##### Post-Animation Re-initialization ($FF1B8A-$FF1BA6)

After the animation completes, the BIOS re-initializes display
state to provide a clean slate for the game:

17. Calls $FF2631 (clear K2GE window position: $8004=$00,
    $8005=$00)
18. Calls $FF253D (clear input/comm state)
19. Calls $FF25DF (clear tile/sprite RAM: fills $9000-$9FFF
    and $9800-$9FFF with $0020)
20. Calls $FF263C (clear OAM: fills $8800-$88FF with $0000,
    sets $8020=$00, $8021=$00)
21. LD A,$03; CALR $FF8D8A - font decompression with color=3.
    Decompresses 1bpp font data to 2bpp K2GE character RAM at
    $A000
22. Calls $FF25AC (K2GE hardware init: unlock $87F0=$AA,
    $87E0=$47, $87F4=$80, $87F2=$00, $87E2=$80 mono mode,
    lock $87F0=$55, $8012=$00, $8118=$80, $8000=$C0 display
    enable, $6F95=$00)
23. Calls $FF261C (K2GE scroll/window defaults: $8002=$00,
    $8003=$00 scroll origin, $8004=$A0, $8005=$98 window
    size 160x152)
24. Checks $6C55 == $02 (commercial game flag). If $02: jumps
    to $FF1C85 (Path 2). Otherwise continues to Path 1.

##### Cart Entry - Path 1 ($FF1BA9-$FF1C84)

Used when $6C55 != $02:

25. SET 2,($B3) - enable power button NMI
26. Flash chip re-verification: checks $6C59 (CS1 present).
    If nonzero: checks $6C5B == $01 (CS1 protection). If CS1
    not present or not protected: services watchdog ($6F=$4E,
    $6E=$F0)
27. Checks $6F83 bit 4 (first-time setup needed). This bit is set
    during Configuration Validation step 6 when the config checksum
    fails, and cleared when config is valid. If set:
    - Calls $FF201B (no-op RET in JE BIOS)
    - Calls $FF26FA (no-op RET in JE BIOS)
    - SET 6,($6F86)
    - Calls $FF239D with A=$01 (RAM vector table init)
    - Calls $FF361E (first-time setup screen: clears display, loads
      font, initializes input state at $5C00-$5C12, enters
      interactive setup loop at $FF35E7 for language/time
      selection with EI 0)
    - DI, RES 6,($6F86)
28. RES 4,($6F83)
29. Calls $FF239D with A=$02 (RAM vector table init)
30. Calls $FF530C (SNK logo display: clears K2GE scroll/OAM
    regs, sets default palettes $8101-$8113, calls $FF518B
    and $FF503D for animation setup, fills tilemaps $9800
    with $00030003 and $9000 with zeros, configures display
    engine at $4E00-$4E0A, EI 0, runs animation loop
    waiting for completion via $6DA2 bit 7 and frame counter
    at $4E01)
31. DI, calls $FF1ED7 (watchdog service)
32. Calls $FF239D with A=$03 (RAM vector table init)
33. Calls $FF6840 (startup screen display: calls $FF3556 for
    display prep, loads data table from $FF685E, sets
    $6DAD=$00, $64E6=$00, EI 0, enters main loop at
    $FF35E7)
34. DI, calls $FF1ED7 (watchdog service)
35. Calls $FF239D with A=$00 (RAM vector table init)
36. Re-probes flash chip IDs:
    - LD XIX,$00800000; CALR $FF30F8 (CS1 probe)
    - Compares result against $6C59/$6C5B
    - LD XIX,$00200000; CALR $FF30F8 (CS0 probe)
    - Compares result against $6C58/$6C5A
    - If any mismatch: clears $6C59/$6C58/$6C5B/$6C5A to $00
37. Calls $FF2E77 (flash chip detection on CS0 at $200000:
    skips if $6C59 != 0 (CS1 already present). Identifies
    manufacturer from byte 0: $98=Toshiba, $EC=Samsung,
    $B0=Sharp, stores type in $6E87. Validates byte 3
    has bit 7 set. Determines flash size from byte 1:
    $AB=256KB ($6E86=$0A), $2C=512KB ($6E86=$12),
    $2F=1MB ($6E86=$22). Cross-validates CS1 chip at
    computed address. On any failure: clears all chip
    state $6C58-$6C5B, $6E80=$00, RES 7,($6F86))
38. Calls $FF314C (license validation and header copy)
39. Verifies software ID: compares ($6E82) against ($6C04)
    and ($6E84) against ($6C06). If mismatch: jumps to $FF1E5C
    (low battery handler)
40. Checks $6F84 bit 7. If not set: SET 6,($6F84) (User Boot
    = Power ON)
41. LD.B ($6F86),$00
42. SET 6,($6F83)
43. Calls $FF1EB8 (pre-cart flash prep: if $6C55 != 0, calls
    $FF337F, sets $B8=$AAAA watchdog, RES 7,($6DA2)).
    Calls $FF1ECB (if $6C55 == 0: SET 6,($6F86)).
    Calls $FF1EE3 (startup color effect: sets vector table
    mode 6 via $FF239D, EI 0, cycles $6E8B through phases
    7->1->6 with $FF1F76 delay loops waiting for VBlank
    handler to clear $6E8B, calls $FF265C palette select,
    different paths for mono vs color based on $6F90.
    DI, restores vector table mode 0)
44. Calls $FF1ED7 (watchdog service)
45. PUSH.W ($6C02); PUSH.W ($6C00) - pushes entry point
    address onto stack as two words
46. LDF 0 - switches to register bank 0
47. RET - pops the pushed entry point, jumping to the cart

##### Cart Entry - Path 2 ($FF1C85-$FF1D05)

Used when $6C55 == $02 (commercial game):

25. Calls $FF2E77 (flash chip detection, see Path 1 step 37)
26. Re-probes flash chip IDs (same sequence as Path 1 step 36)
27. If chip state changed from stored values: clears
    $6C59/$6C58/$6C5B/$6C5A, calls $FF314C (license
    validation), then jumps back to $FF1BA9 (Path 1 step 25)
28. Configures K2GE mode from $6F90 (system code):
    - If $6F90 < $10: $87F0=$AA, $87E2=$80, $87F0=$55 (mono)
    - If $6F90 >= $10: $87F0=$AA, $87E2=$00, $87F0=$55 (color)
29. SET 2,($B3) - enable power button NMI
30. LD.B ($6F86),$00
31. SET 6,($6F83)
32. LD.L XIX,($6C00) - loads entry point from $6C00
33. LDF 0 - switches to register bank 0
34. JP T,(XIX) - jumps to cart entry point

##### Resume Path ($FF1D07)

When $6F86 bit 7 is set at $FF1B28, this path runs instead
of the normal animation sequence. This is the resume-from-sleep
path (User Boot = Resume). It skips the first-time setup screen
and the SNK logo, running only the startup screen before cart
handoff.

1. K2GE unlock/init ($87F0=$AA, $87E0=$53, NOP, $87E0=$47,
   $87F0=$55)
2. LED init ($8400=$FF, $8402=$00)
3. Battery voltage read, compare >= $0210. If below: $FF1E5C
4. DI, $B6=$0005
5. Second battery voltage read, compare >= $0210
6. DI, $6C7C=$5AA5, $6C7A=$A5A5
7. Calls $FF23F3 (interrupt priorities)
8. Calls $FF23E0 (sound system startup)
9. CALR $FF5001 (animation init)
10. Post-animation cleanup: $FF2631, $FF253D, $FF25DF, $FF263C,
    font decompression ($FF8D8A with A=$03), $FF25AC, $FF261C
11. Restores $80 from $6C57 (with NOP delays), restores
    interrupt priorities $70-$7A from $6C24-$6C2B
12. SET 2,($B3), CS1/watchdog check (same as Path 1 step 26)
13. Calls $FF239D with A=$03, calls $FF6840 (startup screen).
    Skips setup screen (step 27) and SNK logo (step 30)
14. DI, watchdog, $FF239D with A=$00
15. Flash re-probe (same as Path 1 step 36)
16. Calls $FF2E77, $FF314C (same as Path 1 steps 37-38)
17. Software ID verify (same as Path 1 step 39)
18. RES 7,($6F86), SET 5,($6F84) (User Boot = Resume)
19. SET 6,($6F83), calls $FF1EB8, $FF1ECB, $FF1EE3, $FF1ED7
20. PUSH.W ($6C02), PUSH.W ($6C00), LDF 0, RET (cart entry)

##### CPU State at Cart Entry

Both paths switch to register bank 0 (LDF 0) before jumping.
The entry point is read from $6C00 (copied from cart header
offset $1C during license validation at $FF314C).

| Register | Value |
|----------|-------|
| PC | ($6C00) = cart header offset $1C (24-bit LE) |
| SR | bank 0 selected (LDF 0). IFF and other SR bits TBD |
| XSP | TBD (stack used for PUSH/RET in Path 1) |

The exact values of general-purpose registers (XWA, XBC, XDE,
XHL, XIX, XIY, XIZ) and the full SR at cart entry have not been
captured. Dynamic tracing through the boot animation is needed
to observe the final register state.

#### Frame Delay Routine ($FF112F)

The BIOS calls a VBlank-synchronized delay routine at $FF112F
multiple times during boot (10 total calls observed). The routine:

1. Clears XDE3 to 0
2. First phase: increments XDE3, compares to $6FFFF (458,751).
   If counter reached, exits. Otherwise checks BIT 6,($8010)
   (K2GE VBlank status). If bit 6 is set (JR NZ), restarts
   counter from 0. This waits for VBlank to be inactive.
3. Second phase: same counter loop but checks for bit 6 clear
   (JR Z continues counting). This waits for VBlank to become
   active again.

The routine effectively waits for one complete VBlank toggle
cycle, with a timeout of ~458K iterations per phase. The
emulator must toggle K2GE $8010 bit 6 based on scanline
position within the frame for this routine to function
correctly. Timing: 515 CPU clocks per scanline, 152 active
scanlines, 199 total scanlines per frame (see K2GE reference
Scanline Timing section). VBlank status (bit 6 set) spans
scanlines 152-198 of each frame.

#### Screen Main Loop ($FF35E7)

The setup screen ($FF361E), SNK logo ($FF530C), and startup
screen ($FF6840) all share a common interrupt-driven main loop
at $FF35E7. The pattern is:

1. Screen init routine sets $64E6 = $00, enables interrupts
   (EI 0), sets up a VBlank user handler via $FF239D, then
   jumps to $FF35E7
2. $FF35E7: CP.B ($64E6),$00 / JR NZ,$FF35E7 - spins until
   the VBlank handler clears $64E6 to 0
3. Once $64E6 is 0: loads a function pointer table from
   ($6440), calls the current handler via CALL T,(XIX)
4. Advances through the table entries (each entry has a
   function pointer, next-table pointer, and completion flag)
5. When the table is exhausted ($FFFF sentinel at offset 8):
   checks $64E5. If $FF: returns (screen complete). Otherwise
   sets $64E6=$FF and loops back to step 2

The VBlank INT4 handler ($FF2163) dispatches to the screen's
user handler via ($6FCC/$6FCE). The user handler processes one
frame of animation/input and clears $64E6 to signal the main
loop. This requires working INT4 interrupts - without them,
$64E6 is never cleared after being set to $FF and the loop
spins forever.

#### Priority Queue Structure

The screen main loop uses a linked-list priority queue to
manage concurrent per-frame functions. Each queue node is a
64-byte ($40) struct allocated from a pool at $64C0:

| Offset | Size | Description |
|--------|------|-------------|
| +0 | 4 | Function pointer (called each frame) |
| +4 | 2 | Previous node pointer |
| +6 | 2 | Next node pointer |
| +8 | 2 | Priority ($0000=highest, $FFFF=sentinel) |
| +10 | 1 | Removal flag ($FF = remove on next iteration) |
| +12 | 2 | Cross-reference pointer (to related node) |
| +14 | 1 | Frame counter (used by state machine) |
| +20-51 | varies | Per-function working data |
| +48 | 1 | Busy flag ($FF=busy, $00=ready) |
| +52 | 2 | Sprite/tile node pointer |
| +58 | 1 | Animation completion flag |
| +60 | 1 | State index |

Two fixed nodes anchor the list:

- $6440: head node (priority $0000), always present
- $6480: tail node (priority $FFFF), always present

**Pool management** ($64C0-$64E4):

The pool at $64C0 is a circular FIFO of 16 two-byte node
addresses (32 bytes, $64C0-$64DF). Three management variables
control it:

| Address | Size | Description |
|---------|------|-------------|
| $64C0 | 32 | Pool slots (16 x 2-byte node addresses) |
| $64E0 | 2 | Free index (where next freed node is written) |
| $64E2 | 2 | Alloc index (where next allocation reads from) |
| $64E4 | 1 | Active node count |

On init ($FF3556), the pool is filled with sequential node
addresses: $6000, $6040, $6080, $60C0, ... $63C0. Both indices
start at $0000. Both indices wrap with AND $001F.

$FF34CB (insert) allocates a node from pool[$64E2], zeroes it
via $FF35C4 (16 iterations of LD.L (XIY+),XHL), sets the
function pointer and priority, then walks the chain from $6440
to find the insertion point (first node with priority > new).
It links the new node in and increments $64E2 and $64E4.

$FF3524 (remove) unlinks a node from the chain by updating its
neighbors' prev/next pointers, then writes the node address to
pool[$64E0], increments $64E0, and decrements $64E4.

The remove function uses `LD.W (XIX+HL),IZ` at $FF353F to
write the freed node address to the pool. This instruction uses
the (R+r16) destination addressing mode with XIX as base ($64C0
loaded via LDA) and HL as the index (free index loaded from
$64E0).

The main loop at $FF35E7 walks the chain from $6440, calling
each node's function. If a function sets offset +10 to $FF,
the node is removed on the next iteration via $FF3524. When
the chain reaches the $FFFF sentinel (tail node at $6480), the
loop checks $64E5: if $FF the screen is complete and the loop
returns. Otherwise it sets $64E6=$FF and waits for the next
VBlank.

#### Animation Queue System

The BIOS uses a ring buffer at $6D80-$6D9F (32 entries) to
queue sound/animation commands for the Z80. The queue state is
stored at $6DA0-$6DA4:

| Address | Description |
|---------|-------------|
| $6DA0 | Queue head index (next to dequeue) |
| $6DA1 | Queue tail index (next to enqueue) |
| $6DA2 | Control flags (see below) |
| $6DA3 | Channel enable mask |
| $6DA4 | Expected Z80 ack value |

$6DA2 bit layout:

| Bit | Description |
|-----|-------------|
| 7 | Dequeue enable (set by INT5, cleared by $FF1EC6/$FF23E7) |
| 5 | Enqueue busy flag (set during enqueue, cleared by dequeue) |

**Enqueue** ($FF33D9): Writes a command byte into the ring
buffer at $6D80[tail], advances the tail index (mod 32), and
sets $6DA2 bit 5 (busy). The command byte in register A
selects the sound/animation type.

**Dequeue** ($FF344C): Called from the VBlank user handler each
frame. Requires $6DA2 bit 7 set and bit 5 clear. Also checks
($6DA4) == ($BC) (Z80 ack from previous command) and head !=
tail (queue not empty) before proceeding. Reads the command
from $6D80[head], writes it to $BC (Z80 communication byte),
computes ~command (XOR $FF) and stores in $6DA4, advances
the head index (mod 32), then writes ~command to $BA
(triggers Z80 NMI).

The Z80 receives the NMI, reads the command from $8000 ($BC),
processes the sound request, and writes an acknowledgment back
to $8000. The dequeue handler checks ($6DA4) == ($BC) each
frame; until the Z80 acks, no further commands are dequeued.

**Activation via INT5**: The dequeue system requires $6DA2
bit 7 to be set. This is done by the INT5 handler at $FF2282.
INT5 is triggered by the Z80 writing to $C000 (mapped to
Z80Bus address $C000 -> IntC SetPending(1, true)). The INT5
handler flow:

1. $FF2282: BIT 6,($6F83) - if set, jumps to $FF229B
   (indirect dispatch via $6FD0/$6FD2 for game-mode handling)
2. Falls through to $FF2288: BIT 7,($6DA2) - if already set,
   just RETI (idempotent)
3. If not set: calls $FF3350 (starts Timer 3 with prescaler),
   then SET 7,($6DA2) to enable dequeuing
4. RETI

Timer 3 setup ($FF3350): calculates TREG3 from a timing
parameter in WA ($07D0 = 2000), clears T3 from TRUN, sets
T23MOD clock source, configures flip-flop control, writes
TREG3, then starts T3 + prescaler via OR ($20),$88.

**Emulation requirement**: The Z80 sound CPU must be running
and must write to $C000 during initialization. Without Z80
execution, INT5 never fires, $6DA2 bit 7 is never set, and the
animation dequeue system never activates. The Z80 runs at half
the main CPU clock (3.072 MHz vs 6.144 MHz).

#### Setup Screen ($FF361E) Internals

The setup screen handles first-boot configuration (language,
clock) and cart-present animation. When $6F83 bit 4 is set (cart
present), it takes the animated path at $FF3743.

**Initialization** ($FF361E):

1. $FF25DF: clears tile/sprite RAM
2. $FF263C: clears OAM
3. $FF8D8A: decompresses font with A=$03 (color index)
4. $FF3556: initializes priority queue (pool at $64C0, anchors
   at $6440/$6480)
5. $FF3691: loads character map data to $8101-$8113
6. $FF364E: initializes screen state ($64E6=$00, $5C00=$00,
   $5C01=$00, checks $6F83 bit 4 for cart mode)
7. $FF75EC/$FF95D6/$FF36AE: additional display setup
8. EI 0: enables all interrupts
9. Sets function chain at $6440 to $FF36E0, jumps to $FF35E7

**Initial function chain**: $FF36E0 inserts two functions:

- $FF371B at priority $1000 (screen state machine)
- $FF36F3 as the input handler (replaces $6440 function)

**Cart-present path** ($FF3743): When $6F83 bit 4 is set,
$FF371B jumps to $FF3743 which:

1. Enqueues animation command 7 via $FF33D9
2. Inserts $FF3984 (scroll adjust) at priority $1500
3. Inserts $FF39CD (sprite/tile setup) at priority $2000
4. Calls $FF38D8 to configure sprite parameters from screen
   index lookup tables at $FF3928/$FF3930/$FF393C
5. Sets (XIZ+48)=$FF (busy flag)
6. Sets chain function to $FF3773 (state machine)

Typical chain at this point:

    $6440: $FF36F3 (input)     pri=$0000
    $6000: $FF3773 (state)     pri=$1000
    $6040: $FF3984 (scroll)    pri=$1500
    $6080: $FF39CD (sprites)   pri=$2000
    $60C0: $FF404E (tiles)     pri=$4000
    $6480: $FF35D6 (idle/end)  pri=$FFFF

**State machine** ($FF3773): The core screen progression logic.
Each frame:

1. Checks $6F83 bit 4: if clear, takes the no-cart path
2. Checks $5C0F bit 5 (B button release): if set, jumps to
   $FF386A (exit/skip path)
3. Checks (XIZ+48): if non-zero (busy), returns immediately
4. Reads (XIZ+60) for current state, branches:
   - State $FF: sets frame counter (XIZ+14)=$1E, changes
     function to $FF37C3
   - State 0: same as $FF, sets (XIZ+14)=$1E
   - State 1 ($FF37FC): reads $5C12 (selection value) into
     $5C01, then falls through to $FF3804 which increments
     $5C00 and calls $FF38D8 to configure next screen
   - State 2 ($FF3804): increments $5C00, calls $FF38D8
   - State 3 ($FF3813): checks $5C12. If $5C12 != 0,
     decrements $5C00 (go back) and calls $FF38D1. If
     $5C12 == 0, falls through to state 2 handler ($FF3804)
     which increments $5C00 (go forward)
   - Other states: sets (XIZ+14)=$1E frame counter, checks
     $5C00 for specific values, sets function accordingly

$5C00 tracks the screen page index. States cycle as the user
navigates menus. The state at (XIZ+60) is set by the transition
functions ($FF38D8/$FF38D1) and the frame counting logic. With
cart present and auto-input, the observed $5C00 progression is:
$01 -> $02 -> $03 -> $04 -> $05 -> $06.

**Input wait functions**: Different menu pages use different
input wait functions at node $6080:

- $FF3B6C: first input wait (states 1/3 initial). Checks
  $5C0F bit 4 (A button release). Calls $FF3D4E for cursor
  navigation. On detection, transitions through $FF3BD1
- $FF3BFB: later input wait (states after $5C00 >= 3). Also
  checks $5C0F bit 4. Calls $FF3D87 for navigation. On
  detection, queues command $22 via $FF33D9 and writes $5C13
  from (XIY+44)

The $5C12 variable holds the selection result from the input
wait. It is NOT set by $FF3B6C/$FF3BFB directly but by the
cursor navigation functions they call. If the user presses A
without navigating, $5C12 remains 0.

**Exit path** ($FF386A): When the user presses B ($5C0F bit 5)
or the state machine decides to exit:

1. Queues command $0E (or $01 if $6F83 bit 4 set) via $FF33D9
2. Sets $6C5E=$02 and waits for it to become 0
3. Sets $64E5=$FF (completion flag) and (XIZ+10)=$FF

**Frame counting**: After the state machine processes a state
transition, it enters a countdown phase:

1. $FF37C3: decrements (XIZ+14) each frame. When it reaches 0,
   sets (XIZ+14)=$D2 and changes to $FF37E1
2. $FF37E1: decrements (XIZ+14) each frame. When <= 0, calls
   $FF3804 which increments $5C00 and the state at (XIZ+60)

Total frames per state: $1E (30) + $D2 (210) = 240 frames
(~4 seconds at 60 fps).

**Busy flag clearing**: The (XIZ+48) busy flag at the state
machine node is cleared by the sprite setup function. When the
sprite/tile animation completes, $FF3A53 (or its successors)
checks the state machine's state via (XIZ+12) cross-reference:

    $FF3A84: LD.L XIY,0
    $FF3A86: LD.W IY,(XIZ+12)      ; load state machine node ptr
    $FF3A89: CP.B (XIY+60),$01     ; check state
    $FF3A8D: JRL Z,$FF3B4E         ; state 1 -> skip clear
    $FF3A90: CP.B (XIY+60),$03     ; check state
    $FF3A94: JRL Z,$FF3B4E         ; state 3 -> skip clear
    $FF3A97: LD.L XIY,0
    $FF3A99: LD.W IY,(XIZ+12)
    $FF3A9C: LD.B (XIY+48),$00     ; CLEAR busy flag

For states 1 and 3, the clear is skipped and $FF3B4E is taken
instead. This path changes the sprite function to $FF3B6C which
waits for user input ($5C0F bit 4) before proceeding. This is
the interactive selection screen where the user must press a
button.

**User input requirement**: At states 1 and 3, the setup
screen waits indefinitely for user input. The input wait
function ($FF3B6C or $FF3BFB) checks $5C0F bit 4 each frame.
If not set (no A button release), it returns immediately.
Without simulated input, the screen never advances past these
states. The setup screen has multiple pages, each with its own
input wait. With cart present, at least 4 A-button presses are
needed to advance through $5C00 states 01 through 06.

**Input detection chain**: Hardware button -> $B0 register ->
$FF26FC edge detection -> $6F82 (1-frame delayed) -> $FF36F3
second edge detection -> $5C0E/$5C0F (newly released flags,
active-high). $5C0F bit 4 = A button, bit 5 = B button. The
value is consumed (cleared) within the same frame it appears.

**Screen completion**: When all states have been processed, the
state machine eventually sets $64E5=$FF, which causes the main
loop at $FF35E7 to return, completing the setup screen.

#### ROM Interrupt Vector Table

The ROM vector table at $FFFF00-$FFFF80 maps hardware
interrupts to BIOS handler addresses. Each entry is a 4-byte
little-endian pointer. Vector index = (address - $FFFF00) / 4.

| Index | Address | Vector | Handler | Source |
|-------|---------|--------|---------|--------|
| 0 | $FFFF00 | Reset / SWI 0 | $FF204A | CPU reset |
| 1 | $FFFF04 | SWI 1 | $FF2772 | System call dispatch |
| 2 | $FFFF08 | SWI 2 | $FF2305 | Software interrupt |
| 3 | $FFFF0C | SWI 3 | $FF2202 | Software interrupt |
| 4 | $FFFF10 | SWI 4 | $FF220B | Software interrupt |
| 5 | $FFFF14 | SWI 5 | $FF2214 | Software interrupt |
| 6 | $FFFF18 | SWI 6 | $FF221D | Software interrupt |
| 7 | $FFFF1C | SWI 7 | $FF2226 | Software interrupt |
| 8 | $FFFF20 | NMI | $FF1898 | Power button |
| 9 | $FFFF24 | WDT | $FF2D98 | Watchdog |
| 10 | $FFFF28 | INT0 | $FF2856 | External (battery) |
| 11 | $FFFF2C | INT4 | $FF2163 | VBlank |
| 12 | $FFFF30 | INT5 | $FF2282 | Z80 communication |
| 13 | $FFFF34 | INT6 | $FF2B25 | External |
| 14 | $FFFF38 | INT7 | $FF2DB6 | External |
| 15 | $FFFF3C | (rsv) | $FF22A4 | RETI (reserved) |
| 16 | $FFFF40 | INTT0 | $FF22A5 | Timer 0 |
| 17 | $FFFF44 | INTT1 | $FF22AE | Timer 1 |
| 18 | $FFFF48 | INTT2 | $FF22B7 | Timer 2 |
| 19 | $FFFF4C | INTT3 | $FF22C0 | Timer 3 |
| 20 | $FFFF50 | INTTR4 | $FF22C9 | RETI (Timer 4) |
| 21 | $FFFF54 | INTTR5 | $FF22CA | RETI (Timer 5) |
| 22 | $FFFF58 | INTTR6 | $FF22CB | RETI (Timer 6) |
| 23 | $FFFF5C | INTTR7 | $FF22CC | RETI (Timer 7) |
| 24 | $FFFF60 | INTRX0 | $FF22CD | Serial RX0 |
| 25 | $FFFF64 | INTTX0 | $FF22D6 | Serial TX0 |
| 26 | $FFFF68 | INTRX1 | $FF22DF | Serial RX1 |
| 27 | $FFFF6C | INTTX1 | $FF22E0 | RETI (Serial TX1) |
| 28 | $FFFF70 | INTAD | $FF2DCE | A/D converter |
| 29 | $FFFF74 | INTTC0 | $FF22E1 | RETI (DMA TC0) |

Timer interrupt handlers (INTT0-INTT3) use indirect dispatch
through the RAM vector table:

- INTT0 ($FF22A5): PUSH ($6FD6) / PUSH ($6FD4) / RET
- INTT1 ($FF22AE): PUSH ($6FDA) / PUSH ($6FD8) / RET
- INTT2 ($FF22B7): PUSH ($6FDE) / PUSH ($6FDC) / RET
- INTT3 ($FF22C0): PUSH ($6FE2) / PUSH ($6FE0) / RET

The PUSH/PUSH/RET sequence pushes the 32-bit handler address
from the RAM vector table entry onto the stack, then RET pops
it as the return address, effectively jumping to the handler.
The target handler must end with RETI to return from the
interrupt. By default all RAM vector entries point to $FF23DF
(a bare RETI instruction).

#### Handler Disassembly Reference

Complete disassembly and behavior documentation for every ROM vector
table handler. Handlers fall into four categories:

1. **PUSH/RET dispatch**: pushes user handler address from RAM table,
   RETs to it. User handler must RETI. This is the standard pattern.
2. **Complex handlers**: perform housekeeping before dispatching.
3. **Bare RETI**: no user dispatch, just returns from interrupt.
4. **Special**: reset, NMI, SWI handlers with unique behavior.

##### SWI 1 - System Call Dispatch ($FF2772)

```
DI                          ; disable interrupts
LDF 3                       ; switch to register bank 3
PUSH XHL                    ; save XHL
ADD.B W,W                   ; W *= 2
ADD.B W,W                   ; W *= 2 (total: W *= 4)
LD XHL,$00FFFE00            ; syscall jump table base
LD.L XHL,(XHL+W)            ; load handler address from table
LD.L ($6C49),XHL            ; store handler address
POP XHL                     ; restore XHL
PUSH.W ($FF27A0)            ; push high word of $FF279D
PUSH.W ($FF279E)            ; push low word of $FF279D
PUSH.W ($6C4B)              ; push high word of handler
PUSH.W ($6C49)              ; push low word of handler
RET                         ; jump to handler via stack
; handler returns here:
$FF279D: RETI               ; return from SWI
```

Uses the jump table at $FFFE00 to look up the syscall handler by
index W (bank 3). The PUSH/PUSH/PUSH/PUSH/RET sequence sets up a
two-level return: the handler RETs to $FF279D, which RETIs back to
the caller.

##### SWI 2 - Cold Reset ($FF2305)

Performs a full system reset sequence: disables interrupts, sets
watchdog, ramps clock gear from 3 to 0, then re-initializes the
system. This is a destructive operation - it does not return.

##### SWI 3 ($FF2202) - PUSH/RET dispatch via $6FB8

```
PUSH.W ($6FBA)
PUSH.W ($6FB8)
RET
```

##### SWI 4 ($FF220B) - PUSH/RET dispatch via $6FBC

```
PUSH.W ($6FBE)
PUSH.W ($6FBC)
RET
```

##### SWI 5 ($FF2214) - PUSH/RET dispatch via $6FC0

```
PUSH.W ($6FC2)
PUSH.W ($6FC0)
RET
```

##### SWI 6 ($FF221D) - PUSH/RET dispatch via $6FC4

```
PUSH.W ($6FC6)
PUSH.W ($6FC4)
RET
```

##### SWI 7 ($FF2226) - Stack Overflow Detection + Cart Resume

```
DI
CP.L XSP,$009F0000
JR C,$FF2238
CP.L XSP,$00A00000
JR C,$FF2248
CP.L XSP,$00006C01
JR NC,$FF2264
CP.L XSP,$00004800
JR C,$FF2264
CALR $FF3097              ; flash-related
...
PUSH.W ($3FFF1E)           ; push cart resume vector (high)
PUSH.W ($3FFF1C)           ; push cart resume vector (low)
RET                        ; jump to cart resume handler
```

Validates SP is in a valid range, then dispatches to the cart's
resume handler at $3FFF1C if conditions are met. If SP is invalid,
falls through to a reset at $FF2264.

##### NMI ($FF1898) - Power Button

```
RES 2,($B3)                ; clear NMI enable bit
BIT 4,($6E)                ; check watchdog mode
JRL Z,$FF204A              ; if not set, jump to reset
...
```

Triggered by the power button. Checks battery voltage, handles
power-down sequencing, may jump to reset vector. Contains delay
loops (NOP sequences) for hardware timing.

##### Watchdog ($FF2D98) - System Reset

```
DI
LD XSP,$00006C00           ; reset stack
LD.B ($6F84),$00
LD.B ($6F85),$00
LD.B ($6F86),$00
AND.B ($6F83),$10          ; preserve only bit 4
JRL T,$FF1074              ; jump to boot sequence
```

Resets system state and restarts the boot sequence. Clears user
boot/shutdown/answer flags while preserving bit 4 of $6F83.

##### INT0 ($FF2856) - RTC/Battery

```
CP.L XSP,$00006C01         ; validate SP range
JR NC,$FF284D
CP.L XSP,$00004800
JR C,$FF284D
PUSH WA
CP.W ($6C7C),$5AA5         ; check boot completion marker
JP NZ,($FF2898)            ; if boot not complete, different path
CP.B ($6C47),$02           ; check state
JR Z,$FF2893
BIT 6,($6F83)              ; check if cart operations enabled
JR Z,$FF2893
CP.B ($6C46),$FF           ; check cart-present flag
JR Z,$FF2893
CALR $FF2AC5               ; cart validation/state management
```

Validates stack, checks boot completion marker, performs cart
state management via $FF2AC5 if conditions are met. The $FF2AC5
subroutine handles flash write monitoring and cart persistence.

##### INT4 ($FF2163) - VBlank

See [VBlank Handler ($FF2163)](#vblank-handler-ff2163) for the
full 10-step sequence. This is the most complex handler.

Summary: input scanning, sound DMA, auto power-off timer, LED
control, battery monitoring, sleep timer, then PUSH/RET dispatch
to user handler at $6FCC.

##### INT5 ($FF2282) - Z80 Communication

```
BIT 6,($6F83)              ; check if cart operations enabled
JR NZ,$FF229B              ; if set, dispatch to user handler
BIT 7,($6DA2)              ; check sound enabled flag
JR NZ,$FF229A              ; if already enabled, just RETI
PUSH WA
LD WA,$07D0
CALR $FF3350               ; sound init with timeout
SET 7,($6DA2)              ; mark sound as enabled
POP WA
RETI
; user dispatch path:
$FF229B:
PUSH.W ($6FD2)             ; push high word of $6FD0
PUSH.W ($6FD0)             ; push low word of $6FD0
RET                        ; dispatch to user handler
```

Two modes: during BIOS boot ($6F83 bit 6 clear), performs one-time
sound initialization and sets $6DA2 bit 7. After cart handoff
($6F83 bit 6 set), dispatches to user handler at $6FD0 via
PUSH/RET.

##### INT6 ($FF2B25) - Bare RETI

```
RETI
```

No user dispatch. Returns immediately.

##### INT7 ($FF2DB6) - Interrupt Priority Update

```
PUSH BC
LD.B B,($6C26)
AND.B B,$0F
LD.B ($72),B               ; write to INTET23 register
LD.B ($6C26),B
LD B,$00
LD C,$00
CALR $FF1034               ; timer/peripheral init
POP BC
RETI
```

Updates the INTET23 interrupt priority register from $6C26, calls
a peripheral initialization routine, returns. No user dispatch.

##### INTWD ($FF22A4) - Watchdog Timer Overflow - Bare RETI

```
RETI
```

##### INTT0 ($FF22A5) - PUSH/RET dispatch via $6FD4

```
PUSH.W ($6FD6)
PUSH.W ($6FD4)
RET
```

##### INTT1 ($FF22AE) - PUSH/RET dispatch via $6FD8

```
PUSH.W ($6FDA)
PUSH.W ($6FD8)
RET
```

##### INTT2 ($FF22B7) - PUSH/RET dispatch via $6FDC

```
PUSH.W ($6FDE)
PUSH.W ($6FDC)
RET
```

##### INTT3 ($FF22C0) - PUSH/RET dispatch via $6FE0

```
PUSH.W ($6FE2)
PUSH.W ($6FE0)
RET
```

##### INTTR4 ($FF22C9), INTTR5 ($FF22CA), INTT6 ($FF22CB), INTT7 ($FF22CC) - Bare RETI

```
RETI
```

These timer sources have no user dispatch.

##### INTRX0 ($FF22CD) - Serial RX - PUSH/RET dispatch via $6FE4

```
PUSH.W ($6FE6)
PUSH.W ($6FE4)
RET
```

Note: the RAM table mapping has serial RX at $6FE4, not $6FE8.

##### INTTX0 ($FF22D6) - Serial TX - PUSH/RET dispatch via $6FE8

```
PUSH.W ($6FEA)
PUSH.W ($6FE8)
RET
```

##### INTRX1 ($FF22DF), INTTX1 ($FF22E0) - Bare RETI

```
RETI
```

Serial channel 1 has no user dispatch.

##### INTAD ($FF2DCE) - A/D Converter

```
PUSH SR
LDF 3                       ; switch to bank 3
PUSH WA
AND.B ($6C24),$0F           ; mask priority bits
LD.B ($70),($6C24)          ; restore INTE0AD register
LD.W WA,($60)               ; read A/D result from $60
SRL.W 6,WA                  ; shift right 6 bits
LD.W ($6F80),WA             ; store battery voltage
LD.W ($6C1A),$0258          ; reset A/D interval
LD.W ($6C18),$0000          ; reset A/D counter
CP.W WA,$0200               ; check if voltage too low
...                          ; low battery handling
```

Reads the A/D conversion result, stores battery voltage at $6F80,
resets the conversion timer. If voltage is below threshold, may
trigger shutdown. No user dispatch - this is BIOS internal.

##### INTTC0 ($FF22E1) - PUSH/RET dispatch via $6FF0

```
PUSH.W ($6FF2)
PUSH.W ($6FF0)
RET
```

##### INTTC1 ($FF22EA) - PUSH/RET dispatch via $6FF4

```
PUSH.W ($6FF6)
PUSH.W ($6FF4)
RET
```

##### INTTC2 ($FF22F3) - PUSH/RET dispatch via $6FF8

```
PUSH.W ($6FFA)
PUSH.W ($6FF8)
RET
```

##### INTTC3 ($FF22FC) - PUSH/RET dispatch via $6FFC

```
PUSH.W ($6FFE)
PUSH.W ($6FFC)
RET
```

##### INT4 VBlank Subroutines

The VBlank handler calls several subroutines. Their addresses and
roles:

| Address | Name | Purpose |
|---------|------|---------|
| $FF26FC | Input scan | Read $B0, edge-detect with $6C5F, store $6F82 |
| $FF344C | Sound DMA | Service Z80 sound ring buffer at $6D80 |
| $FF2B26 | LED monitor | Edge-detect $B1 bit 1, blink LED on overflow |
| $FF2B5F | LED restore | Restore LED when power button released |
| $FF2E5D | ADC trigger | Start battery A/D conversion |
| $FF2B7D | Sleep timer | Monitor $6C20 for sleep timeout |

#### Syscall Jump Table Handler Disassembly ($FFFE00)

The syscall jump table at $FFFE00 contains 27 valid entries (indices
0-26). Each entry is a 4-byte address of the handler function. These
are called via SWI 1 with the index in RW3 (register bank 3).

##### VECT_SHUTDOWN ($00) - $FF27A2

```
DI                          ; disable interrupts
LD.B ($80),$03              ; set clock gear 3
LD.B ($6C2C),$00            ; clear priority shadow
LD.B ($80),$02              ; set clock gear 2
LD.B ($6C2C),$00
LD.B ($80),$01              ; set clock gear 1
LD.B ($6C2C),$00
LD.B ($80),$00              ; set clock gear 0 (full speed)
LD.B ($6C2C),$00
LD.B ($6C55),$00            ; clear system flag
RES 6,($6F83)              ; clear game-running bit
CALR $FF3384                ; clear K2GE state
CALR $FF25AC                ; disable sound/Z80
SET 3,($6F83)               ; set shutdown-in-progress
LD.B ($70),$0A              ; re-init interrupt priorities
LD.B ($71),$DC
LD.B ($72),$00
LD.B ($73),$00
LD.B ($74),$00
LD.B ($77),$00
LD.B ($79),$00
LD.B ($7A),$00
LD.B ($6F84),$00            ; clear power state
AND.B ($6F85),$20           ; keep only bit 5 of power flags
LD A,$06
CALR $FF239D                ; setup for re-entry
LD.B ($6E8B),$04
EI 0
CALR $FF1F76                ; display fade step
DI
LD.B ($6E8B),$07
EI 0
CALR $FF1F76                ; display fade step
DI
LD.B ($6E8B),$06
EI 0
CALR $FF1F76                ; display fade step
DI
CALR $FF23E0                ; disable Z80
CALR $FF5001                ; enter BIOS menu loop
```

Ramps clock gear from 3 down to 0 (clearing priority shadows at each step),
disables the game, re-initializes interrupt priorities, performs a display
fade sequence, then enters the BIOS menu/shutdown loop. Does not return.

##### VECT_CLOCKGEARSET ($01) - $FF1034

```
PUSH.B L
LD.B L,($6C26)              ; load ADC config shadow
AND.B L,$0F                 ; keep low nibble
LD.B ($72),L                ; write to INT priority reg $72
AND.B B,$07                 ; clamp gear to 0-7
CP.B B,4
JR ULE,$FF1049              ; if > 4, clamp to 4
LD B,$04
LD.B ($80),B                ; write gear to clock gear register
NOP x6                      ; delay for clock stabilization
LD.B ($6F88),B              ; store gear in system state
CP.B B,0                    ; if gear == 0, skip INTAD setup
JR Z,$FF1071
CP.B C,0                    ; if C == 0, skip INTAD setup
JR Z,$FF1071
OR.B ($6F88),$C0            ; set INTAD auto-regen bits
LD.B L,($6C26)
OR.B L,$20                  ; set bit 5 in ADC config
LD.B ($72),L                ; write back to priority reg
LD.B ($6C26),L              ; update shadow
POP.B L
RET
```

Writes the gear value (clamped to 0-4) to register $80 with NOP delay.
Stores gear in $6F88. If gear != 0 and C != 0, enables INTAD auto-
regeneration by setting $6F88 bits 7-6 and $72 bit 5.

##### VECT_RTCGET ($02) - $FF1440

```
PUSH WA
PUSH BC
LD.B C,($96)               ; read seconds (for consistency check)
LD.B A,($91)               ; read year
CP.B ($91),A               ; verify stable (re-read)
JR Z,$FF1450
LD.B A,($91)               ; re-read on mismatch
LD.B (XHL),A               ; store year
LD.W WA,($92)              ; read month+day
CP.W ($92),WA              ; verify stable
JR Z,$FF145D
LD.W WA,($92)              ; re-read on mismatch
LD.W (XHL+1),WA            ; store month+day
LD.W WA,($94)              ; read hour+minute
CP.W ($94),WA
JR Z,$FF146B
LD.W WA,($94)
LD.W (XHL+3),WA            ; store hour+minute
LD.W WA,($96)              ; read second+weekday
CP.W ($96),WA
JR Z,$FF1479
LD.W WA,($96)
LD.W (XHL+5),WA            ; store second+weekday
CP.B A,C                   ; compare current seconds to initial
JR Z,$FF1484                ; if same, data is consistent
CP.B A,0                   ; if wrapped to 0, re-read all
JR Z,$FF1442
POP BC
POP WA
RET
```

Reads RTC registers $91-$96 (year, month, day, hour, minute, second)
into the 7-byte buffer at XHL. Each register pair is read, compared,
and re-read on mismatch for consistency. If seconds changed during the
read, the entire sequence restarts to avoid torn reads.

##### VECT_RTC_ALARM_SET ($03) - $FF12B4

```
PUSH.B W
PUSH XHL
PUSH XIX
PUSH XIY
LD.L XWA,0
LDA XIX,($6F00)            ; alarm data area
LD.L (XIX),XWA             ; clear alarm data
LD.L XIY,XHL               ; source pointer
; validate hours (0-23), minutes (0-59), seconds (0-59)
; validate year (0-99), month (1-12)
; calculate alarm ticks, store to (XIX+)
...
```

Validates time fields from the buffer at XHL against BCD range limits.
Stores validated alarm parameters to the alarm data area at $6F00.
Computes alarm trigger time in ticks. Returns error via jump to $FF137D
if any field is out of range.

##### VECT_INTLVSET ($04) - $FF1222

```
PUSH XIX
PUSH XIY
PUSH XIZ
PUSH.B D
CP.B C,0                   ; if source != 0
JR Z,$FF122D
INC 2,C                    ;   C += 2 (adjust index)
LD.B D,B                   ; D = priority level
AND.B D,$07                ; mask to 3 bits
AND.B B,$80                ; keep bit 7 of B
CP.B D,5                   ; clamp priority to max 5
JR C,$FF123B
LD D,$05
OR.B B,D                   ; combine flags
CALR $FF1268               ; adjust B based on odd/even source
AND.B C,$FE                ; clear bit 0
ADD.B C,C                  ; C *= 4
ADD.B C,C
LD XIX,$00FF1284            ; register lookup table
LD.L XIY,(XIX+C)            ; load SFR target address
INC 4,C
LD.L XIZ,(XIX+C)            ; load shadow register address
LD.B A,(XIZ)               ; read current shadow value
AND.B A,D                  ; mask target nibble
OR.B A,B                   ; insert new priority
LD.B (XIY),A               ; write to SFR
LD.B (XIZ),A               ; write to shadow
POP.B D
POP XIZ
POP XIY
POP XIX
RET
```

Read-modify-write on interrupt priority registers. Uses a lookup table
at $FF1284 that maps source index to {SFR address, shadow address}.
The helper at $FF1268 adjusts the mask based on odd/even source and
bit 7 flag. Priority is clamped to 0-5.

##### VECT_SYSFONTSET ($05) - $FF8D8A

```
PUSH XBC
PUSH XIX
PUSH XIY
LD.W QBC,$0800             ; 2048 bytes (256 chars x 8 lines)
LD XIX,$0000A000            ; K2GE character RAM dest
LD XIY,$00FF8DCF            ; 1bpp font data in BIOS ROM
; -- entry point for alternate color call --
PUSH XWA
LD.B B,A                   ; B = background color (bits 4-5)
SRL.B 4,B                  ; shift to low nibble
AND.B A,$03                ; A = font color (bits 0-1)
loop:
LD.B C,(XIY+)              ; read 1bpp source byte
LD.B QA,$08                ; 8 bits per byte
bit_loop:
SLL.W (XIX)                ; shift dest word left
SLL.W (XIX)                ; (make room for 2 bits)
SLL.B 1,C                  ; shift source bit into carry
JR NC,$FF8DBE              ; if bit clear: use background
OR.B (XIX),A               ; bit set: write font color
JR T,$FF8DC0
OR.B (XIX),B               ; bit clear: write bg color
DJNZ.B QA,bit_loop
INC 2,XIX                  ; advance dest by 2 bytes
DJNZ.W QBC,loop
POP XWA
POP XIY
POP XIX
POP XBC
RET
```

Decompresses 1bpp font data (256 chars x 8 rows = 2048 bytes) from
BIOS ROM at $FF8DCF into 2bpp character data at K2GE RAM $A000. Each
source bit selects either the font color (RA3 bits 0-1) or background
color (RA3 bits 4-5) as the 2-bit output pixel.

##### VECT_FLASHWRITE ($06) - $FF6FD8

```
PUSH SR
DI
PUSH XIX
PUSH.W QBC
PUSH XIY
CALR $FF755D               ; select flash chip (CS0/CS1)
CALR $FF7367                ; read flash chip ID, verify
CP.B A,$FF
JR Z,$FF7013               ; skip if no flash
CALR $FF751D               ; enter flash command mode
LD.W QBC,0
SLA.L 7,XBC                ; large timeout counter
LD A,$00
CALR $FF701D               ; write byte, verify
CP.B A,$FF
JR Z,$FF7010               ; error
CALR $FF701D               ; write next byte, verify
CP.B A,$FF
JR Z,$FF7010               ; error
DEC 1,XBC                  ; decrement timeout
CP.L XBC,$00000000
JR NZ,$FF6FF6              ; loop until done or timeout
CALR $FF751D               ; exit flash command mode
CALR $FF7543               ; deselect flash chip
POP XIY
POP.W QBC
POP XIX
POP SR
RET
```

Writes pages to flash memory. Selects flash chip, verifies chip ID,
enters command mode, then writes bytes in a loop with verification.
Times out on write failure. Runs with interrupts disabled.

##### VECT_FLASHALLERS ($07) - $FF7042

```
PUSH SR
DI
PUSH XIX
PUSH XDE
PUSH XIY
CALR $FF755D               ; select flash chip
CALR $FF7367                ; verify chip ID
CP.B A,$FF
JR Z,$FF707A               ; skip if no flash
CALR $FF751D               ; enter command mode
CALR $FF74D9               ; send erase-all command
LD A,$00
LD.L XIY,0
loop:
CP.B (XIX),$FF             ; poll until erased
JR Z,$FF7077               ; done
INC 1,XIY
CP.L XIY,$01555555         ; timeout
JR NC,$FF7075              ; timeout -> error
BIT 5,(XIX)                ; check toggle bit
JR Z,$FF705D               ; not ready, keep polling
CP.B (XIX),$FF             ; double-check
JR Z,$FF7077               ; done
LD A,$FF                   ; error
CALR $FF751D               ; exit command mode
CALR $FF7543               ; deselect flash
...
RET
```

Erases all flash blocks. Sends the erase-all command sequence, then
polls the flash chip for completion (checking for $FF in erased cells).
Uses toggle bit 5 for status and a large timeout counter. Returns A=$00
on success, A=$FF on error/timeout.

##### VECT_FLASHERS ($08) - $FF7082

```
PUSH SR
DI
PUSH XIX
PUSH XDE
PUSH XIY
CALR $FF755D               ; select flash chip
LD.B E,A                   ; save block number
CALR $FF7367                ; verify chip ID
CALR $FF738A               ; compute block base address
CP.B A,$FF
JR Z,$FF70C2               ; skip if error
CALR $FF751D               ; enter command mode
LD A,$00
LD.L XIY,0
CALR $FF74FE               ; send sector erase command
LD.B (XDE),$30             ; confirm erase
loop:
CP.B (XDE),$FF             ; poll until erased
JR Z,$FF70BF
INC 1,XIY
CP.L XIY,$001FFFFF         ; timeout
JR NC,$FF70BD
BIT 5,(XDE)                ; toggle bit
JR Z,$FF70A5
CP.B (XDE),$FF
JR Z,$FF70BF
LD A,$FF                   ; error
CALR $FF751D               ; exit command mode
CALR $FF7543               ; deselect
...
RET
```

Erases one flash block specified by RA3. Computes the block base address
via $FF738A, sends the sector erase command followed by $30 confirm,
then polls for completion with toggle bit 5 and timeout. Returns A=$00
on success, A=$FF on error.

##### VECT_ALARMSET ($09) - $FF149B

```
PUSH XHL
PUSH.B D
LD H,$00
LD A,$FF
LD.B ($6C46),$00            ; clear cart flag
CALR $FF1724                ; compute alarm parameters
CALR $FF4A83                ; validate parameters
JR T,$FF14B4                ; jump to shared alarm setup
```

Falls into shared alarm setup code at $FF14B4. Validates alarm time
parameters (hours 0-23, minutes 0-59, seconds 0-59, day-of-week 0-31).
Sets $6C46 to $00 (disables cart detection during alarm). Uses the same
validation helper ($FF143A) as VECT_RTC_ALARM_SET.

##### Stub ($0A) - $FF1033

```
RET
```

Bare return - no operation.

##### VECT_ALARMDOWNSET ($0B) - $FF1487

```
PUSH XHL
PUSH.B D
LD H,$00
LD A,$FF
LD.B ($6C46),$FF            ; set cart flag to $FF
CALR $FF1724                ; compute alarm parameters
CALR $FF4A83                ; validate
JR T,$FF14B4                ; jump to shared alarm setup
```

Same as VECT_ALARMSET but sets $6C46 to $FF instead of $00. Falls
into the same shared alarm validation and setup path at $FF14B4.

##### VECT_FLASHPROTECT_CHECK ($0C) - $FF731F

```
PUSH SR
DI
PUSH XIX
PUSH XDE
PUSH.B C
CALR $FF755D               ; select flash chip
LD.B E,A                   ; save parameter
CALR $FF7367                ; verify chip ID
CP.B A,$FF
JR Z,$FF7353               ; skip if no flash
CALR $FF738A               ; compute block address
CALR $FF751D               ; enter command mode
LD A,$00
CALR $FF7569               ; send protection check command
LD.B (XDE),$FF             ; write $FF
CALR $FF735C               ; delay ($0800 iterations)
CALR $FF7530               ; read status
LD.B C,(XDE+2)             ; read protection byte
CP.B C,1                   ; check if protected
JR Z,$FF7350               ; protected: A stays 0
LD A,$FF                   ; not protected: A = $FF
CALR $FF751D               ; exit command mode
CALR $FF7543               ; deselect flash
POP.B C
POP XDE
POP XIX
POP SR
RET
```

Checks flash block protection status. Reads the protection byte at
offset +2 from the block base address. Returns A=$00 if the block is
protected (byte == 1), A=$FF if not protected.

##### VECT_FLASHPROTECT ($0D) - $FF70CA

```
PUSH SR
DI
PUSH XIX
PUSH XHL
PUSH XDE
PUSH XBC
CALR $FF755D               ; select flash chip
PUSH DE
PUSH BC
LD XDE,$00006000            ; RAM buffer destination
LD XHL,$00FF70F3            ; protection code in ROM
LD BC,$0300                 ; 768 bytes
LDIR.B (XHL)               ; copy code to RAM
POP BC
POP DE
CALL $6000                  ; execute from RAM
CALR $FF7543               ; deselect flash
POP XBC
POP XDE
POP XHL
POP XIX
POP SR
RET
```

Copies 768 bytes of flash protection code from BIOS ROM ($FF70F3) to
work RAM at $6000, then executes it from RAM. The flash protection
command sequence must run from RAM because the flash chip bus is in
command mode and cannot simultaneously serve code fetches. The copied
code handles chip select, command sequences, and verification.

##### VECT_GEMODESET ($0E) - $FF17C4

```
CP.B A,$10
JR NC,$FF17DE               ; if A >= $10, use mode 1
; Mode 0 (A < $10):
LD.B ($87F0),$AA            ; K2GE unlock
LD.B ($87E2),$80            ; set K2GE mode to $80
LD.B ($6F95),$00            ; store mode 0 flag
LD.B ($87F0),$55            ; K2GE lock
RET
; Mode 1 (A >= $10):
LD.B ($87F0),$AA            ; K2GE unlock
LD.B ($87E2),$00            ; set K2GE mode to $00
LD.B ($6F95),$10            ; store mode 1 flag
LD.B ($87F0),$55            ; K2GE lock
RET
```

Sets K2GE display mode. Uses $87F0 unlock/lock sequence ($AA/$55)
around writes to $87E2. If RA3 < $10, sets K2GE mode to $80 (color
mode). If RA3 >= $10, sets K2GE mode to $00 (monochrome mode). Stores
the mode flag in $6F95.

##### Stub ($0F) - $FF1032

```
RET
```

Bare return - no operation.

##### VECT_COMINIT ($10) - $FF2BBD

```
LD.B ($1B),$01              ; serial control reg
LD.B ($1A),$01              ; serial control reg
LD.B ($18),$3F              ; baud rate
LD.B ($52),$49              ; serial mode
LD.B ($51),$00              ; serial status clear
LD.B ($53),$05              ; serial control
OR.B ($20),$80              ; enable serial port
LDA XWA,($FF2D03)           ; TX interrupt handler address
LD.L ($6FE4),XWA            ; set INTRX0 user vector
LDA XWA,($FF2CF9)           ; RX interrupt handler address
LD.L ($6FE8),XWA            ; set INTTX0 user vector
LD.B ($77),$EE              ; serial interrupt priorities
LD.B ($6D02),$00            ; TX ring head
LD.B ($6D03),$00            ; RX ring head
LD.B ($6D00),$00            ; TX ring count
LD.B ($6D01),$00            ; RX ring count
LD.B ($6D04),$00            ; TX busy flag
LD.B ($6D05),$00            ; RX overflow flag
LD.B ($6D06),$00            ; serial error status
RET
```

Initializes serial communication channel 1. Configures baud rate,
mode, and control registers. Installs TX/RX interrupt handlers into
the RAM vector table at $6FE4/$6FE8. Sets serial interrupt priority
to $EE (level 6 for both RX and TX). Clears ring buffer state at
$6D00-$6D06. Ring buffers: TX at $6C80 (64 bytes), RX at $6CC0
(64 bytes).

##### VECT_COMSENDSTRING ($11) - $FF2C0C

```
CP.B ($6D04),$FF            ; check TX busy flag
JR Z,$FF2C16                ; if not busy, skip
CALR $FF2C17                ; send next byte from ring buffer
RET
```

Sends the next queued byte from the TX ring buffer if the transmitter
is busy. The helper at $FF2C17 reads from the TX ring buffer at $6C80,
writes to the serial data register $50, and manages the ring indices.

##### VECT_COMRECIVESTRING ($12) - $FF2C44

```
OR.B ($52),$20              ; enable serial receive
AND.B ($B2),$FE             ; clear RTS (enable RTS signal)
RET
```

Enables serial receive by setting bit 5 of serial control register $52
and asserting RTS by clearing bit 0 of port register $B2.

##### VECT_COMCREATEDATA ($13) - $FF2C86

```
LD.B ($77),$8E              ; mask TX interrupt
CP.B ($6D00),$3F            ; check if TX buffer full (63)
JR GT,$FF2CAE               ; buffer full -> return $FF
LDA XDE,($6C80)             ; TX ring buffer base
LD.B C,($6D02)              ; TX head index
ADD.B C,($6D00)             ; compute tail position
AND.B C,$3F                 ; wrap at 64
LD.B (XDE+C),B              ; store byte in ring buffer
INC.B 1,($6D00)             ; increment TX count
LD A,$00                    ; success
LD.B ($77),$EE              ; unmask TX interrupt
RET
; buffer full:
LD A,$FF
LD.B ($77),$EE
RET
```

Adds byte RB3 to the TX ring buffer. Returns A=$00 on success, A=$FF
if buffer full (count >= 63). Temporarily masks TX interrupt during
buffer manipulation by writing $8E to $77 (priority reg), then
restores to $EE.

##### VECT_COMGETDATA ($14) - $FF2CB4

```
OR.B ($B2),$01              ; disable RTS
LD.B ($77),$88              ; mask both serial interrupts
CP.B ($6D01),$37            ; if RX count >= 55
JR GE,$FF2CC6               ;   don't clear RTS
AND.B ($B2),$FE             ;   re-enable RTS
CP.B ($6D01),$00            ; if RX count == 0
JR Z,$FF2CEF                ;   return A=1 (empty)
LDA XDE,($6CC0)             ; RX ring buffer base
LD.B C,($6D03)              ; RX head index
INC.B 1,($6D03)             ; advance head
DEC.B 1,($6D01)             ; decrement count
AND.B C,$3F                 ; wrap at 64
LD.B B,(XDE+C)              ; read byte into B
LD A,$00                    ; success
LD.B ($77),$EE              ; unmask interrupts
AND.B ($B2),$FE             ; re-enable RTS
RET
; empty:
LD A,$01
LD.B ($77),$EE
AND.B ($B2),$FE
RET
```

Gets one byte from the RX ring buffer into RB3. Returns A=$00 on
success, A=$01 if buffer empty. Manages RTS flow control: disables
RTS during read, re-enables if buffer has space (count < 55).
Temporarily masks serial interrupts via $77.

##### VECT_COMONRTS ($15) - $FF2D27

```
CP.B ($6D05),$FF            ; check RX overflow flag
JR Z,$FF2D32                ; if overflow, skip
AND.B ($B2),$FE             ; enable RTS (clear bit 0)
RET
```

Enables RTS signal if the receive buffer has not overflowed.

##### VECT_COMOFFRTS ($16) - $FF2D33

```
EI 6
OR.B ($B2),$01              ; disable RTS (set bit 0)
RET
```

Disables RTS signal. Enables interrupts at level 6 first.

##### VECT_COMERROR ($17) - $FF2D3A

```
LD.B ($77),$88              ; mask serial interrupts
CP.B ($6D00),$40            ; check TX count against 64
CCF                         ; complement carry flag
STCF 0,W                   ; store carry to W bit 0
LD.B A,($6D00)              ; A = TX count
LD.B ($77),$EE              ; unmask serial interrupts
RET
```

Returns serial buffer status. A = TX buffer count. W bit 0 = carry
flag from comparing TX count to 64 (set if buffer full).

##### VECT_COMSCOPYTRDATA ($18) - $FF2D4E

```
LD.B ($77),$88              ; mask serial interrupts
LD.B W,($6D05)              ; W = RX overflow flag
AND.B W,$02                 ; keep bit 1
OR.B W,($6D06)              ; combine with error status
SRL.B 1,W                   ; shift right
LD.B ($6D06),$00            ; clear error status
LD.B A,($6D01)              ; A = RX count
LD.B ($77),$EE              ; unmask serial interrupts
RET
```

Gets serial receive status/error info. A = RX buffer count. W =
combined overflow and error flags (cleared after read).

##### VECT_COMRECEIVEDATA ($19) - $FF2D6C

```
LD.B W,B                   ; W = byte count
loop:
LD.B B,(XHL+)              ; read byte from source buffer
CALR $FF2C86               ; add to TX ring (COMCREATEDATA)
CP.B A,$FF
JR Z,$FF2D80               ; error -> stop
DEC 1,W
JR NZ,loop
LD.B B,W                   ; B = remaining (0 = all sent)
RET
error:
DEC 1,XHL                  ; back up pointer
LD.B B,W                   ; B = remaining unsent count
RET
```

Sends multiple bytes from buffer at XHL3. RB3 = count. Calls
COMCREATEDATA for each byte. On error (buffer full), stops and returns
RB3 = remaining count, XHL3 pointing to the failed byte.

##### VECT_COMSENDDATA ($1A) - $FF2D85

```
LD.B W,B                   ; W = byte count
loop:
CALR $FF2CB4               ; get byte (COMGETDATA)
CP.B A,1
JR Z,$FF2D95               ; empty -> done
LD.B (XHL+),B              ; store received byte
DEC 1,W
JR NZ,loop
LD.B B,W                   ; B = remaining (0 = all read)
RET
```

Receives multiple bytes into buffer at XHL3. RB3 = count. Calls
COMGETDATA for each byte. Stops early if RX buffer is empty (A=1).
Returns RB3 = remaining count.

#### Cartridge Validation Subroutines

The following subroutines are called during the INT0 first boot cycle
path (steps 16-21) when a cartridge is detected ($6C46 != $00). They
probe flash chip IDs, validate the license string, copy header
data to system RAM, and check whether the cartridge matches a
previous session's saved state.

##### Flash Chip ID Probing ($FF309C)

Probes both flash chip select regions to identify installed chips:

1. Sets XIX=$200000 (CS0 base), calls $FF30F8 (see below)
2. Stores results: A to $6C58 (chip type), W to $6C5A
   (protection status), copies $6C58 to $6F92
3. Sets XIX=$800000 (CS1 base), calls $FF30F8 again
4. Stores results: A to $6C59, W to $6C5B, copies $6C59
   to $6F93
5. Sets $6C7E=$00. If $6C59 != $00 (CS1 present): sets
   $6C7E=$01, then writes and reads back $55/$AA to
   $9FFFF6 (CS1 flash test)

##### Flash ID Read ($FF30F8)

Identifies a single flash chip by sending AMD command sequences:

1. Calls $FF751D - flash reset command: writes $AA to
   (XIX+$5555), $55 to (XIX+$2AAA), $F0 to (XIX+$5555)
2. Calls $FF7530 - flash enter ID mode: writes $AA to
   (XIX+$5555), $55 to (XIX+$2AAA), $90 to (XIX+$5555)
3. Reads manufacturer ID from (XIX+0). Accepted values:
   - $98 = Toshiba
   - $EC = Samsung
   - $B0 = Sharp
   - Any other value: chip not recognized, returns A=0, W=0
4. Reads (XIX+3), checks upper 5 bits == $80 (bits 7-3
   after AND $F8). If not $80: returns A=0, W=0
5. Reads device ID from (XIX+1). Maps to chip type:
   - $AB: A=$01 (4 Mbit / 512 KB)
   - $2C: A=$02 (8 Mbit / 1 MB)
   - $2F: A=$03 (16 Mbit / 2 MB)
   - Other: returns A=0, W=0
6. Reads protection status from (XIX+2) into W
7. Calls $FF751D - flash reset back to read mode
8. Returns A=chip type (1-3), W=protection byte

##### License Validation ($FF3197)

Validates the cartridge license string at $200000 against two
known strings stored in the BIOS ROM:

1. Sets W=$01 (assume failure)
2. Compares 28 bytes at $200000 against the string at $FF3234:
   `" LICENSED BY SNK CORPORATION"` (leading space)
3. If match: sets W=$00
4. Compares 28 bytes at $200000 against the string at $FF3250:
   `"COPYRIGHT BY SNK CORPORATION"`
5. If match: sets W=$00
6. Returns A=$00 if either string matched, A=$FF if neither

##### License Validation and Header Copy ($FF314C)

Validates the license string and copies cartridge header data
to system RAM:

1. Calls $FF3197 (license validation, see above)
2. If license fails (A != 0):
   - Clears bit 7 of $6F86
   - Sets $6C55=$00 (no commercial game)
   - Calls $FF31B8 (fallback header copy from BIOS ROM at
     $FFE1DA-$FFE252, likely a default/empty header)
   - Returns
3. If license passes:
   - Checks $6C59 (CS1 present): if nonzero, checks $6C5B
     (CS1 protection) == $01; if so, treats as no game
     ($6C55=$00, fallback copy)
   - If $6C59 == $00: checks $6C58 (CS0 present) == $00;
     if so, treats as no game ($6C55=$00, fallback copy)
4. Checks $200020 (software ID) != $FFFE
   - If $FFFE: sets $6C55=$02, calls $FF31EA (header copy from
     cartridge ROM), returns
5. Sets $6C55=$01 (commercial game loaded)
6. Calls $FF31EA (header copy from cartridge ROM):
   - Copies 12 bytes from $200024 to $6C08 (title)
   - Reads 32-bit entry point from $20001C to $6C00
   - Copies 16-bit software ID from $200020 to $6C04
   - Copies 8-bit sub-code from $200022 to $6C06
   - Copies 8-bit system code from $200023 to $6F90

##### Cart Title Match ($FF326C)

Checks whether the current cartridge matches the previous
session by comparing header data in two passes:

**Pass 1 - Cart ROM vs system RAM:**

1. Sets A=$07 (bit mask for mismatch tracking)
2. Compares 12 bytes at $200024 (cart title) against $6C08
   (system RAM title). If all match: clears bit 0 of A
3. Compares 16-bit value at $200020 (software ID) against
   $6C04. If match: clears bits 1 and 2 of A
4. If A == 0: returns (full match)

**Pass 2 - Cart ROM vs BIOS ROM fallback:**

5. Compares 12 bytes at $6C08 (system RAM title) against $FFE246
   (BIOS ROM fallback title data)
6. Compares 16-bit value at $FFE242 against $6C04
7. Returns A=0 if match, nonzero if no match in either pass

##### Cart Saved State Match ($FF32DA)

Compares current cartridge header data against values at
$6C6C and $6C69 (purpose and source of these values unknown):

1. Sets A=$07
2. Compares 12 bytes at $6C08 (title from header copy) against
   $6C6C (source unknown)
3. Compares 16-bit value at $6C69 (source unknown) against
   $6C04 (current software ID)
4. Returns A=0 if match, nonzero if mismatch

This is called at INT0 first boot cycle step 21. A mismatch
causes the BIOS to take the low battery path at $FF2A1E, which
restarts the boot cycle.

**Unsolved: source of $6C6C/$6C69 data on first boot.** The BIOS
clears $6C69 and $6C6C during cold boot step 9, and no BIOS
code writes to these addresses before $FF32DA reads them at
INT0 step 21. Disassembly of the header copy routine ($FF31EA),
its caller ($FF314C), and the INT0 handler ($FF2960-$FF29A6)
confirms there are no writes to $6C6C-$6C77 or $6C69-$6C6A.
The same applies to $6C46 (cart-present flag) - the BIOS reads
it at step 15 ($FF2968) but no BIOS code writes to it during
the cold boot cycle. The warm boot path ($FF1800) does not
clear or re-probe $6C46.

The mechanism by which real hardware populates $6C46, $6C6C,
and $6C69 during cold boot is unknown.

### Warm Boot Flow

The warm boot entry point is **$FF1800** (BIOS ROM offset $1800).
This path performs minimal SFR initialization and then reaches the
shared decision point at $FF1AB2 where the configuration marker
determines whether setup screens run.

The warm boot path:

1. DI, sets XSP=$6C00, LDF 3
2. Initializes SFR registers (timers, watchdog, IIMC, $80 cycling)
3. Calls $FF239D with A=0 (RAM vector table init)
4. Clears all interrupt priorities ($70-$7A = $00)
5. Calls $FF309C (flash chip ID probing)
6. Clears $6F84, $6F85, $6F86; ANDs $6F83 with $10 (preserves bit 4)
7. Jumps to $FF1A73 which performs $80 cycling, resets XSP=$6C00,
   then calls $FF326C (cart title match) and $FF314C (license
   validation / header copy)
8. Continues to $FF1AB2 (shared with the cold boot flow) where
   configuration validity determines whether setup runs

When battery-backed RAM is intact and configuration is valid
($6E95=$4E50 + checksum passes), the $FF1AB2 check passes and
the setup screen is skipped. The path proceeds through battery
checks, boot marker writes, SNK logo animation, startup screen,
then cart handoff.

When configuration is invalid ($6E95!=$4E50 or checksum fails),
the setup screens run before the logo and cart handoff. This
can occur on warm boot if the battery-backed RAM was corrupted
or if the config marker was deliberately cleared.

### NVRAM Persistence

Battery-backed RAM preserves the following state across power cycles
to support the warm boot path:

| Region | Address Range | Size | Contents |
|--------|---------------|------|----------|
| System RAM | $4000-$6FFF | 12 KB ($3000) | Work RAM + system state variables |
| I/O registers | $80-$9F | 32 bytes ($20) | First $20 bytes of custom I/O (includes RTC values) |

Total NVRAM size: $3020 bytes.

On save: capture the full RAM contents and the first $20 bytes of the
custom I/O register space. On load: restore both regions and use the
warm boot entry point ($FF1800).

The I/O register persistence is necessary because it includes the RTC
register shadow values ($91-$96) that the BIOS uses to maintain time
continuity across power cycles.

### Known Challenges

- **Clock gear register**: The hardware register for clock gear control has
  not been identified in TMP95C061 or TMP95CS64F datasheets. The BIOS will
  attempt to write to it during VECT_CLOCKGEARSET. See
  [ngpc_soc_peripherals.md](ngpc_soc_peripherals.md) for details. This may
  need to be identified via BIOS disassembly.
- **First-time setup UI**: The BIOS contains a full interactive setup flow
  for language and clock configuration. This exercises the input system,
  K2GE rendering, and RTC writes.
- **Boot animation**: The SNK logo animation exercises K2GE sprite/tile
  rendering and timing.
- **Flash testing**: The BIOS tests the last flash block on every boot,
  which requires a working flash state machine.
- **A/D converter (battery voltage)**: The BIOS reads AN0 via the A/D
  converter during VBlank to monitor battery voltage. The emulator must
  return a sane value (e.g. $3FF for full battery) or the BIOS may enter
  a low-battery shutdown path.

---

## High-Level Emulation Alternative

### How HLE Works

HLE replaces BIOS code with native emulator functions. The boot
sequence, setup UI, and boot animation are all skipped. The emulator
initializes hardware and RAM to post-boot state and jumps directly
to the cartridge entry point.

### HLE Boot Sequence

1. Fill the BIOS ROM region ($FF0000-$FFFFFF) with $00 or $FF
2. Write the SWI 1 dispatch routine into the BIOS ROM (see below)
3. Write interrupt dispatch stubs into the BIOS ROM (see below)
4. Write the reset vector at $FFFF00 pointing to a HALT or the
   cartridge entry point
5. Populate the SWI 1 jump table at $FFFE00 with handler addresses
   that the emulator can intercept
6. Pre-initialize SFR, K2GE, and system RAM to post-boot state
   (see [HLE Initialization Values](#hle-initialization-values))
7. Copy cartridge header data to system RAM (see below)
8. Set CPU registers: PC = cartridge entry point (ROM[$1C-$1E]),
   SR = $F800, XSP = $6C00
9. Begin execution

#### Cartridge Header Copy

The following fields must be copied from the cartridge ROM header
to system RAM before jumping to the entry point:

| Source (cart offset) | Destination | Size | Field |
|----------------------|-------------|------|-------|
| $1C-$1F | $6C00 | 4 | Entry point (also used as initial PC) |
| $20-$21 | $6C04 | 2 | Software ID |
| $22 | $6C06 | 1 | Sub-code |
| $23 | $6F90 | 1 | System code ($00=mono, $10=color) |
| $24-$2F | $6C08 | 12 | Title |
| $20-$21 | $6E82 | 2 | Software ID (duplicate) |
| $22 | $6E84 | 1 | Sub-code (duplicate) |

Additionally set $6F91 and $6F95 from the system code byte ($23).

### SWI 1 Dispatch

The dispatch mechanism is described in
[System Call Interface - Mechanism](#mechanism). For HLE, the
emulator must generate dispatch code in the synthetic BIOS ROM.

The SWI 1 handler must:

1. Disable interrupts (DI), switch to register bank 3 (LDF 3)
2. Multiply W3 by 4 (ADD.B W,W twice) to get the jump table offset
3. Read the handler address from $FFFE00 + offset
4. Call the handler (which the emulator intercepts)
5. Return via RETI

The jump table entries at $FFFE00 must point to addresses where
the emulator can intercept execution. Two approaches:

- **Trap instruction**: place an undefined opcode at each handler
  address. The CPU raises an exception, the emulator catches it
  and executes the native implementation.
- **Callback hook**: check PC after each step. If PC matches a
  known handler address, execute the native implementation and
  advance PC past the handler.

### HLE Interrupt Dispatch

For each hardware interrupt, the BIOS ROM must contain a stub that
dispatches to the user handler via the RAM vector table ($6FB8-$6FFC).

Each stub must:

1. PUSH SR
2. LDF 3 (switch to register bank 3)
3. Read the 32-bit handler address from the RAM vector table entry
4. If the address equals the default handler ($FF23DF), skip the call
5. Otherwise CALL the user handler address
6. RETI

The RAM vector table entries and their interrupt sources:

| Address | Source | ROM Vector Offset |
|---------|--------|-------------------|
| $6FB8 | SWI 3 | $FFFF0C |
| $6FBC | SWI 4 | $FFFF10 |
| $6FC0 | SWI 5 | $FFFF14 |
| $6FC4 | SWI 6 | $FFFF18 |
| $6FC8 | INT0 (RTC Alarm) | $FFFF28 |
| $6FCC | INT4 (VBlank) | $FFFF2C |
| $6FD0 | INT5 (Z80) | $FFFF30 |
| $6FD4 | INTT0 | $FFFF40 |
| $6FD8 | INTT1 | $FFFF44 |
| $6FDC | INTT2 | $FFFF48 |
| $6FE0 | INTT3 | $FFFF4C |
| $6FE4 | Serial TX (INTTX0) | $FFFF64 |
| $6FE8 | Serial RX (INTRX0) | $FFFF60 |
| $6FEC | (reserved) | - |
| $6FF0 | INTTC0 | $FFFF74 |
| $6FF4 | INTTC1 | $FFFF78 |
| $6FF8 | INTTC2 | $FFFF7C |
| $6FFC | INTTC3 | $FFFF80 |

All 18 entries should be initialized to the default handler address.
For HLE, this can be any address in the synthetic BIOS ROM that
contains a single RETI instruction.

#### VBlank Handler ($FF2163)

The VBlank (INT4) handler is the most important interrupt for HLE.
The real BIOS performs system housekeeping before dispatching to the
user handler. HLE must replicate this or games that depend on
BIOS-maintained state will not function correctly.

The full handler sequence:

1. **Interrupt acknowledge**: if bit 5 of $6F86 is clear, saves
   $B2 to $6E85, then OR $B2 with $01
2. **Auto power-off check**: if bit 7 of $6F85 is set, calls
   SWI 1 with C=0 (VECT_SHUTDOWN)
3. **Input scanning** ($FF26FC): reads $B0, performs edge
   detection with $6C5F (previous state), computes newly-pressed
   and newly-released bits, stores combined state in $6F82.
   The input byte format (active-high):
   - Bit 0: Up
   - Bit 1: Down
   - Bit 2: Left
   - Bit 3: Right
   - Bit 4: Button A
   - Bit 5: Button B
   - Bit 6: Option
4. **Sound DMA** ($FF344C): if $6DA2 bit 7 is set (sound enabled)
   and bit 5 is clear: services Z80 sound ring buffer at $6D80.
   Reads next byte from ring buffer indexed by $6DA0, writes to
   ($BC) and ($BA) ports (Z80 communication). Advances $6DA0,
   wraps at $1F (32-byte ring buffer)
5. **Auto power-off timer**: if $6F85 bit 6 is clear and $6F86
   bit 6 is set: increments ($6C16). If $6F82 != 0 (button
   pressed), resets counter. If $6C16 >= $8CA0 (36000 frames,
   about 10 minutes at 60fps): resets counter, clears $6C1E,
   SET 6,($6F85) to trigger auto power-off
6. **Power button LED** ($FF2B26): reads $B1, edge-detects on
   $6C21. If $B1 bit 1 not pressed: increments $6C22. On
   overflow: $6F8A=$80, $8400=$07, $8402=$10 (LED blink).
   If pressed: resets $6C22
7. **Power button LED restore** ($FF2B5F): if $6F8A bit 7 set
   and $B1 bit 1 pressed: clears $6F8A, $8400=$FF, $8402=$00
8. **Battery monitor**: if $6F83 bit 3 is clear: increments
   ($6C18), compares against ($6C1A). If exceeded: calls $FF2E5D
   (triggers ADC battery read by modifying $70 to enable INTAD
   and setting $6D=$04 to start conversion). If $6E80==$FF
   (flash not ready), skips. If battery ($6F80) < $01D3: calls
   SWI 1 with C=0 (low battery shutdown)
9. **Sleep timer** ($FF2B7D): if $6F85 bit 7 not set and $6C20
   != 0: checks $B1 bit 0. If not pressed: increments $6C1C,
   if >= $001F: clears $6C20, SET 7,($6F85). If pressed:
   resets $6C1C and $6C20
10. **User vector dispatch**: PUSH ($6FCE), PUSH ($6FCC) pushes
    user VBlank handler address. If $6F86 bit 5 is clear:
    restores $B2 from $6E85. RET dispatches to user handler.
    $FF239D configures ($6FCC/$6FCE) based on mode parameter A

Required HLE housekeeping (minimal):

1. **Scan input**: read $B0 -> $6F82 with edge detection
2. **Battery voltage**: write $03FF to $6F80 (full battery)
3. **Dispatch**: call user handler at ($6FCC/$6FCE)

### System Call Implementation Summary

The following table summarizes the implementation approach for each
system call in HLE. See [System Call Details](#system-call-details)
for full parameter and return value documentation.

| Index | Name | HLE Approach |
|-------|------|--------------|
| $00 | VECT_SHUTDOWN | Halt CPU or enter idle loop |
| $01 | VECT_CLOCKGEARSET | Write gear value to $80 (clock gear register) |
| $02 | VECT_RTCGET | Read host system time, convert to BCD, write 7 bytes to (XHL3) |
| $03 | (unknown) | Stub (RET) |
| $04 | VECT_INTLVSET | Read-modify-write interrupt priority register. RC3=source, RB3=level |
| $05 | VECT_SYSFONTSET | Decompress 1bpp font to 2bpp at $A000. Requires font data (see below) |
| $06 | VECT_FLASHWRITE | Write pages to flash. RA3=bank, BC3=page count, XDE3=dest, XHL3=src |
| $07 | VECT_FLASHALLERS | Erase all flash blocks (fill $FF). RA3=bank |
| $08 | VECT_FLASHERS | Erase one flash block. RA3=bank, RB3=block number |
| $09 | VECT_ALARMSET | Stub (return $00 in RA3) |
| $0A | (stub) | RET |
| $0B | VECT_ALARMDOWNSET | Stub (return $00 in RA3) |
| $0C | (unknown) | Stub (RET) |
| $0D | VECT_FLASHPROTECT | Stub (return $00 in RA3) |
| $0E | VECT_GEMODESET | Write K2GE mode register based on $6F91 system code |
| $0F | (stub) | RET |
| $10-$1A | VECT_COM* | Stub all serial calls. Return COM_BUF_EMPTY ($01) for receive, COM_BUF_OK ($00) for send |

### System Font (VECT_SYSFONTSET)

VECT_SYSFONTSET requires 2048 bytes ($800) of 1bpp font data
representing 256 8x8 characters. The font data is stored in the
BIOS ROM at offset $8DCF (CPU address $FF8DCF), spanning $8DCF-$95CE.
The handler at $FF8D8A loads XIY with $00FF8DCF as the source address
and XIX with $0000A000 as the destination. The decompression loop at
$FF8DAA reads source data from XIY and writes 2bpp output to XIX
(K2GE character RAM starting at $A000).

For HLE without a BIOS ROM file, the font data must be provided
separately. Options:

- Extract the font data from a real BIOS ROM at offset $8DCF
- Generate a substitute font (ASCII-range bitmap font in 1bpp format)

The decompression algorithm for each source byte:

1. For each bit in the source byte (MSB first, 8 bits per byte):
   - If bit is set: write the font color index (RA3 bits 0-1)
   - If bit is clear: write the background color index (RA3 bits 4-5)
2. Pack pairs of 2-bit values into output bytes
3. Each 8x8 character produces 16 bytes of 2bpp output ($10 bytes)
4. Total output: 256 characters * 16 bytes = 4096 bytes ($1000)
   written to $A000-$AFFF

### HLE Limitations

- Boot animation and first-time setup UI are skipped entirely
- Alarm wakeup and resume-from-sleep paths are not implemented
- Serial communication calls are stubbed (return COM_BUF_EMPTY)
- Clock gear writes to $80 but actual CPU speed scaling depends on
  the emulator's clock gear implementation
- VECT_SHUTDOWN halts execution rather than powering off
- Warm boot / NVRAM restore is not supported (always cold boot state)
- System font requires either a BIOS ROM file or substitute font data
- Vectors $03 and $0C are undocumented and stubbed as RET

---

## HLE Initialization Values

The following values must be written before jumping to the cartridge
entry point. These represent the hardware and RAM state after the real
BIOS completes boot.

### SFR Registers ($00-$7F)

Key non-zero values to write to the SFR area (verified from memdump
at cart handoff):

| Address | Register | Value | Description |
|---------|----------|-------|-------------|
| $20 | TRUN | $80 | Prescaler enabled, all timers stopped |
| $22 | TREG0 | $01 | Timer 0 compare value |
| $23 | TREG1 | $90 | Timer 1 compare value |
| $24 | T01MOD | $03 | Timer 0/1 mode |
| $25 | TFFCR | $B0 | Timer flip-flop control |
| $26 | TREG2 | $90 | Timer 2 compare value |
| $27 | TREG3 | $62 | Timer 3 compare value |
| $28 | T23MOD | $05 | Timer 2/3 mode |
| $38 | T4MOD | $30 | Timer 4 mode register |
| $52 | SC0MOD | $69 | Serial Channel 0 mode |
| $53 | BR0CR | $15 | Baud rate 0 control |
| $6E | WDMOD | $F0 | Watchdog mode (enabled, all protection bits set) |
| $6F | WDCR | $4E | Watchdog serviced |
| $70 | INTE0AD | $02 | INT0 priority = 2 |
| $71 | INTE45 | $54 | INT4 priority = 4, INT5 priority = 5 |

Additionally, the following custom I/O registers should be set:

| Address | Register | Value | Description |
|---------|----------|-------|-------------|
| $B2 | RTS state | $01 | RTS disabled |
| $B3 | NMI enable | $04 | Power button NMI enabled (bit 2) |
| $B6 | Power state | $05 | Normal operation |

Additional SFR and custom I/O registers are set by HLE - see
hle_init_comparison.md for the complete list including port registers,
memory controller, 16-bit timers, ADC registers, and more.

Note: the real BIOS enables both the T6W28 sound chip and Z80 during
boot via $FF23E0 ($B8=$5555, meaning $B8=$55 and $B9=$55). The BIOS
has a built-in Z80 sound driver at $FF0000-$FF0FFF which is copied to
Z80 RAM ($7000-$7FFF) and plays music during the SNK logo and startup
animations. Before cart handoff (step 43), both are disabled
($B8=$AAAA). For HLE, both $B8 and $B9 can be set to $AA (disabled)
since the boot animation sound is not played. The BIOS sound driver
remains in Z80 RAM after cart handoff (Z80 RAM is not cleared).
Games may load their own driver or re-enable the existing one.

### K2GE Registers ($8000-$BFFF)

| Address | Value | Description |
|---------|-------|-------------|
| $8000 | $C0 | VBlank and HBlank interrupts enabled |
| $8006 | $C6 | Frame rate control |
| $8118 | $80 | Background color on |
| $8400 | $FF | LED on |
| $87F4 | $80 | K2GE config |
| $9000-$91FF | $0020 (word) | Sprite table 1 (64 entries, default tile) |
| $9800-$99FF | $0020 (word) | Sprite table 2 (64 entries, default tile) |

**Grayscale Palette ($8380-$83FF):** The real BIOS writes a grayscale
ramp to this region during boot, but memdump.bin shows all zeros at
cart handoff. HLE does not set these registers. The grayscale palette
data is instead stored in work RAM at $6DD8-$6E57 (8 copies of the
16-byte ramp: FF 0F DD 0D BB 0B 99 09 77 07 44 04 33 03 00 00).

**$87E0 and $87F0:** The real BIOS writes $87E0=$53 then $47 and
$87F0=$AA/$55 during boot. memdump.bin shows both as $00 at cart
handoff. HLE does not set these; $87F0 is written dynamically by
the GEModeSet syscall ($0E) when games call it.

### System RAM ($6C00-$6FFF)

Values derived from the cartridge header and system configuration:

| Address | Size | Value | Description |
|---------|------|-------|-------------|
| $6C00 | 4 | ROM[$1C-$1F] | Cartridge start PC |
| $6C04 | 2 | ROM[$20-$21] | Cartridge software ID |
| $6C06 | 1 | ROM[$22] | Cartridge sub-code |
| $6C08 | 12 | ROM[$24-$2F] | Cartridge title |
| $6C14 | 1 | $DC | Setup completion checksum |
| $6C15 | 1 | $00 | Setup completion checksum high byte |
| $6C18 | 2 | $01BC | A/D conversion counter |
| $6C1A | 2 | $0258 | A/D conversion interval |
| $6C21 | 1 | $02 | Power button state |
| $6C24 | 1 | $0A | INT0/A/D priority config |
| $6C25 | 1 | $DC | Setup checksum input value |
| $6C46 | 1 | $01 | Cart present flag |
| $6C55 | 1 | $01 | Commercial game loaded |
| $6C58 | 1 | flash type | CS0 flash type (derived from ROM size) |
| $6C59 | 1 | flash type if ROM > 2 MB, else $00 | CS1 flash type |
| $6C5F | 1 | (not set) | Previous input state (VBlank housekeeping updates this) |
| $6C69 | 2 | ROM[$20-$21] | Software ID (duplicate) |
| $6C6C | 12 | ROM[$24-$2F] | Cartridge title (duplicate) |
| $6C7A | 2 | $A5A5 | Boot marker |
| $6C7C | 2 | $5AA5 | Boot cycle marker (enables INT0 dispatch path) |
| $6C7E | 1 | $01 if CS1 present, else $00 | CS1 present flag |
| $6E82 | 2 | ROM[$20-$21] | Software ID (duplicate) |
| $6E84 | 1 | ROM[$22] | Sub-code (duplicate) |
| $6E88 | 1 | $01 | Initialized during boot (purpose unknown) |
| $6E95 | 2 | (not set) | Config validity marker (written by setup screen on cold boot) |
| $6F80 | 2 | $03FF | Battery voltage (full, little-endian) |
| $6F82 | 1 | (not set) | Input state (VBlank housekeeping updates this each frame) |
| $6F83 | 1 | $40 | Bit 6 set at cart handoff |
| $6F84 | 1 | $40 | User Boot = Power ON |
| $6F85 | 1 | $00 | No shutdown request |
| $6F86 | 1 | $00 | User Answer clear |
| $6F87 | 1 | $01 | Language ($00=Japanese, $01=English) |
| $6F90 | 1 | system code | $00=mono, $10=color (from ROM header $23) |
| $6F91 | 1 | system code | $00=mono, $10=color (duplicate of $6F90) |
| $6F92 | 1 | flash type | CS0 flash type (duplicate of $6C58) |
| $6F93 | 1 | flash type if CS1, else $00 | CS1 flash type (duplicate of $6C59) |
| $6F94 | 1 | $00 | Mono game color palette (0=Black & White, 1=Red, 2=Green, 3=Blue, 4=Classic) |

Flash type values: 0=none, 1=4Mbit (512KB), 2=8Mbit (1MB), 3=16Mbit (2MB).
Derived from the cartridge ROM size rather than hardcoded.

### RAM Vector Table ($6FB8-$6FFC)

All 18 entries (4 bytes each) should be initialized to point to
a default handler. The real BIOS uses $FF23DF (a single RETI
instruction). For HLE, use the address of a RETI instruction
placed in the synthetic BIOS ROM. Games overwrite individual
entries with their own handler addresses before enabling the
corresponding interrupts.

### CPU Initial State for HLE

| Register | Value |
|----------|-------|
| PC | ROM[$1C-$1E] (cartridge entry point, 24-bit little-endian) |
| SR | $F800 (system mode, IFF=7, MAX mode, bank 0) |
| XSP | $6C00 |

The game's startup code will typically set its own stack pointer
and lower the interrupt mask (EI) early in execution.

---

## System Call Interface

### Mechanism

Games invoke BIOS services via the `SWI 1` instruction. The dispatch
mechanism works as follows:

1. Game executes `SWI 1` - CPU pushes PC and SR onto the system stack,
   reads the SWI 1 vector from $FFFF04
2. The BIOS handler disables interrupts (`DI`), switches to register
   bank 3 (`LDF 3`)
3. Multiplies W3 by 4 to get the jump table offset (`ADD.B W,W` twice,
   byte-width arithmetic limits effective range to indices $00-$3F)
4. Reads the 32-bit handler address from $FFFE00 + W offset
5. Stores handler address to $6C49, dispatches via PUSH/PUSH/RET
6. Handler returns via `RETI`

The call number is passed in the W3 register (RC3/RB3 pair) before the
`SWI 1` instruction. For the real BIOS, this dispatch code lives in the
ROM. For HLE, equivalent dispatch code must be generated in the
synthetic BIOS ROM (see
[SWI 1 Dispatch](#swi-1-dispatch) under HLE).

### Register Convention

Parameters are passed in TLCS-900H register bank 3 (which is reserved for
BIOS use - application software must not use register bank 3):

| Register | Code | Width | Typical Use |
|----------|------|-------|-------------|
| RA3 | $30 | 8-bit | Return status (0=success, $FF=failure) |
| RB3 | $35 | 8-bit | Primary parameter |
| RC3 | $34 | 8-bit | Secondary parameter |
| RW3 | $34-$35 | 16-bit | Function selector / 16-bit parameter |
| RWA3 | $30-$31 | 16-bit | 16-bit return value |
| XHL3 | $3C-$3F | 32-bit | Pointer (source or destination buffer) |
| XDE3 | $38-$3B | 32-bit | Pointer (destination address) |

### Jump Table

The jump table at $FFFE00 has 4-byte spacing. Each entry holds a 24-bit
(3-byte, little-endian) address pointing to the handler code within the
BIOS ROM. The fourth byte of each 4-byte slot is unused padding.

| Address | Index | Handler | System Call | Description |
|---------|-------|---------|-------------|-------------|
| $FFFE00 | $00 | $FF27A2 | VECT_SHUTDOWN | System power-off |
| $FFFE04 | $01 | $FF1034 | VECT_CLOCKGEARSET | Set CPU clock divider |
| $FFFE08 | $02 | $FF1440 | VECT_RTCGET | Read real-time clock |
| $FFFE0C | $03 | $FF12B4 | VECT_RTC_ALARM_SET | RTC alarm set (validates time fields) |
| $FFFE10 | $04 | $FF1222 | VECT_INTLVSET | Set interrupt priority level |
| $FFFE14 | $05 | $FF8D8A | VECT_SYSFONTSET | Load system font to character RAM |
| $FFFE18 | $06 | $FF6FD8 | VECT_FLASHWRITE | Write data to flash |
| $FFFE1C | $07 | $FF7042 | VECT_FLASHALLERS | Erase all flash blocks |
| $FFFE20 | $08 | $FF7082 | VECT_FLASHERS | Erase specified flash block |
| $FFFE24 | $09 | $FF149B | VECT_ALARMSET | Set RTC alarm (during operation) |
| $FFFE28 | $0A | $FF1033 | (stub) | Confirmed RET |
| $FFFE2C | $0B | $FF1487 | VECT_ALARMDOWNSET | Set power-on alarm |
| $FFFE30 | $0C | $FF731F | VECT_FLASHPROTECT_CHECK | Check flash block protection status |
| $FFFE34 | $0D | $FF70CA | VECT_FLASHPROTECT | Protect flash blocks (permanent) |
| $FFFE38 | $0E | $FF17C4 | VECT_GEMODESET | Set graphics mode (K1GE/K2GE) |
| $FFFE3C | $0F | $FF1032 | (stub) | Confirmed RET |
| $FFFE40 | $10 | $FF2BBD | VECT_COMINIT | Initialize serial communication |
| $FFFE44 | $11 | $FF2C0C | VECT_COMSENDSTART | Begin transmission |
| $FFFE48 | $12 | $FF2C44 | VECT_COMRECIVESTART | Begin reception |
| $FFFE4C | $13 | $FF2C86 | VECT_COMCREATEDATA | Send single byte |
| $FFFE50 | $14 | $FF2CB4 | VECT_COMGETDATA | Receive single byte |
| $FFFE54 | $15 | $FF2D27 | VECT_COMONRTS | Enable RTS signal |
| $FFFE58 | $16 | $FF2D33 | VECT_COMOFFRTS | Disable RTS signal |
| $FFFE5C | $17 | $FF2D3A | VECT_COMSENDSTATUS | Query send queue depth |
| $FFFE60 | $18 | $FF2D4E | VECT_COMRECIVESTATUS | Query receive queue depth |
| $FFFE64 | $19 | $FF2D6C | VECT_COMCREATEBUFDATA | Send data buffer |
| $FFFE68 | $1A | $FF2D85 | VECT_COMGETBUFDATA | Receive data buffer |

Handler addresses verified from BIOS ROM hex dump at $FFFE00-$FFFE6B.
Vectors $0A ($FF1033) and $0F ($FF1032) are 1-2 bytes before
VECT_CLOCKGEARSET ($FF1034), strongly suggesting they are stub handlers
(RET or NOP+RET).

Note: the system reference document lists a subset of these communication
vectors with different names (VECT_COMEND, VECT_COMSENDDATA, etc.) sourced
from ngpctech.txt. The names and indices above come from the HLE
implementations which are more complete.

### SYS_PATCH

The SDK specifies that games must call a library subroutine SYS_PATCH during
startup to handle hardware revision differences. The mechanism and contents
of this patch are unknown.

---

## System Call Details

Parameter and return value details below are derived from publicly available
specification documents where possible. Items marked with (?) require
verification via BIOS disassembly.

### VECT_SHUTDOWN ($00)

Initiates system power-off. Games must call this when $6F85 is set (power
button pressed).

- Parameters: none
- Returns: does not return
- HLE: enter an idle loop or halt execution

### VECT_CLOCKGEARSET ($01)

Sets the CPU clock divider.

- Parameters:
  - RB3: gear value (0-4)
  - RC3: auto-regeneration flag (?)
- Gear mapping:

| Gear | Divisor | Clock Rate |
|------|---------|-----------|
| 0 | /1 | 6.144 MHz |
| 1 | /2 | 3.072 MHz |
| 2 | /4 | 1.536 MHz |
| 3 | /8 | 768 KHz |
| 4 | /16 | 384 KHz |

- The gear value is written to custom I/O register $80. The emulator
  uses this to scale CPU cycles per scanline by the gear divisor.
- HLE: write the gear value (clamped to 0-4) to $80.

### VECT_RTCGET ($02)

Reads the current date and time from the RTC.

- Parameters:
  - XHL3: pointer to 7-byte destination buffer
- Returns: 7 bytes written to buffer in BCD format:

| Offset | Content |
|--------|---------|
| 0 | Year (BCD, 00-99) |
| 1 | Month (BCD, 01-12) |
| 2 | Day (BCD, 01-31) |
| 3 | Hour (BCD, 00-23) |
| 4 | Minute (BCD, 00-59) |
| 5 | Second (BCD, 00-59) |
| 6 | Day of week in low nibble (0=Sunday through 6=Saturday). High nibble may contain years since last leap year (?) |

- The hardware RTC has 6 registers at $91-$96 (year through second).
  There is no hardware day-of-week register. The BIOS computes byte 6
  from the date. The exact encoding of the high nibble requires
  verification via BIOS disassembly.

### VECT_INTLVSET ($04)

Sets the interrupt priority level for a specific interrupt source.

- Parameters:
  - RC3: interrupt source number (0-9)
  - RB3: priority level (0-7, where 0=disabled)

| RC3 | Source | Priority Register | Nibble |
|-----|--------|-------------------|--------|
| 0 | RTC alarm (INT0) | $70 | low |
| 1 | Z80 (INT5) | $71 | high |
| 2 | Timer 0 (INTT0) | $73 | low |
| 3 | Timer 1 (INTT1) | $73 | high |
| 4 | Timer 2 (INTT2) | $74 | low |
| 5 | Timer 3 (INTT3) | $74 | high |
| 6 | DMA 0 (INTTC0) | $79 | low |
| 7 | DMA 1 (INTTC1) | $79 | high |
| 8 | DMA 2 (INTTC2) | $7A | low |
| 9 | DMA 3 (INTTC3) | $7A | high |

Each priority register holds two 3-bit fields. For even-numbered
sources, the level is placed in the low nibble (bits 0-2). For odd
sources, the level is placed in the high nibble (bits 4-6). The
implementation reads the existing register value, masks out the
target nibble, and ORs in the new 3-bit level.

Note: the interrupt priority registers ($70-$7A) are write-only on
hardware. HLE implementations should keep a shadow copy of these
registers to support read-modify-write operations.

### VECT_SYSFONTSET ($05)

Copies the system font bitmap from BIOS ROM to K2GE character RAM at $A000.

- Parameters:
  - RA3: color control. Bits 0-1 = font color index (2bpp).
    Bits 4-5 = transparent/background color index (2bpp).
- Source: 2048 bytes ($800) of 1bpp font data (256 8x8 characters)
  stored in the BIOS ROM
- Output: 2-bits-per-pixel data written to K2GE character RAM at $A000.
  For each source bit: if set, the font color index is used; if clear,
  the transparent color index is used. Each pair of 2-bit pixels is
  packed into output bytes, producing $1000 bytes of 2bpp character data.

### VECT_FLASHWRITE ($06)

Writes data to cartridge flash memory.

- Parameters:
  - RA3: bank select (0=CS0 at $200000, 1=CS1 at $800000)
  - BC3 (register word at code $34): size in 256-byte pages
  - XDE3: destination offset within the selected bank
  - XHL3: source RAM address
- Returns:
  - RA3: $00=success (SYS_SUCCESS), $FF=failure (SYS_FAILURE)
- Copies (BC3 * 256) bytes from XHL3 to (bank_base + XDE3)
- The flash must be unlocked for writes during this operation

### VECT_FLASHALLERS ($07)

Erases all blocks on a flash chip (fills with $FF).

- Parameters:
  - RA3: bank select (0=CS0, 1=CS1)
- Returns:
  - RA3: $00=success (SYS_SUCCESS)

### VECT_FLASHERS ($08)

Erases a specific flash block (fills with $FF).

- Parameters:
  - RA3: bank select (0=CS0, 1=CS1)
  - RB3: block number (0-31, block 31 is system-reserved)
- Returns:
  - RA3: $00=success (SYS_SUCCESS)
- Block 31 covers the top of the flash chip (system-reserved area).
  See [ngpc_cartridge_format.md](ngpc_cartridge_format.md) for block
  address ranges.

### VECT_ALARMSET ($09)

Sets an RTC alarm for triggering during active game operation.

- Parameters:
  - RD3: day
  - RB3: hour
  - RC3: minute
- Returns:
  - RA3: $00=success (SYS_SUCCESS)
- HLE: can be stubbed (return success, no alarm fires)

### VECT_ALARMDOWNSET ($0B)

Sets a power-on alarm - the system will wake from off state at the specified
time.

- Parameters: unknown (?)
- Returns:
  - RA3: $00=success (SYS_SUCCESS)
- HLE: stub (return success)

### VECT_FLASHPROTECT ($0D)

Permanently protects flash blocks from further writes.

- Parameters: unknown (?)
- Returns:
  - RA3: $00=success (SYS_SUCCESS)
- Protection is irreversible on real hardware
- HLE: stub (return success). See
  [ngpc_cartridge_format.md](ngpc_cartridge_format.md) for the flash
  block protect command sequence.

### VECT_GEMODESET ($0E)

Sets the K2GE graphics mode based on the cartridge system code byte
(ROM header offset $23).

- Procedure:
  1. Write $AA to $87F0 (unlock K2GE mode register)
  2. If system code < $10: write $80 to $87E2 (mono/K1GE mode),
     write $00 to $6F95
  3. If system code >= $10: write $00 to $87E2 (color/K2GE mode),
     write $10 to $6F95
  4. Write $55 to $87F0 (re-lock K2GE mode register)
- The mode parameter appears to be read from the system code already
  stored at $6F91 during boot initialization

### VECT_COMINIT ($10)

Initializes the serial communication subsystem.

- Parameters: none
- Returns:
  - RA3: $00 (COM_BUF_OK)
- Configures Serial Channel 1 for link cable at 19,200 bps
- HLE: stub (return COM_BUF_OK)

### VECT_COMSENDSTART ($11)

Begins a serial transmission sequence.

- HLE: stub (no-op)

### VECT_COMRECIVESTART ($12)

Begins a serial reception sequence.

- HLE: stub (no-op)

### VECT_COMCREATEDATA ($13)

Sends a single byte over the serial link.

- Parameters:
  - RB3: byte to send
- Returns:
  - RA3: $00 (COM_BUF_OK)
- Side effects: after sending, triggers serial TX interrupt (vector
  at $6FE4) and checks DMA channels for start vector $18
- HLE: stub (return COM_BUF_OK). If link cable is not emulated,
  discard the data.

### VECT_COMGETDATA ($14)

Receives a single byte from the serial link.

- Returns:
  - RB3: received byte
  - RA3: $00=success (COM_BUF_OK), $01=no data (COM_BUF_EMPTY)
- Side effects on success: writes received byte to $50 (SC0BUF),
  triggers serial RX interrupt (vector at $6FE8) and checks DMA
  channels for start vector $19
- HLE: stub (return COM_BUF_EMPTY in RA3)

### VECT_COMONRTS ($15)

Enables the RTS (Request To Send) signal on the serial port.

- Writes $00 to $B2 (RTS state flag)
- HLE: write $00 to $B2

### VECT_COMOFFRTS ($16)

Disables the RTS signal on the serial port.

- Writes $01 to $B2 (RTS state flag)
- HLE: write $01 to $B2

### VECT_COMSENDSTATUS ($17)

Queries the number of bytes waiting to be sent.

- Returns:
  - RWA3: pending byte count
- HLE: return 0 (no pending bytes)

### VECT_COMRECIVESTATUS ($18)

Queries the number of bytes available to read.

- Returns:
  - RWA3: available byte count
- HLE: return 0 (no available bytes)

### VECT_COMCREATEBUFDATA ($19)

Sends a buffer of data over the serial link.

- Parameters:
  - RB3: byte count
  - XHL3: source buffer pointer
- Returns:
  - RA3: $00 (COM_BUF_OK)
- Loops: reads each byte from (XHL3), sends it, increments XHL3,
  decrements RB3 until count reaches zero
- Side effects: triggers serial TX interrupt (vector at $6FE4) and
  checks DMA channels for start vector $18
- HLE: stub (return COM_BUF_OK, discard data)

### VECT_COMGETBUFDATA ($1A)

Receives a buffer of data from the serial link.

- Parameters:
  - RB3: byte count to receive
  - XHL3: destination buffer pointer
- Returns:
  - RA3: $00=success (COM_BUF_OK), $01=no data (COM_BUF_EMPTY)
- Loops: reads each byte, writes to (XHL3), increments XHL3,
  decrements RB3. Writes received byte to $50 (SC0BUF).
- Side effects: triggers serial RX interrupt (vector at $6FE8) and
  checks DMA channels for start vector $19
- HLE: stub (return COM_BUF_EMPTY in RA3)

---

## Interrupt Dispatch

The BIOS owns the hardware interrupt vector table in ROM ($FFFF00). When a
hardware interrupt fires:

1. CPU reads a 24-bit handler address from $FFFF00 + (vector offset)
2. BIOS handler runs - performs system housekeeping:
   - VBlank handler: services watchdog, scans input (updates $6F82),
     monitors power button, reads battery voltage
   - Other handlers: minimal housekeeping
3. BIOS reads the user handler address from the RAM vector table ($6FB8-$6FFC)
4. If the user handler is non-zero, BIOS jumps to it
5. User handler runs and returns

This means the BIOS is always in the interrupt path. For real BIOS execution,
this works naturally. For HLE, the interrupt dispatch logic must be
replicated - see [HLE Interrupt Dispatch](#hle-interrupt-dispatch) for
the implementation approach including VBlank housekeeping requirements.

### ROM Vector Table ($FFFF00-$FFFF74)

See [ROM Interrupt Vector Table](#rom-interrupt-vector-table) for the
complete verified 30-entry table with all handler addresses, interrupt
sources, and dispatch mechanisms.

### RAM Vector Table

| Address | Interrupt Source |
|---------|-----------------|
| $6FB8 | SWI 3 |
| $6FBC | SWI 4 |
| $6FC0 | SWI 5 |
| $6FC4 | SWI 6 |
| $6FC8 | RTC Alarm (INT0) |
| $6FCC | VBlank (INT4) |
| $6FD0 | Z80 (INT5) |
| $6FD4 | Timer 0 (INTT0) |
| $6FD8 | Timer 1 (INTT1) |
| $6FDC | Timer 2 (INTT2) |
| $6FE0 | Timer 3 (INTT3) |
| $6FE4 | Serial TX (Channel 1) |
| $6FE8 | Serial RX (Channel 1) |
| $6FEC | (reserved) |
| $6FF0 | Micro DMA 0 End (INTTC0) |
| $6FF4 | Micro DMA 1 End (INTTC1) |
| $6FF8 | Micro DMA 2 End (INTTC2) |
| $6FFC | Micro DMA 3 End (INTTC3) |

---

## System RAM Layout

The BIOS reserves $6C00-$6FFF (1 KB) for system state. Key locations:

### System State Variables

| Address | Size | Description |
|---------|------|-------------|
| $6C00 | 4 bytes | Cartridge start PC (copied from header offset $1C) |
| $6C04 | 2 bytes | Cartridge software ID (copied from header offset $20) |
| $6C06 | 1 byte | Cartridge sub-code (copied from header offset $22) |
| $6C08 | 12 bytes | Cartridge title (copied from header offset $24) |
| $6C14 | 1 byte | Setup completion checksum ($DC at cart handoff, see below) |
| $6C15 | 1 byte | Setup completion checksum high byte (must equal $00) |
| $6C55 | 1 byte | Commercial game flag ($01 = game loaded, $00 = BIOS menu) |
| $6C58 | 1 byte | CS0 flash chip type (0=none, 1=4Mbit, 2=8Mbit, 3=16Mbit) |
| $6C59 | 1 byte | CS1 flash chip type (same encoding; set if ROM > 2 MB) |
| $6E82 | 2 bytes | Software ID (duplicate of $6C04) |
| $6E84 | 1 byte | Sub-code (duplicate of $6C06) |
| $6F80 | 1 byte | Battery voltage low byte |
| $6F81 | 1 byte | Battery voltage high byte |
| $6F82 | 1 byte | Controller input state (active-high, updated by BIOS VBlank handler each frame) |
| $6F84 | 1 byte | User Boot status (bit 7=alarm, bit 6=power on, bit 5=resume) |
| $6F85 | 1 byte | User Shutdown request (set by BIOS on power button NMI) |
| $6F86 | 1 byte | User Answer |
| $6F87 | 1 byte | Language ($00=Japanese, $01=English) |
| $6F89 | 1 byte | A/D interrupt status (bit 7 monitored by BIOS) |
| $6F91 | 1 byte | System type ($00=mono NGP, $10=color NGPC) |
| $6F95 | 1 byte | Color mode (duplicate of $6F91) |
| $00B2 | 1 byte | RTS state flag (0=enabled, 1=disabled) |

### Additional System Variables (from BIOS disassembly)

| Address | Size | Description |
|---------|------|-------------|
| $6C24 | 1 byte | INT0/A/D priority config. Low nibble OR'd with $60 and written to $70 (INTE0AD). Initial value $0A |
| $6C25 | 7 bytes | Setup checksum input values ($DC, then 6x $00 on cold boot) |
| $6C57 | 1 byte | Low 3 bits of $6F88 (stored during boot step 19) |
| $6C7A | 2 bytes | Boot marker ($A5A5 written during boot step 29) |
| $6C7C | 2 bytes | Boot completion marker ($A55A at step 18, overwritten to $5AA5 at boot completion) |
| $6DA1 | 4 bytes | Sound state (cleared to $00000000 during sound init) |
| $6DA4 | 1 byte | Sound state (cleared during sound init) |
| $6E88 | 1 byte | Initialized to $01 during boot (purpose unknown) |
| $6E95 | 2 bytes | Boot marker ($4E50 written during boot step 29) |
| $6F83 | 1 byte | Bit 4 set (step 14), bit 6 cleared (step 17), bit 3 cleared (step 24) |
| $6F86 | 1 byte | Bit 7 cleared after each HALT/interrupt cycle (step 32) |
| $6C16 | 2 bytes | Cleared to $0000 during boot step 24 |
| $6C18 | 2 bytes | A/D conversion counter (cleared to $0000 by INTAD ISR) |
| $6C1A | 2 bytes | A/D conversion interval ($0258 set by INTAD ISR) |
| $6C1C | 2 bytes | Cleared to $0000 during boot step 24 |
| $6C1E | 2 bytes | Cleared to $0000 during boot step 24 |
| $6C2C | 1 byte | Written $00 after each $80 cycling step. No known functional effect |
| $6C46 | 1 byte | Cart present flag. On cold boot, cleared to $00 at step 9 ($FF24B9), then checked by INT0 handler at $FF2968 (nonzero=cart present). No BIOS code writes this flag during the cold boot cycle before it is checked - the source is external to the BIOS (unknown mechanism). The warm boot path ($FF1800) does not clear or re-probe this flag |
| $6C47 | 1 byte | User setup state (language). Checked by INT0 handler; $02=setup done |
| $6C48 | 1 byte | User setup state (clock). Checked by INT0 handler; $02=setup done |
| $6C55 | 1 byte | Commercial game flag ($01=game loaded, $00=BIOS menu/no game). Set by $FF314C after license validation passes |
| $6C58 | 1 byte | CS0 flash chip type (0=none, 1=4Mbit, 2=8Mbit, 3=16Mbit). Set by $FF309C |
| $6C59 | 1 byte | CS1 flash chip type (same encoding as $6C58). Set by $FF309C |
| $6C5A | 1 byte | CS0 flash protection status byte. Set by $FF309C |
| $6C5B | 1 byte | CS1 flash protection status byte. Set by $FF309C |
| $6C69 | 2 bytes | Software ID compared against $6C04 by $FF32DA. Source unknown - never written by BIOS during first boot cycle (see $FF32DA notes) |
| $6C6C | 12 bytes | Cart title compared against $6C08 by $FF32DA. Source unknown - never written by BIOS during first boot cycle (see $FF32DA notes) |
| $6C7E | 1 byte | CS1 present flag ($00=no CS1, $01=CS1 present). Set by $FF309C |
| $6C61 | 1 byte | A/D conversion in progress flag ($FF=active, $00=idle) |
| $6F88 | 1 byte | Read during boot; bits 0-2 stored to $6C57 |
| $6F89 | 1 byte | Bit 7: A/D conversion started flag (set by $FF2E51, cleared by INTAD ISR) |
| $6F8A | 1 byte | Bit 7 checked by INTAD ISR for LED control |
| $6F90 | 1 byte | System code from cart header ($200023). $00=mono, $10=color. Set by $FF31EA |
| $6F92 | 1 byte | CS0 flash chip type (copy of $6C58). Set by $FF309C |
| $6F93 | 1 byte | CS1 flash chip type (copy of $6C59). Set by $FF309C |
| $6E8B | 1 byte | Boot progress marker. Set to $07 then $08 during cart validation steps 25-27 |

### Setup Completion Checksum

The BIOS computes a checksum to determine if first-time setup has been
completed. The checksum at $6C14-$6C15 is computed as a 16-bit sum
stored in little-endian (low byte at $6C14, high byte at $6C15):

```
$6C14 = $6F87 + $6C25 + $6C26 + $6C27 + $6C28 + $6C29 + $6C2A + $6C2B + $6F94
$6C15 = high byte of the running sum (normally $00)
```

The checksum is computed and stored by $FF3310 (called at boot step 15
and at $FF2840 before each boot cycle restart). On cold boot, the
initial sum is $DC (only $6C25=$DC contributes, $6F87=$00, $6F94=$00).
This value matches on the first pass through $FF1AB2 since $6C14 was
just written. The first-time setup screen runs based on $6F83 bit 4,
not checksum failure. After the user changes $6F87 (e.g., to $01 for
English), the checksum is recomputed on the next boot cycle restart,
storing the new value ($DC+$01=$DD).

For HLE, $6C14=$DC and $6C15=$00. The checksum is computed before the
language selection is applied to $6F87, so the sum is $DC regardless of
language choice. The memdump.bin confirms $DC at cart handoff with
$6F87=$01 (English). The values at $6C25-$6C2B are interrupt priority
configuration mirrored from $70-$7A by $FF23F3.

### HLE RAM Initialization

For HLE, these locations must be pre-initialized to simulate post-boot
state. See [HLE Initialization Values](#hle-initialization-values) for
the complete set of values for SFR, K2GE, and system RAM.

---

## Hardware the BIOS Touches

The following hardware must be emulated for real BIOS execution. This is also
the set of functionality that HLE must account for or bypass.

### During Boot

| Component | What the BIOS Does |
|-----------|--------------------|
| Interrupt controller ($70-$7A) | Sets initial priority levels |
| Timer registers ($20-$2F, $30-$49) | Configures 8-bit and 16-bit timers |
| Watchdog ($6E-$6F) | Enables (WDMOD=$04 then $14) and services (WDCR=$4E via $FF1ED7, also $B1) |
| K2GE ($8000-$BFFF) | Sets display mode, window size, palette defaults, clears sprite tables ($9000/$9800 fill $0020), clears char attrs ($8800 fill $0000), LED control ($8400/$8402) |
| K2GE $87E0 | Writes $53 then $47 during init (between $87F0 unlock/lock), done twice |
| K2GE $8010 | VBlank status polled (bit 6) by frame delay routine at $FF112F |
| Flash controller (CS0/CS1) | Reads chip ID, tests last block |
| RTC ($90-$97) | Writes default time (1998-01-01), configures $90 control (clear bit 1, set bit 0) |
| Input (port registers) | Scans buttons for setup UI (first boot) |
| NMI enable ($B3) | Enables power button NMI (bit 2) |
| Power state ($B4-$B7) | Writes $B4=$A0 before HALT, $B6=$50 during display init |
| Serial port / RTS ($B2) | Sets bit 0 of $B2 (RTS disabled) |
| A/D converter ($60-$67, $6D) | Battery voltage via AN0. ADMOD ($6D) starts conversion, ADREG ($60-$61) read by INTAD ISR |
| ADMOD ($6D) | Set to $04 (START bit) to begin A/D conversion during battery read |
| Custom I/O $80 | Written with $03,$02,$01,$00 during early init and INT0 handler. No known functional effect; not read back by the BIOS. MAME treats it as a plain register with no side effects |
| T6W28 registers ($A0, $A1) | Mutes all 4 channels ($9F, $BF, $DF, $FF) during sound init |
| T6W28 ($B8) | Disabled ($AA), then re-enabled ($55) during boot |
| Z80 ($B9) | Enabled ($55) during boot via $FF23E0 for BIOS sound driver, disabled ($AA) before cart handoff at step 43 |
| System RAM ($6C00-$6FFF) | Initializes all system state variables |
| System RAM ($4000-$6FFF) | Restored from NVRAM on warm boot |
| I/O registers ($80-$9F) | Restored from NVRAM on warm boot |

### During Runtime (System Calls)

| Component | Calls That Use It |
|-----------|-------------------|
| Clock gear register (unknown) | VECT_CLOCKGEARSET |
| RTC ($91-$96) | VECT_RTCGET, VECT_ALARMSET, VECT_ALARMDOWNSET |
| Flash controller | VECT_FLASHWRITE, VECT_FLASHALLERS, VECT_FLASHERS, VECT_FLASHPROTECT |
| K2GE mode ($87E2, $87F0) | VECT_GEMODESET |
| K2GE character RAM ($A000+) | VECT_SYSFONTSET |
| Interrupt priority ($70-$7A) | VECT_INTLVSET |
| Serial channel 1 | All VECT_COM* calls |
| Power state ($B6) | VECT_SHUTDOWN (writes $50) |

### During Interrupts (Ongoing)

| Component | Purpose |
|-----------|---------|
| Watchdog ($6F WDCR) | Serviced every VBlank |
| Input port ($B0) | Scanned and written to $6F82 |
| Power status ($B1) | Checked for power button state |
| A/D converter (AN0) | Battery voltage read to $6F80 |
| Power button (NMI via $B3) | Sets $6F85 shutdown flag |

---

## Sources

- BIOS ROM disassembly
- BIOS ROM execution tracing
- [Neo Geo Pocket Specification (devrs.com)](http://devrs.com/ngp/files/DoNotLink/ngpcspec.txt) -
  System call names and general descriptions
- [Neo Geo Pocket Technical Data (devrs.com)](https://www.devrs.com/ngp/files/ngpctech.txt) -
  Jump table addresses, partial communication call parameters
