#!/usr/bin/env python3
"""Generate a substitute 1bpp 8x8 font for the NGPC HLE BIOS.

Produces a 2048-byte 1bpp binary and a 4x scale 16x16 character grid PNG.

Japanese characters are rendered from misaki_gothic_2nd, ASCII from terminus,
both downscaled to 8x8 with majority-vote merge. Non-Unicode custom entries
(blocks, progress bars, triangles, SNK logo) are manual pixel patterns.

Output files:
  substitute_font.bin      - 2048-byte 1bpp font
  substitute_font_grid.png - 4x scale visual grid

Usage: python3 generate_substitute_font.py

Requires: Pillow, fonts-misaki, fonts-terminus
"""

import struct
import sys
import zlib

from PIL import Image, ImageDraw, ImageFont

FONT_PATH = "/usr/share/fonts/truetype/misaki/misaki_gothic_2nd.ttf"
ASCII_FONT_PATH = "/usr/share/fonts/opentype/terminus/terminus-normal.otb"
FONT_SIZE = 16


# ---------------------------------------------------------------------------
# 1bpp font generation
# ---------------------------------------------------------------------------

def find_global_trim(font, chars):
    """Find the global top/bot content bounds across a set of characters."""
    global_top = FONT_SIZE
    global_bot = 0
    for ch in chars:
        adv = max(1, int(font.getlength(ch)))
        img = Image.new("1", (adv, FONT_SIZE), 1)
        draw = ImageDraw.Draw(img)
        draw.text((0, 0), ch, fill=0, font=font)
        for y in range(FONT_SIZE):
            for x in range(adv):
                if img.getpixel((x, y)) == 0:
                    if y < global_top:
                        global_top = y
                    if y > global_bot:
                        global_bot = y
    return global_top, global_bot


def render_char(font, ch, trim_bounds=None, trim_per_char=False,
                place_rows=None):
    """Render a Unicode character into 8 bytes (1bpp 8x8).

    Uses majority-vote merge: pixel on if MAJORITY of source pixels are on.

    trim_bounds: if set to (top, bot), scale that row range into 8 rows.
      All characters share the same bounds so relative positions are kept.
    trim_per_char: if True, find this character's own content bounds and
      scale into 8 rows. Characters are trimmed independently.
    place_rows: if set to (start, count), trim per-char then scale content
      into count rows starting at output row start.
    If none of the above is set, use fixed 2:1 vertical downscale.
    """
    adv = max(1, int(font.getlength(ch)))
    img = Image.new("1", (adv, FONT_SIZE), 1)
    draw = ImageDraw.Draw(img)
    draw.text((0, 0), ch, fill=0, font=font)

    scale_x = adv // 8 if adv >= 8 else 1

    if trim_bounds is not None:
        top, bot = trim_bounds
        content_h = bot - top + 1
        row_map = []
        if content_h <= 8:
            for oy in range(8):
                if oy < content_h:
                    row_map.append((top + oy, top + oy + 1))
                else:
                    row_map.append(None)
        else:
            for oy in range(8):
                s = top + oy * content_h // 8
                e = top + (oy + 1) * content_h // 8
                row_map.append((s, e))
    elif trim_per_char:
        top = FONT_SIZE
        bot = 0
        for y in range(FONT_SIZE):
            for x in range(adv):
                if img.getpixel((x, y)) == 0:
                    if y < top:
                        top = y
                    if y > bot:
                        bot = y
        if top > bot:
            return bytes(8)
        content_h = bot - top + 1
        row_map = []
        if content_h <= 8:
            for oy in range(8):
                if oy < content_h:
                    row_map.append((top + oy, top + oy + 1))
                else:
                    row_map.append(None)
        else:
            for oy in range(8):
                s = top + oy * content_h // 8
                e = top + (oy + 1) * content_h // 8
                row_map.append((s, e))
    elif place_rows is not None:
        start, count = place_rows
        top = FONT_SIZE
        bot = 0
        for y in range(FONT_SIZE):
            for x in range(adv):
                if img.getpixel((x, y)) == 0:
                    if y < top:
                        top = y
                    if y > bot:
                        bot = y
        if top > bot:
            return bytes(8)
        content_h = bot - top + 1
        row_map = []
        for oy in range(8):
            if oy < start or oy >= start + count:
                row_map.append(None)
            else:
                out_idx = oy - start
                s = top + out_idx * content_h // count
                e = top + (out_idx + 1) * content_h // count
                row_map.append((s, e))
    else:
        row_map = []
        for oy in range(8):
            row_map.append((oy * 2, oy * 2 + 2))

    result = []
    for oy in range(8):
        val = 0
        if row_map[oy] is None:
            result.append(0)
            continue
        y_start, y_end = row_map[oy]
        for ox in range(8):
            count = 0
            total = scale_x * (y_end - y_start)
            for sy in range(y_start, y_end):
                for sx in range(scale_x):
                    src_x = ox * scale_x + sx
                    if src_x < adv and img.getpixel((src_x, sy)) == 0:
                        count += 1
            on = count * 2 >= total
            val = (val << 1) | (1 if on else 0)
        result.append(val)
    return bytes(result)


