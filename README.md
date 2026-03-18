# ecolr

A Neo Geo Pocket / Neo Geo Pocket Color emulator written in Go.

## Project Overview

The focus is on core emulation accuracy for officially licensed and released
games. The desktop UI provides a game library with ROM scanning, artwork,
metadata, save state management, rewind, shader effects, RetroAchievements
integration, and configurable settings.

## Target Hardware

This emulator targets the **Neo Geo Pocket Color** handheld console.

- **Main CPU:** Toshiba TLCS-900/H (TMP95C061) at 6.144 MHz
- **Sound CPU:** Zilog Z80 at 3.072 MHz
- **Sound:** T6W28 PSG (SN76489 variant) with stereo output + DAC channels
- **Graphics:** K2GE (K1GE color mode) with sprites and scroll planes
- **Display:** 160x152 pixel LCD, 1:1 pixel aspect ratio
- **Cartridge:** AMD-style flash ROM (512 KB to 4 MB) with in-system saves

## Features

- Full TLCS-900/H CPU emulation with instruction decode, execute, disassembly,
  and interrupt handling
- Z80 sound CPU running in lockstep with PSG for accurate register timing
- T6W28 PSG stereo output mixed with DAC channels at 48 kHz
- K2GE scanline rendering with sprites, scroll planes, and monochrome
  compatibility mode
- AMD-style flash ROM emulation with block erase, byte program, and chip ID
- NGF block-delta save format for efficient flash persistence
- TMP95C061 peripherals: interrupt controller, 8-bit timers with prescaler,
  A/D converter, real-time clock
- HLE BIOS for running without a BIOS ROM
- Real BIOS support with fast boot and first boot setup options
- Save state serialization
- NTSC timing at ~60 Hz (515 clocks/scanline, 199 scanlines/frame)

## Building

Requires Go 1.25+.

### Desktop Application

```
make desktop
```

Produces `build/ecolr`.

### macOS Application Bundle

```
make macos
```

Creates `build/ecolr.app` with icon and code signing.

### All Targets

| Target | Description |
|--------|-------------|
| `make desktop` | Build desktop binary to `build/ecolr` |
| `make macos` | Build macOS .app bundle to `build/ecolr.app` |
| `make icons` | Generate macOS icon from `assets/icon.png` |
| `make clean` | Remove build directory |

## Running

### Launch the full UI

```
ecolr
```

### Direct (no UI)

```
ecolr -rom game.ngc
```

With a BIOS:

```
ecolr -rom game.ngc -bios ngpc_bios.bin
```

## BIOS

The system BIOS is optional. When provided, the emulator uses the real BIOS
boot sequence. Without a BIOS ROM, HLE mode synthesizes the necessary
initialization and system call handling.

### Options

| Option | Description |
|--------|-------------|
| First Boot | Run the BIOS setup screens (language and color selection) |
| Fast Boot | Skip BIOS animation screens and boot directly to game |
| Language | English or Japanese |
| Monochrome Palette | Color scheme for NGP games (Black & White, Red, Green, Blue, Classic) |

## Controls

**Keyboard:**

| Action | Key |
|--------|-----|
| Up | W |
| Down | S |
| Left | A |
| Right | D |
| Button A | J |
| Button B | K |
| Option | Enter |

**Gamepad:**

| Action | Button |
|--------|--------|
| Up | D-pad / Left Stick |
| Down | D-pad / Left Stick |
| Left | D-pad / Left Stick |
| Right | D-pad / Left Stick |
| Button A | A / Cross |
| Button B | B / Circle |
| Option | Start |

## ROM Support

- Neo Geo Pocket Color (.ngc)
- Neo Geo Pocket (.ngp)

## Emulated Hardware

### CPUs

| Chip | Role | Clock |
|------|------|-------|
| TLCS-900/H (TMP95C061) | Main CPU | 6.144 MHz |
| Zilog Z80 | Sound CPU | 3.072 MHz |

The TLCS-900/H runs per-scanline with budget-based execution and clock gear
scaling. The Z80 runs in sync per scanline with fixed-point cycle accumulation.

### Graphics (K2GE)

- 160x152 pixel resolution
- Sprite and scroll plane rendering
- K1GE monochrome compatibility mode with configurable palettes
- Scanline-based rendering

### Audio

| Component | Type | Output |
|-----------|------|--------|
| T6W28 PSG | SN76489 variant | Stereo |
| DAC | 8-bit left/right channels | Stereo |

- 48 kHz sample rate, 16-bit stereo PCM
- Z80 and PSG run in lockstep for accurate register timing
- DAC mixed with PSG output per-scanline

### TMP95C061 Peripherals

- Interrupt controller with priority-based dispatch
- 8-bit timers (T0-T3) with prescaler, cascade, and external input modes
- A/D converter
- Real-time clock with BCD time/date registers

### Memory Map (TLCS-900/H)

| Address Range | Size | Description |
|---------------|------|-------------|
| $000000-$0000FF | 256 B | SFR + custom I/O registers |
| $004000-$006FFF | 12 KB | Work RAM |
| $007000-$007FFF | 4 KB | Z80 shared RAM |
| $008000-$00BFFF | 16 KB | K2GE VRAM |
| $200000-$3FFFFF | 2 MB | Cartridge CS0 |
| $800000-$9FFFFF | 2 MB | Cartridge CS1 (4 MB ROMs only) |
| $FF0000-$FFFFFF | 64 KB | BIOS ROM |

