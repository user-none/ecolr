# K2GE Video Display Controller Technical Reference

Technical reference for the K2GE (K2 Graphics Engine) video display controller
used in the Neo Geo Pocket Color, including K1GE (K1 Graphics Engine)
compatibility mode for running original Neo Geo Pocket monochrome software.

---

## Table of Contents

- [Chip Identification](#chip-identification)
- [System Integration](#system-integration)
- [Display Characteristics](#display-characteristics)
- [Memory Map](#memory-map)
- [Registers](#registers)
- [Character RAM](#character-ram)
- [Scroll Planes](#scroll-planes)
- [Sprite System](#sprite-system)
- [Color System](#color-system)
- [K1GE Compatibility Mode](#k1ge-compatibility-mode)
- [Window System](#window-system)
- [Layer Priority and Compositing](#layer-priority-and-compositing)
- [Display Timing](#display-timing)
- [Interrupts](#interrupts)
- [LED Control](#led-control)
- [Character Overflow](#character-overflow)
- [Programming Constraints](#programming-constraints)
- [Sources](#sources)

---

## Chip Identification

| Chip | System | Description |
|------|--------|-------------|
| K1GE | Neo Geo Pocket (monochrome, 1998) | Monochrome graphics engine, grayscale palette |
| K2GE | Neo Geo Pocket Color (1999) | Color graphics engine, superset of K1GE |

The K2GE is functionally a superset of the K1GE. It supports full 12-bit
color mode and provides a K1GE backward-compatibility mode for running
monochrome NGP software on the color hardware. The K1GE compatibility mode
is controlled by a locked register that the BIOS manages.

---

## System Integration

The K2GE is controlled exclusively by the TLCS-900H main CPU. It occupies a
16 KB region of the TLCS-900H address space from $8000 to $BFFF.

| Component | Clock | Derivation |
|-----------|-------|------------|
| Master oscillator | 6.144 MHz | Crystal |
| TLCS-900H | 6.144 MHz | Master |
| K2GE | 6.144 MHz | Master (shared) |

The K2GE uses a line buffer rendering method. It reads sprite and scroll
plane data from VRAM, determines visibility, and composites pixels into a
line buffer for each scanline. This is distinct from a framebuffer approach.

---

## Display Characteristics

| Parameter | Value |
|-----------|-------|
| Visible resolution | 160 x 152 pixels |
| Virtual scroll area | 256 x 256 pixels per scroll plane |
| Tile size | 8 x 8 pixels |
| Bits per pixel | 2 (4 color levels per tile, including transparent) |
| Colors per tile/sprite | 4 (including color index 0 as transparent) |
| Maximum simultaneous colors (K2GE) | 146 (from 4,096 possible) |
| Maximum simultaneous shades (K1GE) | ~20 (from 8 possible) |
| Scroll planes | 2 |
| Maximum sprites | 64 |
| Character tiles | 512 (9-bit index) |
| Frame rate | ~59.95 Hz |

---

## Memory Map

The K2GE occupies $8000-$BFFF in the TLCS-900H address space.

| Address Range | Size | Contents |
|---------------|------|----------|
| $8000-$87FF | 2 KB | Control registers, palettes, LED control |
| $8800-$88FF | 256 bytes | Sprite attribute table (64 sprites x 4 bytes) |
| $8C00-$8C3F | 64 bytes | Sprite color palette assignments (K2GE mode only) |
| $9000-$97FF | 2 KB | Scroll Plane 1 tile map (32 x 32 entries x 2 bytes) |
| $9800-$9FFF | 2 KB | Scroll Plane 2 tile map (32 x 32 entries x 2 bytes) |
| $A000-$BFFF | 8 KB | Character RAM (512 tiles x 16 bytes) |

Total video-accessible memory: approximately 14.5 KB of usable data areas
(registers, sprite attributes, tile maps, and character patterns).

---

## Registers

### $8000 - Interrupt Control

| Bit | Name | Function |
|-----|------|----------|
| 7 | VBLK_EN | VBlank interrupt enable |
| 6 | HBLK_EN | HBlank interrupt enable |
| 5-0 | -- | Not used |

### $8002 - Window Horizontal Origin (WBA.H)

All 8 bits define the horizontal pixel origin of the display window.
Reset value: 0.

### $8003 - Window Vertical Origin (WBA.V)

All 8 bits define the vertical pixel origin of the display window.
Reset value: 0.

### $8004 - Window Horizontal Size (WSI.H)

All 8 bits define the horizontal pixel size of the display window.
Reset value: $FF.

Constraint: WBA.H + WSI.H must not exceed 160.

### $8005 - Window Vertical Size (WSI.V)

All 8 bits define the vertical pixel size of the display window.
Reset value: $FF.

Constraint: WBA.V + WSI.V must not exceed 152.

### $8006 - Frame Rate (REF)

Read-only. Reset value: $C6 (198 decimal). This register controls the
blanking period and determines frame timing. It is locked and must not be
written by application software.

### $8008 - Raster Position Horizontal (RAS.H)

Read-only. Returns a value derived from the remaining clocks in the current
scanline. The value is computed as `(clocks_remaining_in_scanline) >> 2`.
At the start of a scanline, the value is approximately 128 ($80). It
decreases toward 0 as the scanline progresses. This provides an
approximate 8-bit horizontal position indicator based on the 515-clock
scanline period. Readable during VBlank.

### $8009 - Raster Position Vertical (RAS.V)

Read-only. Returns the current vertical line number (0-198). Unlike a CRT
scan line, this represents the internal video chip signal position.

### $8010 - 2D Status

| Bit | Name | Function |
|-----|------|----------|
| 7 | C.OVR | Character overflow flag (cleared at end of VBlank) |
| 6 | BLNK | Blanking status: 0 = active display, 1 = VBlank |
| 5-0 | -- | Not used, read as 0 |

### $8012 - 2D Control

| Bit | Name | Function |
|-----|------|----------|
| 7 | NEG | Display inversion: 0 = normal, 1 = inverted RGB output |
| 6-3 | -- | Reserved |
| 2-0 | OOWC | Outside window color index (selects from window palette at $83F0) |

Both NEG and OOWC take effect on the next line drawn, enabling per-scanline
changes. NEG applies bitwise NOT (one's complement) to the 12-bit palette
value (each 4-bit channel `n` becomes `15 - n`). NEG affects both K2GE
color mode and K1GE compatibility mode.

Reset value: 0.

### $8020 - Sprite Position Offset Horizontal (PO.H)

All 8 bits define a global horizontal offset added to all sprite positions.
Final sprite H = H.P + PO.H. Reset value: 0.

### $8021 - Sprite Position Offset Vertical (PO.V)

All 8 bits define a global vertical offset added to all sprite positions.
Final sprite V = V.P + PO.V. Reset value: 0.

### $8030 - Scroll Priority

| Bit | Name | Function |
|-----|------|----------|
| 7 | P.F | 0 = Scroll Plane 1 in front, 1 = Scroll Plane 2 in front |
| 6-0 | -- | Not used |

Takes effect on the next line drawn. Reset value: 0 (Plane 1 in front).

### $8032 - Scroll Plane 1 X Offset (S1SO.H)

All 8 bits define the horizontal scroll offset for Scroll Plane 1.
A positive value moves the viewport to the right (tiles shift left on
screen). Takes effect on the next line drawn. Reset value: 0.

### $8033 - Scroll Plane 1 Y Offset (S1SO.V)

All 8 bits define the vertical scroll offset for Scroll Plane 1.
A positive value moves the viewport downward (tiles shift up on screen).
Takes effect on the next line drawn. Reset value: 0.

### $8034 - Scroll Plane 2 X Offset (S2SO.H)

All 8 bits define the horizontal scroll offset for Scroll Plane 2.
A positive value moves the viewport to the right (tiles shift left on
screen). Takes effect on the next line drawn. Reset value: 0.

### $8035 - Scroll Plane 2 Y Offset (S2SO.V)

All 8 bits define the vertical scroll offset for Scroll Plane 2.
A positive value moves the viewport downward (tiles shift up on screen).
Takes effect on the next line drawn. Reset value: 0.

### $8118 - Background Selection

| Bit | Name | Function |
|-----|------|----------|
| 7-6 | BGON | Background enable: bits 7-6 must be exactly $80 (bit 7 set, bit 6 clear) to enable. All other values disable (black). Applies in both K2GE color and K1GE mono modes. |
| 5-3 | -- | Reserved |
| 2-0 | BGC | Background color palette index (selects from BG palette at $83E0) |

Takes effect on the next line drawn. Reset value: 0 (disabled).

### $87E0 - 2D Software Reset

Write-only. Writing $52 triggers a 2D graphics engine reset.

Note: BIOS disassembly shows the BIOS writes $53 then $47 to this
register during boot initialization (between the $87F0 unlock/lock
sequence). The purpose of these values is unknown - they do not match
the documented $52 reset trigger. The register may accept additional
command values beyond what has been documented.

### $87E2 - Mode Selection

| Bit | Name | Function |
|-----|------|----------|
| 7 | MODE | 0 = K2GE color mode, 1 = K1GE upper palette compatible mode |
| 6-0 | -- | Not used, read as 0 |

This register is locked. Write access is controlled by $87F0. Application
software must use the BIOS system call VECT_GEMODESET to change modes.

### $87F0 - Mode Register Access Control

Write-only. Writing $AA enables writes to $87E2. Writing $55 disables
writes to $87E2.

---

## Character RAM

### Organization ($A000-$BFFF)

Character RAM holds up to 512 tile patterns. Each tile is 8x8 pixels at
2 bits per pixel, consuming 16 bytes per tile.

| Parameter | Value |
|-----------|-------|
| Total size | 8 KB |
| Tiles | 512 maximum |
| Tile dimensions | 8 x 8 pixels |
| Bits per pixel | 2 |
| Colors per tile | 4 (index 0 = transparent, indices 1-3 = opaque) |
| Bytes per tile | 16 |

### Tile Pixel Format

Each row of an 8-pixel tile is stored as a 16-bit little-endian word
(2 bytes). The byte at the lower address is the low byte (bits 7-0 of the
16-bit value) and the byte at the higher address is the high byte (bits
15-8).

Pixels are packed 2 bits each, with the leftmost screen pixel in the
highest bits:

| Screen Pixel | 16-bit Value Bits | Byte |
|---|---|---|
| Pixel 0 (leftmost) | 15:14 | High byte, bits 7:6 |
| Pixel 1 | 13:12 | High byte, bits 5:4 |
| Pixel 2 | 11:10 | High byte, bits 3:2 |
| Pixel 3 | 9:8 | High byte, bits 1:0 |
| Pixel 4 | 7:6 | Low byte, bits 7:6 |
| Pixel 5 | 5:4 | Low byte, bits 5:4 |
| Pixel 6 | 3:2 | Low byte, bits 3:2 |
| Pixel 7 (rightmost) | 1:0 | Low byte, bits 1:0 |

Within each byte, the high bits (7:6) map to the leftmost pixel of that
byte's group of 4, and the low bits (1:0) map to the rightmost. Each
2-bit value is a palette color index (0-3, where 0 is transparent).

In terms of raw byte layout in memory:

```
Byte +0 (low address):  right 4 pixels of the row (pixels 4-7)
Byte +1 (high address): left 4 pixels of the row (pixels 0-3)
```

A full 8x8 tile occupies 16 consecutive bytes (2 bytes per row x 8 rows).

### Tile Indexing

Tiles are addressed with a 9-bit index (0-511). The lower 8 bits come from
the main character code field in sprite and scroll attributes, and bit 8
comes from the control/attribute byte. The VRAM address for tile N is
$A000 + (N x 16).

Character RAM is shared by sprites and both scroll planes. All three layers
reference the same tile patterns.

---

## Scroll Planes

### Overview

The K2GE provides two independent scroll planes:

| Plane | Tile Map Address | Size |
|-------|-----------------|------|
| Scroll Plane 1 | $9000-$97FF | 2 KB |
| Scroll Plane 2 | $9800-$9FFF | 2 KB |

Each plane is a 32x32 grid of tile entries covering a 256x256 pixel virtual
area. The visible 160x152 pixel window scrolls over this virtual area using
the scroll offset registers.

### Tile Map Entry Format (2 bytes per entry)

| Byte | Bits | Field | Description |
|------|------|-------|-------------|
| +0 | 7-0 | C.C[7:0] | Character tile number, lower 8 bits |
| +1 | 7 | H.F | Horizontal flip (0 = normal, 1 = flipped) |
| +1 | 6 | V.F | Vertical flip (0 = normal, 1 = flipped) |
| +1 | 5 | P.C | Palette code (K1GE compatibility mode: selects 1 of 2 palettes) |
| +1 | 4-1 | CP.C | Color palette code (K2GE mode: selects 1 of 16 palettes, 0-15) |
| +1 | 0 | C.C[8] | Character tile number, upper bit |

In K2GE color mode, CP.C (bits 4-1) selects a color palette. Scroll Plane 1
uses palettes from $8280-$82FF and Scroll Plane 2 uses palettes from
$8300-$837F.

In K1GE compatibility mode, P.C (bit 5) selects between 2 monochrome
palettes. CP.C bits are not used.

### Tile Map Layout

Entries are stored in row-major order. The entry for tile at column X, row Y
is at: `base + (Y * 32 + X) * 2`.

### Scrolling

Each plane has independent 8-bit X and Y scroll offset registers:

| Register | Plane | Axis |
|----------|-------|------|
| $8032 | Scroll Plane 1 | Horizontal |
| $8033 | Scroll Plane 1 | Vertical |
| $8034 | Scroll Plane 2 | Horizontal |
| $8035 | Scroll Plane 2 | Vertical |

All scroll offsets take effect on the next line drawn. This enables
per-scanline scroll effects (raster effects) when modified during HBlank
or via interrupt handlers.

The offset values move the viewport over the tile map. During rendering,
the X offset is subtracted from each tile's screen position, and the Y
offset is added to the scanline to determine the tile map row. This means
a positive X offset scrolls the view to the right and a positive Y offset
scrolls the view downward.

The scroll planes wrap at the 256-pixel virtual boundary in both axes.

### Scroll Plane Priority

Register $8030 bit 7 (P.F) determines which plane is rendered in front:
- P.F = 0: Scroll Plane 1 in front of Scroll Plane 2
- P.F = 1: Scroll Plane 2 in front of Scroll Plane 1

This also takes effect on the next line drawn.

---

## Sprite System

### Overview

| Parameter | Value |
|-----------|-------|
| Maximum sprites | 64 |
| Per-scanline limit | 64 (no lower hardware limit) |
| Sprite size | 8 x 8 pixels (fixed) |
| Colors per sprite | 4 (2bpp, including transparent index 0) |
| Sprite attribute table | $8800-$88FF (256 bytes) |
| Sprite palette assignments | $8C00-$8C3F (64 bytes, K2GE mode only) |

### Sprite Attribute Table ($8800-$88FF)

Each sprite occupies 4 bytes. Sprite N is at $8800 + (N x 4).

| Offset | Bits | Field | Description |
|--------|------|-------|-------------|
| +0 | 7-0 | C.C[7:0] | Character tile number, lower 8 bits |
| +1 | 7 | H.F | Horizontal flip (0 = normal, 1 = flipped) |
| +1 | 6 | V.F | Vertical flip (0 = normal, 1 = flipped) |
| +1 | 5 | P.C | Palette code (K1GE compat mode: selects 1 of 2 palettes) |
| +1 | 4-3 | PR.C | Priority code (see below) |
| +1 | 2 | H.ch | Horizontal chain (0 = absolute, 1 = offset from previous sprite) |
| +1 | 1 | V.ch | Vertical chain (0 = absolute, 1 = offset from previous sprite) |
| +1 | 0 | C.C[8] | Character tile number, upper bit |
| +2 | 7-0 | H.P | Horizontal position (or chain offset) |
| +3 | 7-0 | V.P | Vertical position (or chain offset) |

### Priority Codes (PR.C)

| PR.C | Priority |
|------|----------|
| 00 | Hidden (sprite not displayed) |
| 01 | Behind both scroll planes (only in front of background) |
| 10 | Between scroll planes |
| 11 | In front of both scroll planes |

### Sprite Color Palette Assignment ($8C00-$8C3F, K2GE mode only)

In K2GE color mode, each sprite has an additional palette assignment byte
at $8C00 + N (where N is the sprite index 0-63):

| Bits | Field | Description |
|------|-------|-------------|
| 7-4 | -- | Not used, read as 0 |
| 3-0 | CP.C | Color palette code (0-15, selects from sprite palettes at $8200-$827F) |

This area is not used in K1GE compatibility mode.

### Sprite Position Offsets

Global offsets are applied to all sprite positions:

| Register | Function |
|----------|----------|
| $8020 (PO.H) | Added to all sprite horizontal positions |
| $8021 (PO.V) | Added to all sprite vertical positions |

Final position: H = H.P + PO.H, V = V.P + PO.V

No additional hardware pixel offset is applied beyond the OAM position and
the global offset registers. Coordinates use unsigned 8-bit values with
wrap-around: values above 248 and below 256 are treated as negative
(wrapped to the left/top edge of the screen).

### Sprite Chaining

When H.ch = 1, the sprite's H.P field is treated as a signed offset from
the previous sprite's horizontal position rather than an absolute screen
coordinate. Similarly, when V.ch = 1, V.P is a signed offset from the
previous sprite's vertical position.

This allows building multi-tile composite objects where moving the lead
sprite moves the entire group. Each chain dimension is independent - a
sprite can chain horizontally but use absolute vertical positioning, or
vice versa.

Sprite 0 always uses absolute positioning regardless of chain flags.

---

## Color System

### K2GE Color Mode

The K2GE supports 12-bit color: 4 bits per channel (red, green, blue),
providing 4,096 possible colors.

#### Color Palette RAM ($8200-$83FF)

Palette RAM must be accessed using 16-bit (word) reads and writes only.
8-bit access produces unreliable values. Palette RAM access is always
0-wait, even during the hardware drawing period.

Each palette entry is 16 bits:

```
Bit 15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
  --   --   --   --  B3  B2  B1  B0  G3  G2  G1  G0  R3  R2  R1  R0
```

| Bits | Field | Description |
|------|-------|-------------|
| 15-12 | -- | Not used, read as 0 |
| 11-8 | B_PLT | Blue (0-15) |
| 7-4 | G_PLT | Green (0-15) |
| 3-0 | R_PLT | Red (0-15) |

#### Palette Organization

Each palette contains 4 color entries (matching the 2bpp tile format).
Color index 0 is transparent in all palettes.

| Address Range | Entries | Palettes | Purpose |
|---------------|---------|----------|---------|
| $8200-$827F | 64 | 16 x 4 colors | Sprite palettes |
| $8280-$82FF | 64 | 16 x 4 colors | Scroll Plane 1 palettes |
| $8300-$837F | 64 | 16 x 4 colors | Scroll Plane 2 palettes |
| $8380-$839F | 16 | 4 x 4 colors | K1GE compat sprite palettes |
| $83A0-$83BF | 16 | 4 x 4 colors | K1GE compat Scroll Plane 1 palettes |
| $83C0-$83DF | 16 | 4 x 4 colors | K1GE compat Scroll Plane 2 palettes |
| $83E0-$83EF | 8 | -- | Background color |
| $83F0-$83FF | 8 | -- | Window/outside-window color |

#### Maximum Simultaneous Colors

48 palettes (16 sprite + 16 scroll 1 + 16 scroll 2) x 3 non-transparent
colors per palette + 1 background + 1 window = up to 146 simultaneous
colors from the 4,096 color space.

#### 4-Bit to 8-Bit Color Conversion

Each 4-bit channel value (0-15) maps to an 8-bit output using linear
scaling:

```
output = (value << 4) | value    (equivalent to value * 17)
```

Maps 0 to 0, 15 to 255 ($FF). No gamma correction or nonlinear curve
is applied.

---

## K1GE Compatibility Mode

When the K2GE runs in K1GE compatibility mode ($87E2 bit 7 = 1), it
emulates the monochrome K1GE's palette behavior.

### Monochrome Palette LUT ($8100-$8117)

The K1GE palette area stores 3-bit shade values (0-7) for each palette
entry.

| Address Range | Entries | Purpose |
|---------------|---------|---------|
| $8100-$8103 | 4 | Sprite palette 0 ($8100 = color 0, transparent) |
| $8104-$8107 | 4 | Sprite palette 1 ($8104 = color 0, transparent) |
| $8108-$810B | 4 | Scroll Plane 1 palette 0 |
| $810C-$810F | 4 | Scroll Plane 1 palette 1 |
| $8110-$8113 | 4 | Scroll Plane 2 palette 0 |
| $8114-$8117 | 4 | Scroll Plane 2 palette 1 |

Each entry is a single byte. Only bits 2-0 are significant (shade level
0-7). Bits 7-3 are unmapped and read as 0.

### K1GE Shade to Grayscale Mapping

No official documentation specifies the exact grayscale values for the 8
shade levels. A linear mapping using 255/7 per step is recommended, which
produces evenly-spaced levels spanning the full 0-255 range:

| Shade | Grayscale (8-bit) | Description |
|-------|-------------------|-------------|
| 0 | 255 | White |
| 1 | 219 | |
| 2 | 182 | |
| 3 | 146 | |
| 4 | 109 | |
| 5 | 73 | |
| 6 | 36 | |
| 7 | 0 | Black |

Formula: `grayscale = ((7 - shade) * 255 + 3) / 7` (integer math with
rounding). Equivalently, the lookup table {255, 219, 182, 146, 109, 73,
36, 0} can be used directly.

### K1GE Palette Behavior

In K1GE compatibility mode:
- Sprite and scroll tile map entries use the P.C field (bit 5) to select
  between 2 palettes per layer
- The CP.C field (bits 4-1 in scroll entries) is not used
- The sprite color palette assignment table ($8C00-$8C3F) is not used
- Each layer has 2 palettes of 4 shades, providing up to ~20 distinct
  shades across all layers

### K1GE Compatibility Color Mapping

When running monochrome software on NGPC hardware, the BIOS populates the
K1GE compatibility color palette areas ($8380-$83DF) with RGB color values.
The monochrome shade indices from $8100-$8117 are used to look up actual
display colors from these secondary palette areas. This allows the BIOS to
display monochrome games with color tinting.

| K1GE Compat Palette Area | Layer |
|--------------------------|-------|
| $8380-$839F | Sprite palettes (4 palettes x 4 colors) |
| $83A0-$83BF | Scroll Plane 1 palettes (4 palettes x 4 colors) |
| $83C0-$83DF | Scroll Plane 2 palettes (4 palettes x 4 colors) |

Each compat palette area contains 16 entries (32 bytes), organized as two
groups of 8 entries selected by the P.C bit from the tile/sprite attribute:

- P.C=0: shade value indexes entries 0-7 (bytes +$00 to +$0F)
- P.C=1: shade value indexes entries 8-15 (bytes +$10 to +$1F)

The full K1GE color lookup chain is:

1. Tile 2bpp pixel gives color index (0-3)
2. P.C bit selects shade LUT palette (0 or 1)
3. Shade = LUT[palette * 4 + colorIndex] (3-bit value 0-7)
4. Color = compat_palette[P.C * 8 + shade] (12-bit RGB entry)

#### BIOS Color Schemes

The BIOS setup screen allows the user to select one of 5 color schemes
for monochrome games. The selection is stored at $6F94 and the
corresponding 8-shade palette is written to all three compat palette
areas ($8380-$83DF). The palette data is stored in the BIOS ROM at
$FF50B5 (16 bytes per scheme, 8 entries of 16-bit LE in K2GE $0BGR
hardware format).

| $6F94 | Name | Shade 0 | Shade 1 | Shade 2 | Shade 3 | Shade 4 | Shade 5 | Shade 6 | Shade 7 |
|-------|------|---------|---------|---------|---------|---------|---------|---------|---------|
| $00 | Black & White | $0FFF | $0DDD | $0BBB | $0999 | $0777 | $0444 | $0333 | $0000 |
| $01 | Red | $0FFF | $0CCF | $099F | $055F | $011D | $0009 | $0006 | $0000 |
| $02 | Green | $0FFF | $0BFB | $07F7 | $03D3 | $00B0 | $0080 | $0050 | $0000 |
| $03 | Blue | $0FFF | $0FCC | $0FAA | $0F88 | $0E55 | $0B22 | $0700 | $0000 |
| $04 | Classic | $0FFF | $0ADE | $08BD | $059B | $0379 | $0157 | $0034 | $0000 |

All schemes use $0FFF (white) for shade 0 and $0000 (black) for shade 7.
The background color palette at $83E0 and window color at $83F0 are also
set to $0FFF by the BIOS.

### Mode Switching

The NGPC determines the mode from the cartridge header system code:
- $00: Monochrome only - BIOS sets K1GE compatibility mode
- $10: Color - BIOS sets K2GE color mode

Application software can request a mode change via the BIOS system call
VECT_GEMODESET. Direct writes to $87E2 require first unlocking via $87F0.

---

## Window System

The display window defines the visible rectangular region of the screen.

### Window Registers

| Register | Function | Reset Value |
|----------|----------|-------------|
| $8002 (WBA.H) | Window horizontal origin | 0 |
| $8003 (WBA.V) | Window vertical origin | 0 |
| $8004 (WSI.H) | Window horizontal size | $FF |
| $8005 (WSI.V) | Window vertical size | $FF |

### Constraints

- WBA.H + WSI.H must not exceed 160
- WBA.V + WSI.V must not exceed 152
- If the sum exceeds the hardware limit, display and VBlank/HBlank
  interrupt generation are disrupted

The hardware reset values for WSI.H and WSI.V are $FF, which violates the
window size constraints. The BIOS sets the window to minimized values during
boot initialization before transferring control to the user program.
At user program entry, the window size is effectively zero. The application
must set appropriate window dimensions before rendering.

### Outside Window Color

The area outside the defined window rectangle is filled with a solid color:
- The color index is selected by bits 2-0 of $8012 (OOWC)
- The actual color comes from the window color palette at $83F0-$83FF

### Window and Interrupts

The VBlank interrupt is generated after the line at position WBA.V + WSI.V
is drawn. If WSI.V = 0, VBlank occurs after line WBA.V. This means the
window vertical settings directly affect VBlank timing.

HBlank interrupts are independent of window settings. 152 HBlank signals
are always generated per frame regardless of window configuration.

---

## Layer Priority and Compositing

### Layer Sources

The K2GE composites four layer sources:

1. Background color (lowest priority)
2. Scroll Plane (rear, determined by $8030 P.F)
3. Scroll Plane (front, determined by $8030 P.F)
4. Sprites (priority per-sprite via PR.C)

### Compositing Order

The scroll plane ordering is set by register $8030 bit 7 (P.F):
- P.F = 0: Scroll Plane 1 is in front of Scroll Plane 2
- P.F = 1: Scroll Plane 2 is in front of Scroll Plane 1

Each sprite has a 2-bit priority code (PR.C) that places it in one of
four levels relative to the scroll planes:

| PR.C | Sprite Position |
|------|-----------------|
| 00 | Hidden (not rendered) |
| 01 | Behind both scroll planes (only in front of background) |
| 10 | Between the front and rear scroll planes |
| 11 | In front of both scroll planes |

Within the same PR.C level, lower-numbered sprites have higher priority.
When two sprites with the same PR.C value overlap, the lower-numbered
sprite's pixel is displayed.

### Transparency

Color index 0 in any tile or sprite is transparent, allowing layers behind
to show through.

### Per-Scanline Changes

The scroll priority register ($8030), scroll offsets ($8032-$8035), control
register ($8012), and background selection ($8118) all take effect on the
next line drawn. This enables per-scanline priority changes, scroll effects,
and display inversion effects when modified during the drawing period or
from interrupt handlers.

### Compositing Diagram

```
                               +---------------------------+
Background Color ($8118) ----->|                           |
                               |                           |
Scroll Plane (rear) ---------> |     K2GE Line Buffer      |---> LCD
                               |       Compositor          |
Scroll Plane (front) -------->|                           |
                               |                           |
Sprites (per PR.C) ---------->|                           |
                               +---------------------------+
                                           |
                               Window mask applied
                               (outside = OOWC color)
```

---

## Display Timing

### Scanline Timing

| Parameter | Value |
|-----------|-------|
| Horizontal period | 515 TLCS-900H clocks (~83.83 us) |
| Hardware drawing period | ~78.83 us |
| HBlank period | ~5 us |

### Frame Timing

| Parameter | Value |
|-----------|-------|
| Active display lines | 152 (lines 0-151) |
| VBlank lines | 47 |
| Total lines per frame | 199 |
| Frame rate | ~59.95 Hz |
| Frame period | ~16.68 ms |
| VBlank duration | ~3.94 ms |

### Timing Derivation

```
6,144,000 Hz / 59.95 Hz = ~102,485 clocks per frame
102,485 / 515 = ~199 lines per frame
199 - 152 = 47 VBlank lines
```

### Frame Rate Register

Register $8006 (REF) reads as $C6 (198 decimal) at reset. This value
determines the blanking period. The exact relationship between the register
value and the total line count (199) suggests the register may represent
the last line index (0-based: line 198 is the 199th line).

### Raster Position

Unlike a CRT-based system, the NGPC uses an LCD with no scanning beam.
The raster position registers ($8008 and $8009) reflect the internal
video chip timing signal rather than a physical beam position.

---

## Interrupts

### VBlank Interrupt

The VBlank interrupt is generated after the line at (WBA.V + WSI.V) is
drawn. The interrupt vector is at $6FCC (micro DMA vector $0B).

The VBlank interrupt is system-critical. The specification states it is
"forbidden to prohibit" this interrupt because system operations depend
on it. The interrupt mask level must be kept at IFF2-IFF0 <= 2 during
normal operation.

### HBlank Interrupt

The K2GE generates exactly 152 HBlank (H_INT) signals per frame,
independent of window settings. Key timing details:

- H_INT generation begins 1 line before the hardware drawing period starts
- No H_INT is generated at line 151
- The H_INT for line 0 occurs at the start of line 198

The H_INT signal is not directly vectored as a CPU interrupt. Instead, it
is connected to Timer 0's external clock input (TI0). Timer 0 can be
configured to count H_INT pulses and fire its own interrupt after a
programmable number of lines. The Timer 0 interrupt vector is at $6FD4.

This architecture provides flexible per-N-scanline interrupt capability:
- Set Timer 0 to count mode with TI0 as the clock source
- Configure the timer compare value to the desired line interval
- The timer interrupt fires after that many HBlank signals

### Interrupt Vectors (Video-Related)

| Address | Vector | Description |
|---------|--------|-------------|
| $6FCC | $0B | VBlank interrupt |
| $6FD4 | $10 | Timer 0 (driven by H_INT for raster effects) |

---

## LED Control

The NGPC power indicator LED is software-controllable through K2GE
registers.

### $8400 - LED Control (LED_CTL)

| Bits | Description |
|------|-------------|
| 7-3 | Control flash/on mode |
| 2-0 | Always read as 1 |

### $8402 - LED Flash Cycle (LED_FLC)

All 8 bits define the flash cycle period. The value multiplied by ~10.6 ms
gives the flash cycle duration.

Reset value: $80 (128 x 10.6 ms = approximately 1.36 seconds).

This allows games to make the power LED blink in patterns or at different
rates as a visual feedback mechanism.

---

## Character Overflow

Character overflow is a phenomenon associated with the line buffer rendering
method. It occurs when the K2GE cannot complete reading sprite and scroll
plane data from VRAM and updating the line buffer within a single scanline
period.

### Cause

The K2GE reads character data from VRAM during the hardware drawing period.
If program code also accesses VRAM (sprite attributes, scroll maps, or
character RAM) during this time, the combined access time may exceed the
available HBlank margin (~5 us). When this happens, the K2GE cannot finish
compositing all visible characters for that scanline.

### Effect

Characters (tiles/sprites) may partially or completely disappear from the
display on affected scanlines.

### Status Flag

Bit 7 (C.OVR) of the status register ($8010) is set when character overflow
occurs. The flag is cleared at the end of VBlank.

### Mitigation

- Minimize VRAM access during the active display period
- Register access ($8000-$87FF) does not trigger the access timing
  adjustment circuitry and is safe during active display
- Palette RAM access is always 0-wait and does not contribute to overflow
- Concentrate VRAM writes during VBlank when possible

---

## Programming Constraints

### System Restrictions

- Register bank 3 is reserved for the system program. Application software
  must not use it.
- The HALT instruction must not be used because voltage management is
  handled by the system program.
- The watchdog timer control register (WDCR) at address $6E must be written
  with $4E at least once every ~100 ms to prevent a system restart.
- The interrupt mask (IFF2-IFF0) must be kept at <= 2 during normal
  operation to avoid interfering with power management.

### Display Initialization

At user program startup, the window size is minimized. Before rendering,
the application must:
1. Set WBA.H ($8002) and WBA.V ($8003) to the desired origin
2. Set WSI.H ($8004) and WSI.V ($8005) to the desired size
3. Verify WBA.H + WSI.H <= 160 and WBA.V + WSI.V <= 152

### Palette RAM Access

Color palette RAM ($8200-$83FF) must be accessed with 16-bit word reads and
writes only. 8-bit access produces unreliable values.

### Protected Registers

The following registers must not be written by application software:
- $8006 (REF): Frame rate - locked, read-only
- $87E2 (MODE): Mode selection - locked, use BIOS system call

---

---

## Sources

- [Neo Geo Pocket Color Technical Specification (devrs.com)](http://devrs.com/ngp/files/DoNotLink/ngpcspec.txt) - Primary technical reference for K2GE registers, VRAM layout, sprite system, scroll planes, color palettes, display timing, interrupt generation, and programming constraints
- [Neo Geo Pocket Technical Data (devrs.com)](https://www.devrs.com/ngp/files/ngpctech.txt) - Memory map, interrupt vectors, display register addresses
- [SNK Neo Geo Pocket Hardware Information (Data Crystal)](https://datacrystal.tcrf.net/wiki/SNK_Neo_Geo_Pocket/Hardware_information) - System specifications, VRAM size, sprite and palette counts
- [Neo Geo Pocket Color (Game Tech Wiki)](https://www.gametechwiki.com/w/index.php/Neo_Geo_Pocket_Color) - Hardware overview, system identification
- [Neo Geo Pocket Emulation Thread (NESdev Forums)](https://forums.nesdev.org/viewtopic.php?t=18579) - Hardware research findings including palette register behavior, sprite coordinate observations, undocumented register usage
- [NeoGeo Pocket Dev'rs Documentation Page](https://www.devrs.com/ngp/docs.php) - Development resource index