def parse_glyph(s):
    """Convert an 8-line '#'/'.' pattern string to 8 bytes."""
    lines = [line for line in s.strip().split("\n")]
    assert len(lines) == 8, f"expected 8 lines, got {len(lines)}"
    out = []
    for line in lines:
        line = line.strip()
        assert len(line) == 8, f"expected 8 chars, got {len(line)}: {line!r}"
        val = 0
        for ch in line:
            val = (val << 1) | (1 if ch == "#" else 0)
        out.append(val)
    return bytes(out)


# ---------------------------------------------------------------------------
# Manual glyph definitions
# ---------------------------------------------------------------------------

EMPTY = """
........
........
........
........
........
........
........
........
"""

glyphs = {}

# $00-$01, $07: empty
glyphs[0x00] = EMPTY
glyphs[0x01] = EMPTY
glyphs[0x07] = EMPTY

# $02-$03, $0F: full block
FULL_BLOCK = """
########
########
########
########
########
########
########
########
"""
glyphs[0x02] = FULL_BLOCK
glyphs[0x03] = FULL_BLOCK
glyphs[0x0F] = FULL_BLOCK

# $04: vertical bar (2px centered)
glyphs[0x04] = """
...##...
...##...
...##...
...##...
...##...
...##...
...##...
...##...
"""

# $05: horizontal bar (2px centered)
glyphs[0x05] = """
........
........
........
########
########
........
........
........
"""

# $06: corner piece (horizontal top + vertical down-right)
glyphs[0x06] = """
........
........
........
#####...
#####...
...##...
...##...
...##...
"""

# $08-$0E: progressive left blocks (1/8 through 7/8)
glyphs[0x08] = """
#.......
#.......
#.......
#.......
#.......
#.......
#.......
#.......
"""

glyphs[0x09] = """
##......
##......
##......
##......
##......
##......
##......
##......
"""

glyphs[0x0A] = """
###.....
###.....
###.....
###.....
###.....
###.....
###.....
###.....
"""

glyphs[0x0B] = """
####....
####....
####....
####....
####....
####....
####....
####....
"""

glyphs[0x0C] = """
#####...
#####...
#####...
#####...
#####...
#####...
#####...
#####...
"""

glyphs[0x0D] = """
######..
######..
######..
######..
######..
######..
######..
######..
"""

glyphs[0x0E] = """
#######.
#######.
#######.
#######.
#######.
#######.
#######.
#######.
"""

# $10-$13: directional triangles
glyphs[0x10] = """
..#.....
..##....
..###...
..####..
..###...
..##....
..#.....
........
"""

glyphs[0x11] = """
.....#..
....##..
...###..
..####..
...###..
....##..
.....#..
........
"""

glyphs[0x12] = """
........
........
#######.
.#####..
..###...
...#....
........
........
"""

glyphs[0x13] = """
........
........
...#....
..###...
.#####..
#######.
........
........
"""

# $1C-$1F: SNK logo tiles (2x2 arrangement)
# Top-left
glyphs[0x1C] = """
.#######
########
###.....
########
.#######
......##
########
########
"""