### Region

| Region | Scanlines | FPS | CPU Clock | Z80 Clock |
|--------|-----------|-----|-----------|-----------|
| NTSC | 199 | ~60 | 6.144 MHz | 3.072 MHz |

The NGPC has no PAL variant.

## Architecture

```
cmd/
  desktop/             Desktop entry point (Ebiten UI)
adapter/
  adapter.go           CoreFactory: system info, emulator creation, region detection
core/                  Core emulator (platform-independent)
  tlcs900h/              TLCS-900/H CPU (decode, execute, registers, disasm, interrupts)
  emulator.go            Main loop: per-scanline CPU sync, interrupt dispatch, audio mix
  mem.go                 Memory map: BIOS, cart, work RAM, Z80 RAM, K2GE, SFR I/O
  flash.go               AMD-style flash ROM emulation
  ngf.go                 NGF block-delta save format for flash persistence
  k2ge.go                K2GE graphics engine (sprites, scroll planes, scanline rendering)
  t6w28.go               T6W28 PSG (SN76489 variant) with stereo output
  z80bus.go              Z80 sound CPU bus adapter
  intc.go                TMP95C061 interrupt controller
  timer.go               TMP95C061 8-bit timers with prescaler
  adc.go                 TMP95C061 A/D converter
  rtc.go                 Real-time clock
  hle.go                 HLE BIOS for running without a BIOS ROM
  sysstate.go            System state initialization
  serialize.go           Save state serialization
  region.go              NTSC timing constants
  version.go             Application name and version
assets/
  icon.png               Application icon
  macos_info.plist       macOS app bundle metadata
docs/                    Technical reference documentation
utils/emudbg/            Debug and analysis utilities
```

The core `core/` package is platform-independent. It implements the
`eblitui/coreif` interfaces:

- **Emulator** - frame execution, framebuffer, audio, input
- **SaveStater** - save and load state with CRC verification
- **BatterySaver** - flash save get/set (NGF format)
- **MemoryInspector** - flat address read for RetroAchievements
- **MemoryMapper** - enumerate and access memory regions

The `adapter/` package bridges between the core and the UI frameworks by
implementing `CoreFactory`. The `cmd/` packages are thin entry points that
wire the adapter to a specific frontend.

## Testing

```
go test ./core/... -count=1
```

## Compatibility

This emulator targets officially licensed and released Neo Geo Pocket and
Neo Geo Pocket Color games.

**Not supported:**

- Unlicensed or homebrew games
- Prototype or beta ROMs

## Dependencies

- [eblitui](https://github.com/user-none/eblitui) - shared emulator UI
  framework (desktop)
- [go-chip-z80](https://github.com/user-none/go-chip-z80) - Zilog Z80 CPU
- [go-chip-sn76489](https://github.com/user-none/go-chip-sn76489) - T6W28
  PSG base

## Debug Tools

Debug and analysis utilities live under `utils/emudbg/`. They are part of
the main module and can be run with `go run`.

### biostrace

Steps through the BIOS boot sequence instruction-by-instruction, simulating
VBlank interrupts and Z80 sound CPU timing. Reports when the BIOS exits to
the cartridge entry point or crashes.

```
go run ./utils/emudbg/biostrace -bios ngpc_bios.bin -cart game.ngc -force-cart -mirror-cart-header -auto-input
```

Flags:

- `-bios <path>` - Path to BIOS ROM (required)
- `-cart <path>` - Path to cartridge ROM
- `-steps N` - Max instructions to step (0 = unlimited)
- `-log` - Enable per-instruction disassembly logging
- `-log-range X-Y` - Only log steps in the given range (implies `-log`)
- `-trace-sound` - Log sound system state at each VBlank
- `-auto-input` - Simulate A button presses to advance past setup screen input waits
- `-dump <path>` - Write binary RAM/state dump on BIOS exit
- `-render-frame N:file.png` - Render frame at VBlank N to PNG (comma-separated for multiple)
- `-pc <hex>` - Override starting PC
- `-watch-pc <hex,...>` - Log when PC hits any of the given addresses
- `-force-cart` - Pin `$6C46` to `$FF` so the BIOS sees a cartridge present
- `-mirror-cart-header` - Mirror BIOS cart header fields (software ID and title)
- `-mirror <dst:src,...>` - Arbitrary address mirrors (comma-separated hex pairs)
- `-pin <addr:val,...>` - Pin memory addresses to fixed byte values (comma-separated hex pairs)

### disasm

Disassembles TLCS-900/H instructions from a BIOS or cartridge ROM.

```
go run ./utils/emudbg/disasm -bios ngpc_bios.bin -addr FF0000 -count 20
```

Flags:

- `-bios <path>` - Path to BIOS ROM
- `-cart <path>` - Path to cartridge ROM
- `-addr <hex>` - Address to start disassembly
- `-count N` - Number of instructions to disassemble (default 20)

At least one of `-bios` or `-cart` is required.

### extractfont

Extracts the 8x8 1bpp bitmap font (256 glyphs, 8 bytes each) from the BIOS ROM.

```
go run ./utils/emudbg/extractfont -bios ngpc_bios.bin
```

Flags:

- `-bios <path>` - Path to BIOS ROM (required)
- `-output <path>` - Output file path (default `ngpc_bios_font.bin`)
