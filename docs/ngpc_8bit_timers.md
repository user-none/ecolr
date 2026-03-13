# TMP95C061 8-Bit Timer System Reference

Reference documentation for the 8-bit timer subsystem (Timers 0-3) of the
TMP95C061, as used by the NGPC's TMP95CS64F SoC. All register layouts and
behavior are taken directly from the TMP95C061BDFG datasheet
(TMP95C061BDFG.pdf, section 3.8, pages 82-91).

For 16-bit timers (T4-T7), see
[ngpc_soc_peripherals.md](ngpc_soc_peripherals.md).

---

## Table of Contents

- [Overview](#overview)
- [Prescaler](#prescaler)
- [Up-Counter and Comparator](#up-counter-and-comparator)
- [Register Map](#register-map)
- [T01MOD - Timer 0/1 Mode Register ($24)](#t01mod---timer-01-mode-register-24)
- [T23MOD - Timer 2/3 Mode Register ($28)](#t23mod---timer-23-mode-register-28)
- [TRUN - Timer Run Control Register ($20)](#trun---timer-run-control-register-20)
- [TFFCR - Timer Flip-Flop Control Register ($25)](#tffcr---timer-flip-flop-control-register-25)
- [TRDC - Timer Register Double Buffer Control ($29)](#trdc---timer-register-double-buffer-control-29)
- [Timer Register Addresses](#timer-register-addresses)
- [Clock Source Summary](#clock-source-summary)
- [Timer Interrupts](#timer-interrupts)
- [Timer Flip-Flops and TO3](#timer-flip-flops-and-to3)
- [Clock Gear Interaction](#clock-gear-interaction)
- [NGPC BIOS Timer Configuration](#ngpc-bios-timer-configuration)
- [Known Code Issues](#known-code-issues)

---

## Overview

The TMP95C061 contains four 8-bit timers (T0, T1, T2, T3) organized as two
pairs: T0/T1 and T2/T3. Each pair shares a mode register and can operate in
several configurations:

- **Two independent 8-bit timers** (mode 00) - each timer clocked independently
- **One 16-bit timer** (mode 01) - even timer overflow cascades into odd timer
- **8-bit PPG output** (mode 10) - programmable pulse generation
- **8-bit PWM + 8-bit timer** (mode 11) - PWM on even timer, independent odd timer

Each timer has an 8-bit up-counter, an 8-bit compare register (TREGn), and
a comparator. Timer flip-flops (TFF1 for T0/T1, TFF3 for T2/T3) provide
output signals. The TO3 output (Timer 3 flip-flop) is used on the NGPC to
generate Z80 sound CPU interrupts.

Source: TMP95C061BDFG.pdf page 82

---

## Prescaler

The prescaler is a 9-bit free-running counter that generates the internal
clock taps used by all 8-bit and 16-bit timers.

### Block Diagram (from datasheet page 84)

```
                                    Tap Positions on 9-bit Prescaler
                                    --------------------------------
                                    Bit 0: oT0 (not used by 8-bit timers)
oscillation    fc     1/4           Bit 1: oT1  (fc/8)
 circuit  ---------> -----> 9-bit   Bit 2: oT2
                            prescaler
                                    Bit 3: oT4  (fc/32)
                      1/2           ...
                     -----> o1      Bit 5: oT8
                     -----> o2      Bit 6: oT16 (fc/128)
                                    Bit 7: oT32
                                    Bit 8: oT256 (fc/2048)
```

The prescaler input is **fc/4** where fc is the raw oscillator frequency.
The prescaler is NOT directly connected to the CPU clock output. It is
connected to the 1/4 divider output from the oscillation circuit.

### Prescaler Clock Taps

The datasheet describes prescaler taps using the notation "fc/N" where fc
refers to the oscillator frequency. The CPU state rate is fosc/2 (confirmed
by the TLCS-900/H instruction manual: 1 state = 100 ns at 20 MHz, 80 ns at
25 MHz). Because the prescaler is clocked from a divider chain off the
oscillator (not the CPU state clock), the tap divisors in CPU states are
larger than the raw "fc/N" values suggest.

At NGPC master clock = 6.144 MHz (CPU state rate = 6.144 MHz at gear 0):

| Clock | Datasheet | CPU State Divisor | Period          | Frequency  |
|-------|-----------|-------------------|-----------------|------------|
| oT1   | fc/8      | 128               | 20.833 us       | 48 kHz     |
| oT4   | fc/32     | 512               | 83.333 us       | 12 kHz     |
| oT16  | fc/128    | 2048              | 333.333 us      | 3 kHz      |
| oT256 | fc/2048   | 32768             | 5.333 ms        | 187.5 Hz   |

### Prescaler Control

The prescaler is started and stopped via TRUN bit 7 (PRRUN):
- PRRUN = 1: Prescaler runs (counting)
- PRRUN = 0: Prescaler stops and clears to zero

Source: TMP95C061BDFG.pdf page 84

---

## Up-Counter and Comparator

Each 8-bit timer has an up-counter that increments on each tick of its
selected clock source. The counter is compared against the timer register
(TREGn).

### Match Behavior

From the datasheet (page 85):

> "When the set value of timer registers TREG0, TREG1, TREG2, TREG3
> matches the value of up-counter, the comparator match detect signal
> becomes active. If the set value is 00H, this signal becomes active when
> the up-counter overflows."

This means:
- **TREG != 0**: Counter counts 0, 1, 2, ..., TREG. Match fires at TREG.
  Period = TREG + 1 ticks. Counter resets to 0 on the next tick.
- **TREG == 0**: Counter counts 0, 1, 2, ..., 255. Match fires on overflow
  (when counter wraps from 255 to 0). Period = 256 ticks.

### Counter Control

Each timer's counter can be independently started/stopped via TRUN bits 0-3.
When a timer's run bit transitions from 0 to 1, its counter is cleared to 0.
When a timer's run bit is 0, the counter is stopped and cleared.

Source: TMP95C061BDFG.pdf page 85

---

## Register Map

| Address | Register | R/W | Description |
|---------|----------|-----|-------------|
| $20     | TRUN     | R/W | Timer run control |
| $22     | TREG0    | W   | Timer 0 compare value |
| $23     | TREG1    | W   | Timer 1 compare value |
| $24     | T01MOD   | R/W | Timer 0/1 mode control |
| $25     | TFFCR    | R/W | Timer flip-flop control |
| $26     | TREG2    | W   | Timer 2 compare value |
| $27     | TREG3    | W   | Timer 3 compare value |
| $28     | T23MOD   | R/W | Timer 2/3 mode control |
| $29     | TRDC     | R/W | Timer register double buffer control |

Note: TREG0-TREG3 are write-only. The initial value is indeterminate.

Source: TMP95C061BDFG.pdf page 86

---

## T01MOD - Timer 0/1 Mode Register ($24)

```
  Bit:    7       6       5       4       3       2       1       0
       +-------+-------+-------+-------+-------+-------+-------+-------+
       | T01M1 | T01M0 | PWM01 | PWM00 | T1CLK1| T1CLK0| T0CLK1| T0CLK0|
       +-------+-------+-------+-------+-------+-------+-------+-------+
  R/W:    R/W     R/W     R/W     R/W     R/W     R/W     R/W     R/W
  Reset:   0       0       0       0       0       0       0       0
```

### Bit Fields

**Bits 1-0: T0CLK (Timer 0 Clock Source)**

| Value | Source                    |
|-------|---------------------------|
| 00    | External clock input TI0  |
| 01    | Internal clock oT1        |
| 10    | Internal clock oT4        |
| 11    | Internal clock oT16       |

**Bits 3-2: T1CLK (Timer 1 Clock Source)**

When T01M = 01 (16-bit mode), T1CLK is ignored; Timer 1 is clocked by
Timer 0 overflow.

| Value | Mode 00 (8-bit)              | Mode 01 (16-bit)       |
|-------|------------------------------|------------------------|
| 00    | Comparator output of Timer 0 | Overflow of Timer 0    |
| 01    | Internal clock oT1           | (16-bit timer mode)    |
| 10    | Internal clock oT16          |                        |
| 11    | Internal clock oT256         |                        |

**Bits 5-4: PWM0 (PWM Cycle Select)**

Don't care except in PWM mode.

| Value | PWM Cycle |
|-------|-----------|
| 00    | ---       |
| 01    | 2^6 - 1  |
| 10    | 2^7 - 1  |
| 11    | 2^8 - 1  |

**Bits 7-6: T01M (Operation Mode)**

| Value | Mode                                        |
|-------|---------------------------------------------|
| 00    | Two 8-bit timers (Timer 0 and Timer 1)      |
| 01    | 16-bit timer                                |
| 10    | 8-bit PPG output                            |
| 11    | 8-bit PWM output (Timer 0) + 8-bit (Timer 1)|

Source: TMP95C061BDFG.pdf page 87, Figure 3.8 (4)

---

## T23MOD - Timer 2/3 Mode Register ($28)

```
  Bit:    7       6       5       4       3       2       1       0
       +-------+-------+-------+-------+-------+-------+-------+-------+
       | T23M1 | T23M0 | PWM21 | PWM20 | T3CLK1| T3CLK0| T2CLK1| T2CLK0|
       +-------+-------+-------+-------+-------+-------+-------+-------+
  R/W:    R/W     R/W     R/W     R/W     R/W     R/W     R/W     R/W
  Reset:   0       0       0       0       0       0       0       0
```

### Bit Fields

**Bits 1-0: T2CLK (Timer 2 Clock Source)**

| Value | Source             |
|-------|--------------------|
| 00    | ---                |
| 01    | Internal clock oT1 |
| 10    | Internal clock oT4 |
| 11    | Internal clock oT16|

Note: Timer 2 has NO external clock input (unlike Timer 0 which has TI0).
Value 00 is listed as "---" (no clock / stopped).

**Bits 3-2: T3CLK (Timer 3 Clock Source)**

When T23M = 01 (16-bit mode), T3CLK is ignored; Timer 3 is clocked by
Timer 2 overflow.

| Value | Mode 00 (8-bit)               | Mode 01 (16-bit)       |
|-------|-------------------------------|------------------------|
| 00    | Comparator output of Timer 2  | Overflow of Timer 2    |
| 01    | Internal clock oT1            | (16-bit timer mode)    |
| 10    | Internal clock oT16           |                        |
| 11    | Internal clock oT256          |                        |

**Bits 5-4: PWM2 (PWM Cycle Select)** - Same encoding as T01MOD PWM0.

**Bits 7-6: T23M (Operation Mode)**

| Value | Mode                                        |
|-------|---------------------------------------------|
| 00    | Two 8-bit timers (Timer 2 and Timer 3)      |
| 01    | 16-bit timer                                |
| 10    | 8-bit PPG output                            |
| 11    | 8-bit PWM output (Timer 2) + 8-bit (Timer 3)|

Source: TMP95C061BDFG.pdf page 88, Figure 3.8 (5)

### Clock Source Comparison: Even vs Odd Timers

The even timers (T0, T2) and odd timers (T1, T3) have DIFFERENT clock
source encodings:

| Value | T0 Clock (bits 1-0)  | T1 Clock (bits 3-2)  | T2 Clock (bits 1-0)  | T3 Clock (bits 3-2)  |
|-------|----------------------|----------------------|----------------------|----------------------|
| 00    | TI0 (external)       | T0 cascade           | --- (stopped)        | T2 cascade           |
| 01    | oT1                  | oT1                  | oT1                  | oT1                  |
| 10    | oT4                  | oT16                 | oT4                  | oT16                 |
| 11    | oT16                 | oT256                | oT16                 | oT256                |

Key differences:
- Even timer value 00: T0=external pin, T2=stopped
- Odd timer value 00: cascade from even timer overflow
- Even timers use oT1/oT4/oT16 for values 01/10/11
- Odd timers use oT1/oT16/oT256 for values 01/10/11
- Odd timers do NOT have oT4 as an option; they have oT256 instead

---

## TRUN - Timer Run Control Register ($20)

```
  Bit:    7       6       5       4       3       2       1       0
       +-------+-------+-------+-------+-------+-------+-------+-------+
       | PRRUN |  ---  | T5RUN | T4RUN | T3RUN | T2RUN | T1RUN | T0RUN |
       +-------+-------+-------+-------+-------+-------+-------+-------+
  R/W:    R/W             R/W     R/W     R/W     R/W     R/W     R/W
  Reset:   0       0       0       0       0       0       0       0
```

| Bit | Name  | Description |
|-----|-------|-------------|
| 0   | T0RUN | Timer 0: 0=Stop & Clear, 1=Count |
| 1   | T1RUN | Timer 1: 0=Stop & Clear, 1=Count |
| 2   | T2RUN | Timer 2: 0=Stop & Clear, 1=Count |
| 3   | T3RUN | Timer 3: 0=Stop & Clear, 1=Count |
| 4   | T4RUN | 16-bit Timer 4/5: 0=Stop & Clear, 1=Count |
| 5   | T5RUN | 16-bit Timer 6/7: 0=Stop & Clear, 1=Count |
| 6   | ---   | Reserved |
| 7   | PRRUN | Prescaler: 0=Stop & Clear, 1=Count |

Source: TMP95C061BDFG.pdf page 90, Figure 3.8 (7)

---

## TFFCR - Timer Flip-Flop Control Register ($25)

```
  Bit:    7       6       5       4       3       2       1       0
       +-------+-------+-------+-------+-------+-------+-------+-------+
       | FF3C1 | FF3C0 | FF3IE | FF3IS | FF1C1 | FF1C0 | FF1IE | FF1IS |
       +-------+-------+-------+-------+-------+-------+-------+-------+
  R/W:    W       W      R/W     R/W      W       W      R/W     R/W
  Reset:  ---     ---      0       0      ---     ---      0       0
```

### TFF1 (Timer Flip-Flop 1, for T0/T1 pair)

| Bits | Field | Description |
|------|-------|-------------|
| 0    | FF1IS | Inversion source: 0=Timer 0, 1=Timer 1 |
| 1    | FF1IE | Inversion enable: 0=Disable, 1=Enable |
| 3-2  | FF1C  | Control: 00=Invert, 01=Set, 10=Clear, 11=Don't care |

### TFF3 (Timer Flip-Flop 3, for T2/T3 pair)

| Bits | Field | Description |
|------|-------|-------------|
| 4    | FF3IS | Inversion source: 0=Timer 2, 1=Timer 3 |
| 5    | FF3IE | Inversion enable: 0=Disable, 1=Enable |
| 7-6  | FF3C  | Control: 00=Invert, 01=Set, 10=Clear, 11=Don't care |

Note: FF3IS and FF3IE descriptions say "Don't care" except in 8-bit timer
mode. In non-8-bit modes, the flip-flop behavior is determined by the mode.

The FF1C and FF3C bits always read as 11 (Don't care).

Source: TMP95C061BDFG.pdf page 89, Figure 3.8 (6)

---

## TRDC - Timer Register Double Buffer Control ($29)

```
  Bit:    7       6       5       4       3       2       1       0
       +-------+-------+-------+-------+-------+-------+-------+-------+
       |  ---  |  ---  |  ---  |  ---  |  ---  |  ---  | TR2DE | TR0DE |
       +-------+-------+-------+-------+-------+-------+-------+-------+
  R/W:                                                    R/W     R/W
  Reset:                                                   0       0
```

| Bit | Name  | Description |
|-----|-------|-------------|
| 0   | TR0DE | TREG0 double buffer: 0=Disable, 1=Enable |
| 1   | TR2DE | TREG2 double buffer: 0=Disable, 1=Enable |

When double buffering is disabled (TRxDE=0), writes go directly to both the
timer register and the register buffer. When enabled (TRxDE=1), writes go
only to the register buffer, and the buffer is transferred to the timer
register on specific events (2^n-1 overflow in PWM mode, or PPG cycle match
in PPG mode).

Source: TMP95C061BDFG.pdf page 91, Figure 3.8 (8)

---

## Timer Register Addresses

| Register | Address | Size | R/W | Description |
|----------|---------|------|-----|-------------|
| TREG0    | $22     | Byte | W   | Timer 0 compare value |
| TREG1    | $23     | Byte | W   | Timer 1 compare value |
| TREG2    | $26     | Byte | W   | Timer 2 compare value |
| TREG3    | $27     | Byte | W   | Timer 3 compare value |

All TREG registers are write-only with indeterminate initial values.

Source: TMP95C061BDFG.pdf page 86

---

## Clock Source Summary

### Prescaler Tap Divisors (in CPU states)

| Tap   | Datasheet (fc/N) | CPU State Divisor |
|-------|------------------|-------------------|
| oT1   | fc/8             | 128               |
| oT4   | fc/32            | 512               |
| oT16  | fc/128           | 2048              |
| oT256 | fc/2048          | 32768             |

The datasheet "fc/N" notation uses fc = oscillator frequency. The CPU state
rate is fosc/2, and the prescaler is clocked from a divider chain off the
oscillator, resulting in the larger CPU state divisors shown above. See the
[Prescaler Clock Taps](#prescaler-clock-taps) section for derivation.

### Timer Period Calculation

For a timer clocked by prescaler tap oTx with TREG value R:

```
Period (in fc cycles) = oTx_divisor * (R + 1)
Period (in fc cycles) = oTx_divisor * 256        (when R = 0)
```

Example: Timer 3 with oT1 clock and TREG3 = $62 (98 decimal):
```
Period = 128 * (98 + 1) = 128 * 99 = 12,672 CPU states
```

---

## Timer Interrupts

Each 8-bit timer generates an interrupt on comparator match:

| Timer | Interrupt | Priority Register | Priority Bits | Vector Address |
|-------|-----------|-------------------|---------------|----------------|
| T0    | INTT0     | INTET01 ($73)     | Bits 2-0      | $FFFF40        |
| T1    | INTT1     | INTET01 ($73)     | Bits 6-4      | $FFFF44        |
| T2    | INTT2     | INTET23 ($74)     | Bits 2-0      | $FFFF48        |
| T3    | INTT3     | INTET23 ($74)     | Bits 6-4      | $FFFF4C        |

Source: TMP95C061BDFG.pdf page 15, Table 3.3 (1); page 23

---

## Timer Flip-Flops and TO3

### TO3 and Z80 Sound CPU Interrupts

On the NGPC, the Timer 3 flip-flop output (TFF3) drives the TO3 pin
(Port A3). This signal is routed to the Z80 sound CPU's INT input.
Rising edges of TO3 trigger Z80 interrupts.

The BIOS configures TFFCR so that TFF3 is inverted by Timer 3 match events
(FF3IS=1 for Timer 3 as source, FF3IE=1 to enable). Each Timer 3 overflow
toggles TFF3. Since Z80 INT is triggered on rising edges, each pair of
Timer 3 overflows generates one Z80 interrupt. The Z80 interrupt rate is
therefore half the Timer 3 overflow rate.

### Z80 Interrupt Rate Calculation

With BIOS configuration (TREG2=$90, TREG3=$62, T23MOD=$05):
- Timer 2: oT1 clock, TREG2 = $90 (144 decimal), period = 128 * 145 = 18,560 CPU states
- Timer 3: oT1 clock, TREG3 = $62 (98 decimal), period = 128 * 99 = 12,672 CPU states

Timer 3 overflow rate = 6,144,000 / 12,672 = 484.8 Hz
TO3 toggle rate = 484.8 Hz
Z80 INT rate (rising edges only) = 484.8 / 2 = 242.4 Hz

---

## Clock Gear Interaction

### TMP95C061 (Standard Part)

The TMP95C061 does NOT have a clock gear feature. The prescaler is driven
by fc/4 from the oscillation circuit (page 84 diagram). The CLK output pin
provides "System Clock / 4" (page 7). There is no gear divider in the
standard part.

### TMP95CS64F (NGPC Custom SoC)

The NGPC uses a custom TMP95CS64F which adds a clock gear divider. The
gear divides the CPU execution rate:

| Gear | Divisor | CPU Clock (fc=6.144 MHz) |
|------|---------|--------------------------|
| 0    | 1       | 6.144 MHz                |
| 1    | 2       | 3.072 MHz                |
| 2    | 4       | 1.536 MHz                |
| 3    | 8       | 768 kHz                  |
| 4    | 16      | 384 kHz                  |

### Prescaler is Independent of Clock Gear

The TMP95C061 prescaler diagram (page 84) shows the prescaler fed from the
oscillator 1/4 divider, separate from the CPU clock path. The prescaler
runs at a fixed rate from the oscillator regardless of the clock gear
setting. Timer periods are constant in real time regardless of CPU speed.

The emulator accounts for this by converting geared CPU cycles to ungeared
fc-scale before feeding the prescaler: `fcDelta = delta * (1 << clockGear)`.
This ensures the prescaler accumulates at the correct oscillator-derived
rate even when the CPU is running at a reduced clock gear.

---

## NGPC BIOS Timer Configuration

The BIOS configures the following timer registers during boot:

| Register | Value | Meaning |
|----------|-------|---------|
| TRUN     | $88   | PRRUN=1 (prescaler on), T3RUN=1 (Timer 3 on) |
| TREG2    | $90   | Timer 2 compare = 144 decimal |
| TREG3    | $62   | Timer 3 compare = 98 decimal |
| T23MOD   | $05   | T23M=00 (two 8-bit timers), PWM2=00, T3CLK=01 (oT1), T2CLK=01 (oT1) |

Decoding T23MOD = $05 = 0000_0101:
- Bits 1-0 (T2CLK) = 01 -> oT1
- Bits 3-2 (T3CLK) = 01 -> oT1
- Bits 5-4 (PWM2)  = 00 -> don't care
- Bits 7-6 (T23M)  = 00 -> two 8-bit timers

Both Timer 2 and Timer 3 are clocked by oT1 (128 CPU state divisor, 48 kHz).

Timer 2 is configured but not started (TRUN bit 2 is 0 in $88). Only
Timer 3 and the prescaler are running.

---

## Sources

- TMP95C061BDFG.pdf (in docs/) - Section 3.8 "8-bit Timers", pages 82-91
- TMP95C061BDFG.pdf - Section 3.3 "Interrupts", pages 12-15, 23
- TMP95C061BDFG.pdf - Pin descriptions, pages 6-7