# Top-right
glyphs[0x1D] = """
##.###..
##.####.
...#####
#..#####
##.#####
##.###.#
##.###..
#..###..
"""

# Bottom-left
glyphs[0x1E] = """
..###.##
..###.##
..###.##
#.###.##
#####.##
#####.##
#####.##
.####.##
"""

# Bottom-right
glyphs[0x1F] = """
#...####
#..####.
#.####..
#####...
#####...
#.####..
#..####.
#...####
"""

# Manual ASCII overrides
glyphs[0x24] = """
..#.....
.####...
#.#.....
.###....
..#.#...
####....
..#.....
........
"""

glyphs[0x25] = """
##...#..
##..#...
...#....
..#.....
.#......
#...##..
....##..
........
"""

glyphs[0x40] = """
.######.
#......#
#..##..#
#.#....#
#..##..#
#......#
.######.
........
"""

glyphs[0x3D] = """
........
........
.######.
........
.######.
........
........
........
"""

glyphs[0x3A] = """
........
........
...#....
........
........
...#....
........
........
"""

glyphs[0x2E] = """
........
........
........
........
........
...#....
........
........
"""

glyphs[0x23] = """
........
..#..#..
.######.
..#..#..
.######.
..#..#..
........
........
"""

glyphs[0x7B] = """
...##...
..#.....
..#.....
.#......
..#.....
..#.....
...##...
........
"""

glyphs[0x7D] = """
.##.....
...#....
...#....
....#...
...#....
...#....
.##.....
........
"""

glyphs[0x68] = """
.#......
.#......
.#.##...
.##..#..
.#...#..
.#...#..
.#...#..
........
"""

glyphs[0x74] = """
........
...#....
...#....
.#####..
...#....
...#....
....##..
........
"""

glyphs[0x70] = """
........
.####...
.#..#...
.#..#...
.####...
.#......
.#......
........
"""

glyphs[0x71] = """
........
..####..
..#..#..
..#..#..
..####..
.....#..
.....#..
........
"""

glyphs[0x6C] = """
..##....
...#....
...#....
...#....
...#....
...#....
..###...
........
"""

glyphs[0x6A] = """
....#...
........
....#...
....#...
....#...
#...#...
.###....
........
"""

glyphs[0x67] = """
........
..####..
.#...#..
.#...#..
..####..
.....#..
.####...
........
"""

glyphs[0x79] = """
........
........
.#...#..
.#...#..
..####..
.....#..
..###...
........
"""

glyphs[0x62] = """
.#......
.#......
.#.##...
.##..#..
.#...#..
.##..#..
.#.##...
........
"""

glyphs[0x64] = """
.....#..
.....#..
..##.#..
.#..##..
.#...#..
.#..##..
..##.#..
........
"""

glyphs[0x66] = """
....###.
...#....
...#....
.#####..
...#....
...#....
...#....
........
"""

glyphs[0x47] = """
.#####..
#.....#.
#.......
#..###..
#....##.
#.....#.
.#####..
........
"""

glyphs[0x7F] = EMPTY
glyphs[0x80] = EMPTY
glyphs[0xA0] = EMPTY


# ---------------------------------------------------------------------------
# Unicode character map
# ---------------------------------------------------------------------------

charmap = {}

# $14-$1B: calendar kanji
charmap[0x14] = "\u5E74"  # year
charmap[0x15] = "\u6708"  # month
charmap[0x16] = "\u65E5"  # day
charmap[0x17] = "\u706B"  # fire
charmap[0x18] = "\u6C34"  # water
charmap[0x19] = "\u6728"  # wood
charmap[0x1A] = "\u91D1"  # gold
charmap[0x1B] = "\u571F"  # earth

# $20-$7E: ASCII (except $40 which is copyright)
for i in range(0x20, 0x7F):
    if i == 0x40:
        charmap[i] = "\u00A9"
    else:
        charmap[i] = chr(i)

# $81-$85: Japanese punctuation
charmap[0x81] = "\u3002"  # ideographic period
charmap[0x82] = "\u300C"  # left corner bracket
charmap[0x83] = "\u300D"  # right corner bracket
charmap[0x84] = "\u3001"  # ideographic comma
charmap[0x85] = "\u30FB"  # middle dot

