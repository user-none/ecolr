# Neo Geo Pocket Cartridge ROM Format

Technical reference for the ROM file format used by Neo Geo Pocket (NGP) and
Neo Geo Pocket Color (NGPC) cartridges. Covers file structure, header layout,
ROM-to-address mapping, flash chip identification, and save data persistence.

For the hardware cartridge interface (bus signals, chip selects, wait states),
see [ngpc_system_reference.md](ngpc_system_reference.md).

---

## Table of Contents

- [File Format](#file-format)
- [ROM Header](#rom-header)
- [ROM Size and Flash Chip Mapping](#rom-size-and-flash-chip-mapping)
- [Address Mapping](#address-mapping)
- [License String Validation](#license-string-validation)
- [Flash Command Protocol](#flash-command-protocol)
- [Flash Status Polling](#flash-status-polling)
- [Flash Block Protection](#flash-block-protection)
- [Emulation Notes](#emulation-notes)
- [Save Data Format](#save-data-format)
- [Loading Procedure](#loading-procedure)
- [Known Header Defects](#known-header-defects)
- [Sources](#sources)

---

## File Format

NGP/NGPC ROM files are raw binary dumps of the cartridge flash ROM contents.
There is no container format, no file header beyond the ROM data itself, and no
compression. Byte 0 of the file corresponds directly to address $200000 in the
NGPC memory map.

### File Extensions

| Extension | Description |
|-----------|-------------|
| .ngp | Neo Geo Pocket ROM (monochrome) |
| .ngc | Neo Geo Pocket Color ROM |
| .ngpc | Neo Geo Pocket Color ROM (alternate) |
| .npc | Neo Geo Pocket Color ROM (alternate) |

The file extension is a convention only. It does not determine mono vs color
mode - that is specified by the system code byte at header offset $23.

---

## ROM Header

The first 64 bytes ($00-$3F) of the ROM file contain the cartridge header.
This data resides at address $200000-$20003F in the NGPC memory map.

### Layout

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| $00 | 28 bytes | License | Copyright/license string (see below) |
| $1C | 4 bytes | Start PC | Entry point address (32-bit little-endian) |
| $20 | 2 bytes | Software ID | Catalog number (BCD, little-endian) |
| $22 | 1 byte | Sub-code | Software version / revision |
| $23 | 1 byte | System code | $00 = monochrome (NGP), $10 = color (NGPC) |
| $24 | 12 bytes | Title | Game title (ASCII, padded with spaces or zeros) |
| $30 | 16 bytes | Reserved | Must be zero |

### Field Details

**License ($00-$1B):** One of two 28-byte strings:

```
"COPYRIGHT BY SNK CORPORATION"    (28 bytes, no padding)
" LICENSED BY SNK CORPORATION"    (28 bytes, leading space)
```

Both strings share a common suffix " BY SNK CORPORATION" starting at offset
$09. The BIOS validates this string during boot.

**Start PC ($1C-$1F):** The 32-bit little-endian address where the BIOS
transfers control after boot initialization. Values observed in retail games:

| Start PC | Meaning |
|----------|---------|
| $200040 | Entry immediately after the header (common) |
| $200100+ | Entry deeper in ROM (game-specific) |

The address must be within the cartridge ROM address space ($200000+).

**Software ID ($20-$21):** A BCD-encoded catalog number assigned by SNK. The
two bytes are little-endian. For example, bytes `$68 $00` represent catalog
ID 0068.

**Sub-code ($22):** Version or revision number. Not BCD - raw hex value.

**System Code ($23):** Determines the graphics mode the BIOS selects:

| Value | Mode | Graphics Engine |
|-------|------|-----------------|
| $00 | Monochrome | K1GE compatibility mode |
| $10 | Color | K2GE color mode |

The BIOS reads this byte and configures the K2GE mode register ($87E2)
accordingly. A .ngp file should have system code $00 and a .ngc file should
have system code $10, though the system code takes precedence over the file
extension.

**Title ($24-$2F):** 12-byte ASCII game title. Unused bytes may be spaces
($20) or null ($00). Non-printable characters should be treated as spaces.

**Reserved ($30-$3F):** 16 bytes, must all be zero. Verified across all
examined ROM files.

### Example Headers

```
Puzzle Link (USA) - 512 KB, color:
  License:  " LICENSED BY SNK CORPORATION"
  Start PC: $200040
  SW ID:    0054  Sub: 20
  System:   $10 (color)
  Title:    "PUZZLE LINK "

King of Fighters R-1 (Europe) - 2 MB, mono:
  License:  "COPYRIGHT BY SNK CORPORATION"
  Start PC: $200040
  SW ID:    0001  Sub: 0A
  System:   $00 (mono)
  Title:    "KOF R1      "
```

---

## ROM Size and Flash Chip Mapping

### Valid ROM Sizes

ROM size is determined from the file size. The following sizes correspond to
real cartridge flash chip configurations:

| File Size | Flash Capacity | Device ID | Manufacturer ID | Chips |
|-----------|---------------|-----------|-----------------|-------|
| 512 KB | 4 Mbit | $AB | $98 | 1 (CS0) |
| 1 MB | 8 Mbit | $2C | $98 | 1 (CS0) |
| 2 MB | 16 Mbit | $2F | $98 | 1 (CS0) |
| 4 MB | 32 Mbit | $2F | $98 | 2 (CS0 + CS1) |

Manufacturer ID $98 is Toshiba. Real cartridges may also contain Samsung ($EC)
or Sharp ($B0) chips. The BIOS identifies the chip by issuing the flash ID
command (see [Flash Command Protocol](#flash-command-protocol)).

Only 4 MB ROMs use two flash chips. The first 2 MB maps to CS0 and the second
2 MB maps to CS1.

### Small / Non-Standard Sizes

For robustness, accept power-of-two sizes from 32 KB to 4 MB. ROMs smaller
than 512 KB should use device ID $AB (4 Mbit chip).

---

## Address Mapping

The ROM file is mapped into the TLCS-900H address space as follows:

```
File offset 0x000000  -->  CPU address $200000  (CS0 start)
File offset 0x1FFFFF  -->  CPU address $3FFFFF  (CS0 end, 2 MB max)
File offset 0x200000  -->  CPU address $800000  (CS1 start, 4 MB ROMs only)
File offset 0x3FFFFF  -->  CPU address $9FFFFF  (CS1 end)
```

### Memory Map

| Chip Select | CPU Address Range | File Offset Range | Max Size |
|-------------|-------------------|-------------------|----------|
| CS0 | $200000-$3FFFFF | $000000-$1FFFFF | 2 MB |
| CS1 | $800000-$9FFFFF | $200000-$3FFFFF | 2 MB |

CS1 is only active when the ROM file is larger than 2 MB. Reads from the CS1
region when no second chip is present should return open bus / undefined data.

The region $3F0000-$3FFFFF (file offsets $1F0000-$1FFFFF) is the system-
reserved area at the top of CS0. The last block of each flash chip is reserved
for BIOS flash testing on every boot.

### Buffer Allocation

Allocate a 4 MB buffer and load the ROM file at the start. If the ROM is
smaller than 2 MB, only the CS0 window contains valid data. If the ROM is
exactly 4 MB, the second 2 MB half is mapped to CS1.

---

## License String Validation

The BIOS validates the license string during boot. Validation checks the
common suffix at offset $09:

```
Offset:  00 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 ...
         C  O  P  Y  R  I  G  H  T     B  Y     S  N  K     ...
                                     ^-- check starts here
         _  L  I  C  E  N  S  E  D     B  Y     S  N  K     ...
                                     ^-- same position
```

The 19-character string " BY SNK CORPORATION" at offset $09-$1B is the
minimum validation needed. If this check fails, the BIOS will not boot the
cartridge.

---

## Flash Command Protocol

The cartridge flash chips use AMD-style command sequences. The emulator must
implement a flash state machine that responds to these commands because the
BIOS uses them during boot (chip ID read, last-block test) and runtime (save
data writes, block erase).

### Command Sequences

All command sequences write to specific offsets within the flash chip's
address space. The addresses below are relative to the chip base ($200000
for CS0, $800000 for CS1).

| Command | Sequence | Description |
|---------|----------|-------------|
| Chip ID | $5555=$AA, $2AAA=$55, $5555=$90 | Enter ID mode. Read manufacturer at offset $00, device at offset $01, protection at offset $02. |
| ID Exit | $5555=$AA, $2AAA=$55, $5555=$F0 | Return to read mode. Also: write $F0 to any address. |
| Program Byte | $5555=$AA, $2AAA=$55, $5555=$A0, addr=data | Write one byte. Flash can only clear bits (1 to 0). |
| Block Erase | $5555=$AA, $2AAA=$55, $5555=$80, $5555=$AA, $2AAA=$55, block_addr=$30 | Erase block to $FF. 6-cycle command. |
| Chip Erase | $5555=$AA, $2AAA=$55, $5555=$80, $5555=$AA, $2AAA=$55, $5555=$10 | Erase entire chip. 6-cycle command. |
| Block Protect | $5555=$AA, $2AAA=$55, $5555=$9A, $5555=$AA, $2AAA=$55, $5555=$9A | Permanently protect a flash block from writes. 6-cycle command. |
| Reset | any=$F0 | Return to read mode from any state. |

### Flash State Machine

The flash controller tracks command state per chip.

```
READ (default)
  |
  +-- Write $AA to $5555 --> CMD1
  |
  +-- Write $F0 to any --> READ (reset)

CMD1
  |
  +-- Write $55 to $2AAA --> CMD2
  |
  +-- Other --> READ (abort)

CMD2
  |
  +-- Write $90 to $5555 --> ID_MODE
  |
  +-- Write $A0 to $5555 --> PROGRAM (next write programs the byte)
  |
  +-- Write $80 to $5555 --> ERASE1
  |
  +-- Write $9A to $5555 --> PROTECT1
  |
  +-- Write $F0 to $5555 --> READ (ID exit)
  |
  +-- Other --> READ (abort)

ID_MODE
  |
  +-- Read offset $00 --> returns manufacturer ID
  +-- Read offset $01 --> returns device ID
  +-- Read offset $02 --> returns block protection status ($00 = not protected)
  +-- Read offset $03 --> returns $80 (additional device info)
  +-- Write $F0 to any --> READ (exit ID mode)
  +-- Write $AA to $5555 --> CMD1 (start new command sequence)

PROGRAM
  |
  +-- Write data to addr --> data = existing & written (AND operation)
  |                          transition to BUSY, then READ on completion
  |
  +-- See "Flash Status Polling" for read behavior during BUSY

ERASE1
  |
  +-- Write $AA to $5555 --> ERASE2
  |
  +-- Other --> READ (abort)

ERASE2
  |
  +-- Write $55 to $2AAA --> ERASE3
  |
  +-- Other --> READ (abort)

ERASE3
  |
  +-- Write $30 to block_addr --> erase that block (fill with $FF),
  |                               transition to BUSY, then READ on completion
  +-- Write $10 to $5555 --> erase entire chip (fill with $FF),
  |                          transition to BUSY, then READ on completion
  |
  +-- Other --> READ (abort)

PROTECT1
  |
  +-- Write $AA to $5555 --> PROTECT2
  |
  +-- Other --> READ (abort)

PROTECT2
  |
  +-- Write $55 to $2AAA --> PROTECT3
  |
  +-- Other --> READ (abort)

PROTECT3
  |
  +-- Write $9A to $5555 --> protect block, return to READ
  |
  +-- Other --> READ (abort)
```

### ID Mode Read Addresses

In ID mode, the low 2 bits of the read address select which value is
returned:

| Address & $03 | Value | Description |
|---------------|-------|-------------|
| $00 | Manufacturer ID | $98 (Toshiba), $EC (Samsung), or $B0 (Sharp) |
| $01 | Device ID | $AB (4 Mbit), $2C (8 Mbit), or $2F (16 Mbit) |
| $02 | Block protection | $00 = not protected |
| $03 | Additional info | $80 |

ID reads work relative to any block base address within the chip. For
example, reading from chip_base + $00, chip_base + $7C000, or
chip_base + $1FC000 all return the manufacturer ID when in ID mode.

When entering ID mode, the emulator must either intercept reads to return
the ID values, or temporarily overwrite the ROM data at the affected
offsets and restore the original data when exiting ID mode.

### Block Layout

Each flash chip divides into main blocks and boot blocks. The boot blocks
occupy the top of the address space:

| Chip Size | Main Blocks | Boot Blocks (top of address space) |
|-----------|-------------|-----------------------------------|
| 4 Mbit (512 KB) | 7 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |
| 8 Mbit (1 MB) | 15 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |
| 16 Mbit (2 MB) | 31 x 64 KB | 32 KB + 8 KB + 8 KB + 16 KB |

The last block (16 KB boot block at the very top) is system-reserved. The
BIOS tests this block on every boot.

### Boot Block Address Ranges

The boot blocks occupy the top 64 KB of each chip. Their irregular sizes
require special handling during block erase. The erase command address
determines which block is erased.

**16 Mbit (2 MB) chip - boot blocks at $1F0000-$1FFFFF:**

| Block | Address Range | Size | Address Mask |
|-------|--------------|------|-------------|
| SA31 | $1F0000-$1F7FFF | 32 KB | addr & $1F8000 |
| SA32 | $1F8000-$1F9FFF | 8 KB | addr & $1FE000 |
| SA33 | $1FA000-$1FBFFF | 8 KB | addr & $1FE000 |
| SA34 | $1FC000-$1FFFFF | 16 KB | addr & $1FC000 |

**8 Mbit (1 MB) chip - boot blocks at $F0000-$FFFFF:**

| Block | Address Range | Size | Address Mask |
|-------|--------------|------|-------------|
| SA15 | $F0000-$F7FFF | 32 KB | addr & $F8000 |
| SA16 | $F8000-$F9FFF | 8 KB | addr & $FE000 |
| SA17 | $FA000-$FBFFF | 8 KB | addr & $FE000 |
| SA18 | $FC000-$FFFFF | 16 KB | addr & $FC000 |

**4 Mbit (512 KB) chip - boot blocks at $70000-$7FFFF:**

| Block | Address Range | Size | Address Mask |
|-------|--------------|------|-------------|
| SA7 | $70000-$77FFF | 32 KB | addr & $78000 |
| SA8 | $78000-$79FFF | 8 KB | addr & $7E000 |
| SA9 | $7A000-$7BFFF | 8 KB | addr & $7E000 |
| SA10 | $7C000-$7FFFF | 16 KB | addr & $7C000 |

Main blocks (SA0 through the block before the boot region) are all 64 KB
and can be identified by masking the address with the chip size boundary
for the main region (e.g., addr & $1F0000 for 16 Mbit).

### Program Byte Behavior

Flash programming can only change bits from 1 to 0. The result of a program
operation is `existing_byte AND new_byte`. To write arbitrary data, the
containing block must first be erased (set to all $FF), then programmed.

---

## Flash Status Polling

AMD-style flash chips provide status information through reads during
program and erase operations. The BIOS uses these status bits to determine
when an operation has completed.

### Status Bits

During a program or erase operation, reads to the chip return status
information instead of data. The relevant bits are:

| Bit | Name | During Operation | After Completion |
|-----|------|-----------------|------------------|
| DQ7 | Data Polling | Complement of bit 7 of written data (program) or $00 (erase) | Actual data bit 7 (program) or $01 (erase) |
| DQ6 | Toggle | Toggles between 0 and 1 on consecutive reads | Stops toggling, returns actual data bit 6 |
| DQ5 | Exceeded Timing | $00 while operation is within time limit | $01 if operation has exceeded maximum time (error) |
| DQ3 | Sector Erase Timer | $00 during erase timeout window | $01 after erase has begun |
| DQ2 | Toggle (alternate) | Toggles in conjunction with DQ6 | Stops toggling |

### Read Scope During Operations

During a sector erase, only reads to addresses within the erasing sector
return status bits. Reads to addresses outside the erasing sector return
normal ROM data. This allows software to continue reading from other
parts of the chip while an erase is in progress.

During a byte program operation, reads to the address being programmed
return status bits. Reads to other addresses return normal data.

### DQ7 - Data Polling

This is the primary completion detection mechanism. During a byte program
operation, reading the target address returns the complement of bit 7 of
the byte being programmed on DQ7. Once programming completes, DQ7 returns
the true value of bit 7.

During an erase operation, reading an address within the erasing sector
returns $00 on DQ7 while erasing is in progress and $01 when the erase
has completed.

The polling algorithm:
1. Read the target address
2. Check DQ7 - if it matches the expected value, the operation is complete
3. Check DQ5 - if set, read again and recheck DQ7. If DQ7 still does not
   match, the operation has failed (exceeded timing limit)
4. If neither condition is met, repeat from step 1

### DQ6 - Toggle Bit

Bit 6 toggles on every consecutive read while a program or erase operation
is in progress. Once the operation completes, DQ6 stops toggling and
returns the actual data. This provides an alternative to DQ7 polling.

During erase, bit 2 also toggles in conjunction with bit 6.

### DQ3 - Sector Erase Timer

After the first sector erase command ($30 in ERASE3 state), there is a
brief timeout window (~50us on real hardware) during which additional
sectors can be queued for simultaneous erasure by sending additional $30
commands to different block addresses. DQ3 reads as $00 during this
window and $01 once the erase operation has begun. After DQ3 goes high,
no more sectors can be added to the current erase operation.

For NGPC emulation, multi-sector erase queuing is unlikely to be used by
the BIOS or games. Treating each $30 command as an independent single-
sector erase is acceptable.

---

## Flash Block Protection

The BIOS system call VECT_FLASHPROTECT ($FFFE34) permanently protects
flash blocks from further writes and erases. This uses a $9A command
sequence:

```
$5555=$AA, $2AAA=$55, $5555=$9A   (first $9A - enter protect mode)
$5555=$AA, $2AAA=$55, $5555=$9A   (second $9A - confirm and protect)
```

Once a block is protected, program and erase commands targeting that block
are ignored by the flash chip. Protection is permanent and cannot be
reversed through software commands.

In ID mode, offset $02 within any block returns the protection status:
$00 = not protected, non-zero = protected.

---

## Emulation Notes

### Instant Completion

All known NGPC emulators treat program and erase operations as completing
instantly - the status bits immediately reflect the "completed" state.
This works because the BIOS polling loops check DQ7 or DQ6 and
immediately see the completion condition.

For a correct implementation:
- After a program byte command, transition to a BUSY state where DQ7
  returns the complement of the written byte's bit 7, then immediately
  transition to READ
- After an erase command, transition to a BUSY state where DQ7 returns
  $00 and DQ6 toggles, then immediately transition to READ

Instant completion is an acceptable simplification. If compatibility
issues arise with specific games that have tight timing dependencies,
a small cycle delay can be introduced.

### Real Hardware Timing

For reference, typical AMD flash operation times:
- Byte program: 7-14 us
- Sector erase: 0.7-1.0 s
- Chip erase: 8-64 s (varies by chip capacity)

These timings are not needed for functional emulation but may be relevant
if cycle-accurate timing is ever required.

### Features Not Needed

The following AMD flash features exist in the datasheets but are not
used by the NGPC BIOS or games based on analysis of all known emulator
implementations:

- **Unlock bypass mode** ($20 command): Allows faster bulk programming
  with a 2-cycle program sequence instead of 4 cycles. No NGPC emulator
  implements this.
- **Erase suspend/resume** ($B0 to suspend, $30 to resume): Allows
  pausing an erase to read or program other sectors. No NGPC emulator
  implements this.

These can be omitted from the initial implementation and added later
if a compatibility issue is discovered.

---

## Save Data Format

Games save data to the cartridge flash ROM using the BIOS system calls
(VECT_FLASHWRITE, VECT_FLASHERS). The emulator must persist flash
modifications between sessions.

### Approach: Block-Based Delta

Rather than saving the entire ROM (up to 4 MB), only modified flash blocks
are saved. On load, the original ROM data is read from the ROM file, then
saved block deltas are overlaid on top to restore the previous flash state.

### NGF File Format

The .ngf format stores a header followed by variable-length block records:

**File Header (8 bytes):**

| Offset | Size | Field | Value |
|--------|------|-------|-------|
| $00 | 2 bytes | Magic | $0053 (little-endian) |
| $02 | 2 bytes | Block count | Number of saved blocks (little-endian) |
| $04 | 4 bytes | File length | Total file size in bytes (little-endian) |

**Block Record (variable length):**

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| $00 | 4 bytes | Address | NGPC address of block start (little-endian) |
| $04 | 4 bytes | Length | Data byte count (little-endian) |
| $08 | N bytes | Data | Modified flash content |

Block records immediately follow each other after the header. Maximum block
count: 256.

### Save/Load Procedure

**Saving:** After emulation ends (or periodically), compare the ROM buffer
against the original ROM data. Write only the modified blocks to the save
file.

**Loading:** After loading the ROM file into the buffer, read the save file
and overlay each saved block onto the ROM buffer at the specified address.
This restores the flash state from the previous session.

The original unmodified ROM data must be preserved separately (either a copy
or by tracking dirty regions) to support save file generation and save state
operations.

---

## Loading Procedure

Summary of steps to load a ROM file:

1. Read the file into a buffer. Verify the file size is a power of two
   between 32 KB and 4 MB.

2. Parse the header at offset $00:
   - Validate the license string suffix at offset $09 (19 bytes)
   - Read the system code at offset $23 to determine mono ($00) vs color
     ($10) mode
   - Read the start PC at offset $1C (32-bit little-endian)
   - Read the title at offset $24 (12 bytes ASCII)

3. Determine flash chip configuration from file size:
   - Set manufacturer ID to $98
   - Set device ID based on size ($AB / $2C / $2F)
   - If 4 MB: configure two flash chips, one per chip select

4. Map the ROM buffer into the address space:
   - First 2 MB at $200000-$3FFFFF (CS0)
   - If > 2 MB: second 2 MB at $800000-$9FFFFF (CS1)

5. Preserve a copy of the original ROM data for save delta computation.

6. Load the save file (.ngf) if it exists and overlay modified blocks onto
   the ROM buffer.

7. Initialize the flash state machine(s) to READ state.

---

## Known Header Defects

A small number of ROM dumps have incorrect header fields. The most common
issue is a wrong system code byte at offset $23 (mono/color mode). These
defects cause the BIOS to configure the wrong graphics mode.

### Mode Byte Defects

| Software ID | Sub | Title | Header Mode | Correct Mode |
|-------------|-----|-------|-------------|-------------|
| 0000 | $10 | Neo-Neo! V1.0 (PD) | $00 (mono) | $10 (color) |
| 1234 | $A1 | Cool Cool Jam SAMPLE (U) | $00 (mono) | $10 (color) |
| 0033 | $21 | Dokodemo Mahjong (J) | $10 (color) | $00 (mono) |

These are identified by matching the software ID at offset $20 and sub-code
at offset $22. If detected, the mode byte at offset $23 should be corrected
before the BIOS reads the header.

---

## Sources

- [Neo Geo Pocket Specification (devrs.com)](http://devrs.com/ngp/files/DoNotLink/ngpcspec.txt) -
  ROM header format, cartridge specifications
- [Neo Geo Pocket Technical Data (devrs.com)](https://www.devrs.com/ngp/files/ngpctech.txt) -
  Memory map, cartridge address space
- [NGPC Flash Board (NeoGeo Development Wiki)](https://wiki.neogeodev.org/index.php?title=NGPC_flash_board) -
  Cartridge flash memory, connector pinout
- [SNK Neo Geo Pocket Hardware Information (Data Crystal)](https://datacrystal.tcrf.net/wiki/SNK_Neo_Geo_Pocket/Hardware_information) -
  System specifications, ROM sizes
- [AMD Am29F040 datasheet](https://datasheet.octopart.com/AM29F040-90JC-AMD-datasheet-18512040.pdf) -
  4 Mbit flash command protocol, status polling (DQ7/DQ6/DQ5), sector
  erase, block protection
- [AMD Am29F080 datasheet (Am29F080B)](https://www.alldatasheet.com/datasheet-pdf/pdf/55463/AMD/AM29F080B-90EC.html) -
  8 Mbit flash, same command protocol as Am29F040
- [AMD Am29F016 datasheet](https://www.alldatasheet.com/datasheet-pdf/pdf/92562/AMD/AM29F016.html) -
  16 Mbit flash, same command protocol as Am29F040
- The cartridge chips are Toshiba TC58FVT series (TC58FVT004 / TC58FVT800 /
  TC58FVT160) which implement the AMD-compatible command interface
- Direct ROM file analysis: Puzzle Link (512 KB), Bust-A-Move Pocket (1 MB),
  Infinity Cure (1 MB), Dark Arms (2 MB), KOF R-1 (2 MB), Card Fighters
  Clash (2 MB)
