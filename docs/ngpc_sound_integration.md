# Neo Geo Pocket Color Sound System Integration

Technical reference for how the NGPC integrates its sound generators, routes
CPU control to them, and mixes their outputs into a final stereo signal.

For T6W28-specific hardware differences from the base SN76489, see
[section 5.1](#51-t6w28-psg-output). For base SN76489 tone/noise generation
internals, see the go-chip-sn76489 documentation.

---

## 1. System Audio Architecture

The Neo Geo Pocket Color produces audio from two independent sources:

| Source | Type | Channels | Output |
|--------|------|----------|--------|
| T6W28 PSG | Programmable sound generator (SN76489 variant) | 3 tone + 1 noise | Stereo (independent L/R volume per channel) |
| DAC pair | Digital-to-analog converters (6-bit hardware, 8-bit register) | 2 (left + right) | Stereo |

The T6W28 is controlled exclusively by the Z80 sound CPU. The DACs are
controlled exclusively by the TLCS-900H main CPU. Neither source is aware
of the other. Their outputs are mixed in hardware before reaching the
speaker or headphone output.

### Signal Flow

```
              +-----------+
   Z80 ------>|   T6W28   |---> PSG Left ----+
              |   (PSG)   |---> PSG Right ---+---> Mixing ---> Amplifier
              +-----------+                  |     Circuit     ---> Speaker
                                             |
              +-----------+                  |
  TLCS-900H ->| DAC Left  |---> DAC Left ----+
              | DAC Right |---> DAC Right ---+
              +-----------+
```

---

## 2. CPU Control

### 2.1 Dual-CPU Design

The NGPC has two CPUs that share responsibility for sound:

- **Toshiba TLCS-900H** (main CPU): Runs game logic. Manages the Z80
  lifecycle, uploads sound driver code, streams music data, and writes
  directly to the DAC registers. Clocked at 6.144 MHz.
- **Zilog Z80** (sound CPU): Dedicated to audio processing. Has direct
  access to the T6W28 PSG write ports. Clocked at 3.072 MHz (master / 2).

The division of labor is fixed by hardware constraints:

| CPU | Responsibility |
|-----|---------------|
| TLCS-900H | Z80 lifecycle control (enable/disable/reset), sound chip power, DAC sample output, streaming music data to shared RAM |
| Z80 | T6W28 PSG register programming, sound driver execution, real-time tone/noise generation |

The Z80 cannot access the DACs. The TLCS-900H does not directly access the
T6W28 PSG write ports.

### 2.2 Z80 as Sound Processor

The Z80 is **suspended on system reset** and does not execute until
explicitly started. Its interrupt line is also cleared.

The BIOS includes a built-in Z80 sound driver in the first 4KB of
BIOS ROM ($FF0000-$FF0FFF). During boot, $FF23E0 copies this driver
to Z80 shared RAM ($7000-$7FFF) and enables both the sound chip and
Z80 by writing $5555 to $B8 (little-endian: $B8=$55 sound on,
$B9=$55 Z80 on). The Z80 then plays music during the SNK logo and
startup animations. Before handing off to the cartridge (step 43),
the BIOS disables both with $B8=$AAAA ($B8=$AA, $B9=$AA) but does
not clear Z80 RAM. The BIOS sound driver remains at $7000-$7FFF
after cart handoff. Games may load their own sound driver or
re-enable the existing one.

The Z80 has 4KB of shared RAM for its sound driver code and data. A typical
game boot sequence is:

1. TLCS-900H writes the sound driver program into shared RAM ($7000-$7FFF)
2. TLCS-900H enables the sound chip by writing $55 to register $00B8
3. TLCS-900H enables the Z80 by writing $55 to register $00B9
4. Z80 is reset (all registers cleared, PC = $0000) and begins executing
   from address $0000 (which maps to the shared RAM)
5. TLCS-900H sends commands to the Z80 via the communication register

Enabling the Z80 always performs a reset before execution begins. This
ensures the Z80 starts from a clean state at $0000 regardless of any
prior state. Games that need to load a new sound driver can disable the
Z80, write new code to shared RAM, and re-enable it to start the new
driver from $0000.

To disable the Z80:
- Write $AA to register $00B9 (Z80 activation off)

To disable the sound chip:
- Write $AA to register $00B8 (sound chip activation off)

### 2.2.1 Z80 Initial Register State

On reset, the Z80 registers are initialized to the following values:

| Register | Value | Notes |
|----------|-------|-------|
| PC | $0000 | Standard Z80 reset vector |
| SP | $0000 | See note below |
| IX | $0000 | See note below |
| IY | $0000 | See note below |
| AF, BC, DE, HL | $0000 | |
| AF', BC', DE', HL' | $0000 | |
| I, R | $00 | |
| IFF1, IFF2 | 0 | Interrupts disabled |
| IM | 0 | Interrupt mode 0 |

The Z80 hardware reset only defines PC, I, R, IFF1, IFF2, and IM. The
remaining registers (SP, IX, IY, and the general purpose registers) are
architecturally undefined after reset. The values above (all zero) are
chosen for emulation purposes since no official documentation specifies
their post-reset state on the NGPC. In practice this does not affect
correct operation because the TLCS-900H loads a sound driver into shared
RAM before enabling the Z80, and the driver is expected to initialize SP
and any other registers it uses before relying on them.

### 2.3 Z80 Interrupt

The Z80 sound driver is interrupt-driven via two mechanisms:

**NMI:** Triggered by the TLCS-900H writing any value to I/O register
$00BA. This is not tied to VBlank or any hardware timer - the rate depends
entirely on how often the game's TLCS-900H code writes to $00BA. Each NMI,
the sound driver typically:

- Reads any pending commands from the communication interface
- Advances the music sequence
- Updates T6W28 PSG registers (tone frequencies, noise mode, volumes)

**IRQ:** The Timer 3 flip-flop output (TO3, which appears on port A bit 3)
is connected to the Z80 IRQ pin. The Z80 IRQ is asserted on the **rising
edge** of TO3 (0-to-1 transition). The IRQ rate depends on the TLCS-900H
timer configuration (T23MOD, TREG3, TRUN registers). This provides a
secondary timing mechanism for sound processing.

### 2.4 Z80-to-TLCS-900H Interrupt

The Z80 can trigger an interrupt on the TLCS-900H by writing any value to
Z80 address $C000. This fires the "Interrupt from Z80" handler at
TLCS-900H interrupt vector address $6FD0 (vector $0C).

This mechanism allows the Z80 to signal the main CPU when it needs
attention (e.g., requesting new music data, signaling a sound event).

### 2.5 Shared RAM Bus Priority

The TLCS-900H has priority over the Z80 on the shared RAM data bus. When
both CPUs attempt to access shared RAM simultaneously, the Z80 is stalled
until the TLCS-900H access completes. Frequent TLCS-900H accesses to the
$7000-$7FFF range can degrade Z80 performance.

---

## 3. Memory-Mapped I/O

### 3.1 TLCS-900H Address Map (Sound-Related)

| Address | Size | R/W | Function |
|---------|------|-----|----------|
| $007000-$007FFF | 4KB | R/W | Shared sound RAM (Z80 program + data) |
| $00A0 | Byte | W | T6W28 right channel write port (see gate condition below, corresponds to Z80 $4000) |
| $00A1 | Byte | W | T6W28 left channel write port (see gate condition below, corresponds to Z80 $4001) |
| $00A2 | Byte | W | DACL - DAC Left output (unsigned 8-bit, 0x80 = center) |
| $00A3 | Byte | W | DACR - DAC Right output (unsigned 8-bit, 0x80 = center) |
| $00B8 | Byte | W | Sound chip activation ($55 = on, $AA = off) |
| $00B9 | Byte | W | Z80 activation ($55 = on, $AA = off) |
| $00BA | Byte | W | Z80 NMI trigger (any write fires Z80 NMI) |
| $00BC | Byte | R/W | Z80 <-> TLCS-900H communication interface |

**T6W28 TLCS-900H write gate:** Writes to $00A0/$00A1 are only forwarded
to the T6W28 when both conditions are met: $00B8 == $55 (sound chip
enabled) AND $00B9 == $AA (Z80 disabled). If either condition is not
met, writes to these ports are ignored. When the Z80 is running
($B9 == $55), it accesses the T6W28 directly via its own address space
at $4000/$4001.

### 3.2 Z80 Address Map

| Address Range | R/W | Function |
|---------------|-----|----------|
| $0000-$0FFF | R/W | Shared sound RAM (4KB, maps to TLCS-900H $7000-$7FFF) |
| $1000-$3FFF | - | Unmapped (reads return 0, writes ignored) |
| $4000 | W | T6W28 right channel write port (reads return 0) |
| $4001 | W | T6W28 left channel write port (reads return 0) |
| $4002-$7FFF | - | Unmapped (reads return 0, writes ignored) |
| $8000 | R/W | Z80 <-> TLCS-900H communication interface |
| $8001-$BFFF | - | Unmapped (reads return 0, writes ignored) |
| $C000 | W | Write any value to trigger TLCS-900H interrupt (reads return 0) |
| $C001-$FFFF | - | Unmapped (reads return 0, writes ignored) |

All I/O addresses use exact 16-bit decoding with no mirroring. RAM is
decoded as $0000-$0FFF only and does not mirror into $1000-$3FFF.

### 3.3 T6W28 Write Ports

The T6W28 has two write ports, each with independent latch state:

| Z80 Address | Port | Volume Target | Frequency Target |
|-------------|------|---------------|-----------------|
| $4000 | Right | Right volume registers | Shared tone/noise frequency |
| $4001 | Left | Left volume registers | Shared tone/noise frequency |

Each port uses the standard SN76489 latch/data byte protocol:

| Bit 7 | Type | Bits 6-4 | Bits 3-0 |
|-------|------|----------|----------|
| 1 | Latch + data | `RR C` (register, channel) | Data (low 4 bits) |
| 0 | Data only | - | Data (6 bits, to last latched register) |

When a volume write arrives, it is routed to the left or right volume
register array based on which port ($4000 or $4001) received the write.
Tone and noise frequency writes affect the shared generators regardless of
which port is used.

### 3.4 DAC Registers

The two DACs provide direct digital audio output independent of the T6W28
PSG. Although the hardware DACs are 6-bit, software writes a full unsigned
8-bit value to each register. The center point for PCM waveforms is 0x80
(128); values above and below 0x80 represent positive and negative
displacement from the center.

| Register | Address | Bits | Function |
|----------|---------|------|----------|
| DACL | $00A2 | 7-0 | Left channel DAC output (unsigned 8-bit, 0x80 = center) |
| DACR | $00A3 | 7-0 | Right channel DAC output (unsigned 8-bit, 0x80 = center) |

The DACs are accessible only from the TLCS-900H. The Z80 cannot write to
them. Games use the DACs for digitized audio playback (voice samples,
ADPCM-decoded audio, sound effects) by writing sample values at a fixed
rate from the main CPU.

---

## 4. Clock Derivation and Timing

### 4.1 System Clocks

Both CPUs derive their clocks from a single master oscillator:

| Component | Clock | Derivation |
|-----------|-------|------------|
| Master oscillator | 6.144 MHz | Crystal |
| TLCS-900H | 6.144 MHz | Master (variable, can run as low as 384 kHz) |
| Z80 | 3.072 MHz | Master / 2 |
| Timer prescaler input | 1.536 MHz | Master / 4 (feeds prescaler divider chain) |

The NGPC is a handheld system with a single clock domain. There is no
NTSC/PAL distinction - all units run at the same frequency.

### 4.2 T6W28 Timing

The T6W28 is clocked at the Z80 rate and applies a /16 internal divider:

```
Z80 clock (3.072 MHz) --> /16 internal divider --> tone counter input
                           (192,000 Hz)
```

The tone counter divides further by the programmed period value to produce
the square wave output. The noise channel uses a similar mechanism with an
LFSR for pseudo-random output.

### 4.3 Display Timing

| Parameter | Value |
|-----------|-------|
| Scanline duration | 515 TLCS-900H clocks (~83.83 us) |
| Frame rate | ~59.95 Hz |
| Scanline time | 515 / 6,144,000 = ~83.83 us |

### 4.4 Samples Per Frame

At 44.1 kHz output sample rate:

| Parameter | Value |
|-----------|-------|
| Samples per frame | ~735 |
| Frames per second | ~59.95 |

---

## 5. Output Characteristics

### 5.1 T6W28 PSG Output

- **Format:** Stereo (independent left/right volume per channel)
- **Channels:** 3 square-wave tone generators + 1 noise generator
- **Volume:** 4-bit per channel per side (16 levels, ~2 dB per step)
- **Stereo model:** Each of the 4 channels has independent left and right
  volume registers, allowing full stereo positioning through volume
  differences

The T6W28 differs from the Game Gear PSG stereo model. The Game Gear uses
a binary on/off panning bitmask per channel. The T6W28 provides independent
4-bit volume control per channel per side, allowing smooth stereo panning
rather than just left/right/both selection.

#### T6W28 Hardware Differences from Base SN76489

The T6W28 shares tone/noise generation with the Sega SN76489 variant but
differs in the following ways:

| Feature | SN76489 (Sega) | T6W28 |
|---------|----------------|-------|
| Write ports | Single port | Two ports ($4000 right, $4001 left) with independent latch state per port |
| Volume registers | One set (4 channels) | Two sets: left and right (4 channels each) |
| Volume write routing | Always to the single register set | Routed to left or right set based on which port received the write |
| Tone/noise frequency | Single port writes | Shared across both ports (frequency writes from either port affect the same generators) |
| LFSR initial seed | 0x8000 | 0xFFFE (hardware-verified) |
| LFSR feedback taps | Bits 0,3 (0x0009) | Bits 0,3 (0x0009) - same as Sega |
| LFSR width | 16-bit | 16-bit - same as Sega |
| Noise rate 3 source | Tone channel 2 frequency | Tone channel 2 frequency - same as Sega |
| Output | Mono (single mixed output) | Stereo (independent left/right mix from separate volume registers) |

The stereo write port behavior, independent latch state, and volume routing
are documented in [section 3.3](#33-t6w28-write-ports). The LFSR seed
difference (0xFFFE vs 0x8000) was hardware-verified via oscilloscope
recordings of the noise channel output.

### 5.2 DAC Output

- **Format:** Stereo (independent left and right registers)
- **Resolution:** Hardware is 6-bit per channel, but software uses the full
  unsigned 8-bit register range (0x00-0xFF) with 0x80 as the center/silence
  point
- **Control:** TLCS-900H only (not accessible from Z80)
- **Typical usage:** PCM sample playback, ADPCM-decoded audio

### 5.3 DAC Sample Playback

The TLCS-900H handles all DAC-based audio playback directly. The playback
rate is entirely game-specific - there is no BIOS-configured default. A
typical approach:

1. Game code decodes or reads PCM sample data from ROM
2. TLCS-900H writes sample values to DACL ($00A2) and DACR ($00A3) at a
   timer-driven rate
3. Timer 2 is the commonly used timer for DAC timing
4. Micro DMA (HDMA) can be configured with Timer 2's interrupt vector
   ($12) as the start trigger, causing automatic memory-to-I/O transfers
   that stream sample data to the DAC register without per-sample CPU
   involvement

DAC values are updated immediately on each CPU write using a
sample-and-hold approach (the last written value is held until the next
write), which matches the hardware behavior. Common playback rates are
in the range of 8-16 kHz, though the rate varies by title.

---

## 6. Mixing

### 6.1 Hardware Mixing

On real hardware, mixing is analog:

1. The T6W28 outputs stereo audio (separate left and right)
2. The DAC pair outputs stereo audio (separate left and right)
3. Both stereo signals are mixed on the board
4. The combined signal goes to the amplifier and then to the speaker or
   headphone output

### 6.2 Emulation Mixing

For emulation, the PSG and DAC outputs are mixed digitally:

1. T6W28 generates stereo sample pairs (left, right) at the output sample
   rate
2. DAC values are converted to the same sample format
3. Both sources are summed per-channel (PSG left + DAC left, PSG right +
   DAC right)
4. The mixed stereo buffer is delivered to the audio output system

The relative volume balance between PSG and DAC is determined by the
hardware mixing circuit. The exact attenuation values on the NGPC board
have not been precisely characterized in available documentation.

---

## 7. TLCS-900H to Z80 Communication

### 7.1 Communication Register

The primary communication mechanism is a shared byte:

| CPU | Address | Access |
|-----|---------|--------|
| TLCS-900H | $00BC | Read/Write |
| Z80 | $8000 | Read/Write |

These map to a single shared physical register. Both CPUs read and write
the same byte. There is no contention handling, handshake mechanism, or
additional communication registers. Writing to $00BC does not trigger any
Z80 action - the Z80 NMI is triggered separately by writing to $00BA.
Writes are asynchronous and do not stall either CPU.

### 7.2 Typical Communication Pattern

1. TLCS-900H writes a command byte to $00BC (e.g., play track, stop music,
   play sound effect)
2. Z80 reads the command from $8000 during its next NMI handler
3. Z80 processes the command and updates PSG registers accordingly
4. Z80 can signal completion or request data by writing to $8000 and
   triggering an interrupt via $C000

### 7.3 Music Data Streaming

Because the Z80 only has 4KB of shared RAM and cannot access ROM directly,
music data must be streamed from the TLCS-900H:

1. TLCS-900H reads music sequence data from cartridge ROM
2. TLCS-900H writes portions of music data into shared RAM ($7000-$7FFF)
3. Z80 reads the data from its local view ($0000-$0FFF) and sequences it
4. The process repeats as the Z80 consumes data

This streaming model is necessary because the Z80 address space is limited
to the 4KB shared RAM window and a few I/O registers. Unlike the Genesis
where the Z80 can bank-switch into the 68000 address space, the NGPC Z80
has no access to ROM.

---

## 8. Initialization

### 8.1 Power-On Defaults

At power-on or reset, the sound system should be initialized to:

| Setting | Default |
|---------|---------|
| Sound chip activation ($00B8) | Off ($AA) |
| Z80 activation ($00B9) | Off ($AA) |
| T6W28 all channels | Maximum attenuation (silent, volume = $0F) |
| DACL ($00A2) | 0 (silent) |
| DACR ($00A3) | 0 (silent) |
| Z80 | Not running |

### 8.2 Typical Game Boot Sequence

1. BIOS performs initial hardware setup
2. TLCS-900H writes sound driver code into shared RAM ($7000-$7FFF)
3. TLCS-900H enables sound chip ($00B8 = $55)
4. TLCS-900H enables Z80 ($00B9 = $55)
5. Z80 begins executing sound driver from $0000
6. Z80 initializes T6W28 (silence all channels)
7. TLCS-900H sends "play music" command via communication register ($00BC)
8. Z80 begins sequencing music and programming T6W28 each NMI

---

## 9. Timing and Synchronization

### 9.1 Per-Scanline Execution

Audio processing is synchronized with display scanlines:

1. TLCS-900H executes 515 clocks per scanline
2. Z80 executes ~257 clocks per scanline (515 / 2, when not bus-stalled)
3. T6W28 advances by the Z80 cycle count, generating PSG samples
4. DAC values are updated by the TLCS-900H as needed

### 9.2 Frame Boundary

At the end of each frame:

1. T6W28 buffer contains stereo sample pairs for the frame
2. DAC buffer contains stereo sample pairs for the frame
3. Both buffers are mixed into a final stereo output buffer
4. The mixed buffer is delivered to the audio output system

### 9.3 Resampling

The T6W28 generates audio derived from its 3.072 MHz clock. The PSG
library handles resampling from this native rate to the output sample rate
internally.

DAC samples are written at a software-determined rate (commonly 16 kHz)
and must be upsampled to match the output sample rate. A simple
sample-and-hold approach (repeat the last written value until the next
write) is appropriate since this matches the hardware behavior.

---

---

## Sources

- [Neo Geo Pocket Technical Data (devrs.com)](https://www.devrs.com/ngp/files/ngpctech.txt) - Memory map, Z80 control registers, interrupt vectors
- [Neo Geo Pocket Specification (devrs.com)](http://devrs.com/ngp/files/DoNotLink/ngpcspec.txt) - CPU clocks, timer system, display timing, Z80 RAM
- [SNK Neo Geo Pocket Hardware Information (Data Crystal)](https://datacrystal.tcrf.net/wiki/SNK_Neo_Geo_Pocket/Hardware_information) - System specifications, memory sizes
- [Neo Geo Pocket Color (Game Tech Wiki)](https://www.gametechwiki.com/w/index.php/Neo_Geo_Pocket_Color) - Hardware overview, sound chip identification
- [NGPC Sound (jiggawatt.org)](http://jiggawatt.org/badc0de/ngpcsound.html) - Sound register details, DAC documentation
- [Furnace Tracker Issue #2660](https://github.com/tildearrow/furnace/issues/2660) - Hardware-verified T6W28 LFSR behavior
- [NGPC BIOS Sound Recording](https://www.youtube.com/watch?v=uZLQF7u5t9k) - Reference BIOS sound recorded from real hardware