# $86: hiragana wo
charmap[0x86] = "\u3092"

# $87-$8F: small hiragana
charmap[0x87] = "\u3041"  # small a
charmap[0x88] = "\u3043"  # small i
charmap[0x89] = "\u3045"  # small u
charmap[0x8A] = "\u3047"  # small e
charmap[0x8B] = "\u3049"  # small o
charmap[0x8C] = "\u3083"  # small ya
charmap[0x8D] = "\u3085"  # small yu
charmap[0x8E] = "\u3087"  # small yo
charmap[0x8F] = "\u3063"  # small tsu

# $90: prolonged sound mark
charmap[0x90] = "\u30FC"

# $91-$9F: hiragana a through so
for i, cp in enumerate([
    0x3042, 0x3044, 0x3046, 0x3048, 0x304A,  # a i u e o
    0x304B, 0x304D, 0x304F, 0x3051, 0x3053,  # ka ki ku ke ko
    0x3055, 0x3057, 0x3059, 0x305B, 0x305D,  # sa si su se so
]):
    charmap[0x91 + i] = chr(cp)

# $A1-$A5: Japanese punctuation (same as $81-$85)
charmap[0xA1] = "\u3002"
charmap[0xA2] = "\u300C"
charmap[0xA3] = "\u300D"
charmap[0xA4] = "\u3001"
charmap[0xA5] = "\u30FB"

# $A6: katakana wo
charmap[0xA6] = "\u30F2"

# $A7-$AF: small katakana
charmap[0xA7] = "\u30A1"  # small a
charmap[0xA8] = "\u30A3"  # small i
charmap[0xA9] = "\u30A5"  # small u
charmap[0xAA] = "\u30A7"  # small e
charmap[0xAB] = "\u30A9"  # small o
charmap[0xAC] = "\u30E3"  # small ya
charmap[0xAD] = "\u30E5"  # small yu
charmap[0xAE] = "\u30E7"  # small yo
charmap[0xAF] = "\u30C3"  # small tsu

# $B0: prolonged sound mark
charmap[0xB0] = "\u30FC"

# $B1-$DD: full katakana a through n
for i, cp in enumerate([
    0x30A2, 0x30A4, 0x30A6, 0x30A8, 0x30AA,  # a i u e o
    0x30AB, 0x30AD, 0x30AF, 0x30B1, 0x30B3,  # ka ki ku ke ko
    0x30B5, 0x30B7, 0x30B9, 0x30BB, 0x30BD,  # sa si su se so
    0x30BF, 0x30C1, 0x30C4, 0x30C6, 0x30C8,  # ta ti tu te to
    0x30CA, 0x30CB, 0x30CC, 0x30CD, 0x30CE,  # na ni nu ne no
    0x30CF, 0x30D2, 0x30D5, 0x30D8, 0x30DB,  # ha hi hu he ho
    0x30DE, 0x30DF, 0x30E0, 0x30E1, 0x30E2,  # ma mi mu me mo
    0x30E4, 0x30E6, 0x30E8,                   # ya yu yo
    0x30E9, 0x30EA, 0x30EB, 0x30EC, 0x30ED,  # ra ri ru re ro
    0x30EF, 0x30F3,                            # wa n
]):
    charmap[0xB1 + i] = chr(cp)

# $DE-$DF: dakuten, handakuten
charmap[0xDE] = "\u309B"
charmap[0xDF] = "\u309C"

# $E0-$FD: hiragana ta through n
for i, cp in enumerate([
    0x305F, 0x3061, 0x3064, 0x3066, 0x3068,  # ta ti tu te to
    0x306A, 0x306B, 0x306C, 0x306D, 0x306E,  # na ni nu ne no
    0x306F, 0x3072, 0x3075, 0x3078, 0x307B,  # ha hi hu he ho
    0x307E, 0x307F, 0x3080, 0x3081, 0x3082,  # ma mi mu me mo
    0x3084, 0x3086, 0x3088,                   # ya yu yo
    0x3089, 0x308A, 0x308B, 0x308C, 0x308D,  # ra ri ru re ro
    0x308F, 0x3093,                            # wa n
]):
    charmap[0xE0 + i] = chr(cp)

# $FE-$FF: dakuten, handakuten (same as $DE-$DF)
charmap[0xFE] = "\u309B"
charmap[0xFF] = "\u309C"


# ---------------------------------------------------------------------------
# 1bpp to 2bpp conversion
# ---------------------------------------------------------------------------

def convert_1bpp_to_2bpp(data):
    """Convert 2048-byte 1bpp font to 4096-byte 2bpp format."""
    out = bytearray(4096)

    for i in range(2048):
        byte = data[i]
        row_idx = i % 8
        char_idx = i // 8
        base = char_idx * 16 + row_idx * 2

        # High byte: left 4 pixels (bits 7-4 of 1bpp byte)
        hi_val = 0
        for px in range(4):
            if byte & (0x80 >> px):
                hi_val |= 1 << (6 - px * 2)

        # Low byte: right 4 pixels (bits 3-0 of 1bpp byte)
        lo_val = 0
        for px in range(4):
            if byte & (0x08 >> px):
                lo_val |= 1 << (6 - px * 2)

        out[base] = lo_val
        out[base + 1] = hi_val

    return bytes(out)


# ---------------------------------------------------------------------------
# Grid PNG rendering
# ---------------------------------------------------------------------------

def decode_pixels(data, scale):
    """Decode 2bpp font data into a pixel grid with integer scaling."""
    cell = 8 * scale + 1  # scaled char + 1px gap
    img_w = 16 * cell + 1
    img_h = 16 * cell + 1

    pixels = [
        [2 if (x % cell == 0 or y % cell == 0) else 0 for x in range(img_w)]
        for y in range(img_h)
    ]

    for char_idx in range(256):
        grid_x = char_idx % 16
        grid_y = char_idx // 16
        base = char_idx * 16

        for row in range(8):
            lo = data[base + row * 2]
            hi = data[base + row * 2 + 1]

            for px in range(4):
                shift = 6 - px * 2
                val = (hi >> shift) & 0x03
                if val != 0:
                    for sy in range(scale):
                        for sx in range(scale):
                            px_x = 1 + grid_x * cell + px * scale + sx
                            px_y = 1 + grid_y * cell + row * scale + sy
                            pixels[px_y][px_x] = 1

            for px in range(4):
                shift = 6 - px * 2
                val = (lo >> shift) & 0x03
                if val != 0:
                    for sy in range(scale):
                        for sx in range(scale):
                            px_x = 1 + grid_x * cell + (4 + px) * scale + sx
                            px_y = 1 + grid_y * cell + row * scale + sy
                            pixels[px_y][px_x] = 1

    return pixels, img_w, img_h


def write_png(pixels, img_w, img_h, output_path):
    """Write pixel grid as PNG (no external dependencies beyond zlib)."""
    palette = {0: (255, 255, 255), 1: (0, 0, 0), 2: (200, 200, 200)}

    def png_chunk(chunk_type, data):
        chunk = chunk_type + data
        return (struct.pack(">I", len(data)) + chunk
                + struct.pack(">I", zlib.crc32(chunk) & 0xFFFFFFFF))

    ihdr = struct.pack(">IIBBBBB", img_w, img_h, 8, 2, 0, 0, 0)

    raw = bytearray()
    for y in range(img_h):
        raw.append(0)  # filter: None
        for x in range(img_w):
            r, g, b = palette[pixels[y][x]]
            raw.extend([r, g, b])

    compressed = zlib.compress(bytes(raw))

    with open(output_path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(png_chunk(b"IHDR", ihdr))
        f.write(png_chunk(b"IDAT", compressed))
        f.write(png_chunk(b"IEND", b""))


# ---------------------------------------------------------------------------
# Font generation pipeline
# ---------------------------------------------------------------------------

def generate_font():
    """Generate the 1bpp font data (2048 bytes)."""
    try:
        font = ImageFont.truetype(FONT_PATH, size=FONT_SIZE)
    except OSError:
        print(f"Cannot load font: {FONT_PATH}", file=sys.stderr)
        sys.exit(1)

    try:
        ascii_font = ImageFont.truetype(ASCII_FONT_PATH, size=FONT_SIZE)
    except OSError:
        print(f"Cannot load ASCII font: {ASCII_FONT_PATH}", file=sys.stderr)
        sys.exit(1)

    print(f"Using font: {FONT_PATH}")
    print(f"Using ASCII font: {ASCII_FONT_PATH}")

    # ASCII characters that look good with global trim
    global_trim_chars = set(
        "0123456789/,-()'" + '|;<>?ABCDEFGHIJKLMNOPQRSTUVWXYZ'
        + '[\\]`"&'
    )

    ascii_global = [ch for ch in global_trim_chars if ch in charmap.values()]
    ascii_trim = find_global_trim(ascii_font, ascii_global)
    print(f"ASCII trim bounds: rows {ascii_trim[0]}-{ascii_trim[1]} "
          f"(height={ascii_trim[1]-ascii_trim[0]+1})")

    # Post-render vertical shift: negative = up, positive = down
    ascii_vshift = {
        '0': -1, '1': -1, '2': -1, '3': -1, '4': -1,
        '5': -1, '6': -1, '7': -1, '8': -1, '9': -1,
        '!': -1, '#': -1,
        '(': -1, ')': -1, '&': -1, '/': -1, '?': -1,
        '>': -1, '<': -1, ';': -1, '[': -1, ']': -1, '\\': -1,
        'A': -1, 'B': -1, 'C': -1, 'D': -1, 'E': -1, 'F': -1,
        'G': -1, 'H': -1, 'I': -1, 'J': -1, 'K': -1, 'L': -1,
        'M': -1, 'N': -1, 'O': -1, 'P': -1, 'Q': -1, 'R': -1,
        'S': -1, 'T': -1, 'U': -1, 'V': -1, 'W': -1, 'X': -1,
        'Y': -1, 'Z': -1,
        '*': 1, '+': 1, '~': 2, '_': 6, '|': -1, 'k': -1, 'i': -1,
    }

    # Lowercase letters that need trim+scale into 5 rows at position 2-6
    place_chars = set("acegmnorsuvwxyz")

    data = bytearray()
    for i in range(256):
        if i in glyphs:
            g = glyphs[i]
            if isinstance(g, str):
                data.extend(parse_glyph(g))
            else:
                data.extend(g)
        elif i in charmap:
            is_ascii = 0x20 <= i <= 0x7E
            f = ascii_font if is_ascii else font
            if is_ascii and charmap[i] in place_chars:
                glyph = render_char(f, charmap[i], place_rows=(2, 5))
            elif is_ascii and charmap[i] in global_trim_chars:
                glyph = render_char(f, charmap[i], trim_bounds=ascii_trim)
            elif is_ascii:
                glyph = render_char(f, charmap[i], trim_per_char=True)
            else:
                glyph = render_char(f, charmap[i])
            # Apply vertical shift if specified
            shift = ascii_vshift.get(charmap[i], 0)
            if shift != 0:
                rows = list(glyph)
                if shift < 0:
                    rows = rows[-shift:] + [0] * (-shift)
                else:
                    rows = [0] * shift + rows[:8 - shift]
                glyph = bytes(rows[:8])
            data.extend(glyph)
        else:
            data.extend(bytes(8))

    assert len(data) == 2048, f"expected 2048 bytes, got {len(data)}"
    return bytes(data)


def main():
    bin_path = "substitute_font.bin"
    png_path = "substitute_font_grid.png"

    # Step 1: Generate 1bpp font
    font_1bpp = generate_font()

    # Step 2: Write 1bpp binary
    with open(bin_path, "wb") as f:
        f.write(font_1bpp)
    print(f"Wrote {len(font_1bpp)} bytes to {bin_path}")

    # Step 3: Convert to 2bpp in memory
    font_2bpp = convert_1bpp_to_2bpp(font_1bpp)

    # Step 4: Render grid PNG at 4x scale
    pixels, w, h = decode_pixels(font_2bpp, scale=4)
    write_png(pixels, w, h, png_path)
    print(f"Wrote {w}x{h} PNG (scale 4x) to {png_path}")


if __name__ == "__main__":
    main()
