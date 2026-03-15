package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/user-none/ecolr/core"
	"github.com/user-none/ecolr/core/tlcs900h"
	"github.com/user-none/ecolr/utils/emudbg/internal/debug"
)

func main() {
	biosPath := flag.String("bios", "", "path to BIOS ROM (64 KB)")
	cartPath := flag.String("cart", "", "path to cartridge ROM (optional)")
	steps := flag.Int("steps", 0, "max instructions to step (0 = unlimited)")
	logEnabled := flag.Bool("log", false, "enable disassembly logging")
	logRange := flag.String("log-range", "", "step range to log (e.g. 100-200); implies -log")
	traceSound := flag.Bool("trace-sound", false, "log sound system state at each VBlank")
	autoInput := flag.Bool("auto-input", false, "simulate A button press to advance past setup screen input waits")
	dumpFile := flag.String("dump", "", "write binary RAM/state dump to file on BIOS exit")
	renderFrame := flag.String("render-frame", "", "render frame at VBlank N to PNG (N:file.png); comma-separated for multiple")
	startPC := flag.String("pc", "", "override starting PC (hex address, e.g. FF1800)")
	watchPCs := flag.String("watch-pc", "", "comma-separated hex addresses to log when hit")
	forceCart := flag.Bool("force-cart", false, "pin $6C46 to $FF so BIOS sees a cart present")
	mirrorCartHeader := flag.Bool("mirror-cart-header", false, "mirror BIOS cart header fields (software ID + title)")
	mirrorFlag := flag.String("mirror", "", "comma-separated dst:src hex address pairs (e.g. 6C69:6C04,6C6A:6C05)")
	pinFlag := flag.String("pin", "", "comma-separated addr:val hex pairs to pin (e.g. 6C46:FF,6C47:01)")
	flag.Parse()

	if *biosPath == "" {
		fmt.Fprintln(os.Stderr, "usage: biostrace -bios <path> [-cart <path>] [-steps N] [-log] [-log-range X-Y] [-auto-input] [-dump FILE]")
		os.Exit(1)
	}

	maxSteps := *steps
	logStart, logEnd := 0, maxSteps
	if *logRange != "" {
		n, err := fmt.Sscanf(*logRange, "%d-%d", &logStart, &logEnd)
		if n != 2 || err != nil {
			fmt.Fprintf(os.Stderr, "invalid -log-range %q: use X-Y (e.g. 100-200)\n", *logRange)
			os.Exit(1)
		}
		*logEnabled = true
	}
	if *logEnabled && logEnd == 0 {
		logEnd = int(^uint(0) >> 1) // max int
	}

	// Parse -render-frame N:file.png or N:file.png,N:file.png,...
	renderFrames := map[int]string{}
	if *renderFrame != "" {
		for _, entry := range strings.Split(*renderFrame, ",") {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "invalid -render-frame entry %q: use N:file.png\n", entry)
				os.Exit(1)
			}
			n, err := strconv.Atoi(parts[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -render-frame number %q: %v\n", parts[0], err)
				os.Exit(1)
			}
			renderFrames[n] = parts[1]
		}
	}

	bios, err := os.ReadFile(*biosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading bios: %v\n", err)
		os.Exit(1)
	}

	var cart []byte
	if *cartPath != "" {
		cart, err = os.ReadFile(*cartPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading cart: %v\n", err)
			os.Exit(1)
		}
	}

	mem, err := core.NewMemory(cart, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating memory: %v\n", err)
		os.Exit(1)
	}
	mem.SetBIOS(bios)

	bus := debug.NewDebugBus(mem)

	if *forceCart {
		bus.Pin(0x6C46, 0xFF)
	}

	if *mirrorCartHeader {
		// Software ID: $6C69-$6C6A -> $6C04-$6C05 (2 bytes)
		bus.Mirror(0x6C69, 0x6C04)
		bus.Mirror(0x6C6A, 0x6C05)
		// Title: $6C6C-$6C77 -> $6C08-$6C13 (12 bytes)
		for i := uint32(0); i < 12; i++ {
			bus.Mirror(0x6C6C+i, 0x6C08+i)
		}
	}

	if *mirrorFlag != "" {
		for _, entry := range strings.Split(*mirrorFlag, ",") {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "invalid -mirror entry %q: use dst:src (hex)\n", entry)
				os.Exit(1)
			}
			dst, err := debug.ParseHexAddr(strings.TrimSpace(parts[0]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -mirror dst %q: %v\n", parts[0], err)
				os.Exit(1)
			}
			src, err := debug.ParseHexAddr(strings.TrimSpace(parts[1]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -mirror src %q: %v\n", parts[1], err)
				os.Exit(1)
			}
			bus.Mirror(dst, src)
		}
	}

	if *pinFlag != "" {
		for _, entry := range strings.Split(*pinFlag, ",") {
			parts := strings.SplitN(entry, ":", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "invalid -pin entry %q: use addr:val (hex)\n", entry)
				os.Exit(1)
			}
			addr, err := debug.ParseHexAddr(strings.TrimSpace(parts[0]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -pin addr %q: %v\n", parts[0], err)
				os.Exit(1)
			}
			val, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 16, 8)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -pin val %q: %v\n", parts[1], err)
				os.Exit(1)
			}
			bus.Pin(addr, uint8(val))
		}
	}

	c := tlcs900h.New(bus)
	c.LoadResetVector()
	mem.SetCPU(c)

	if *startPC != "" {
		pcVal, err := debug.ParseHexAddr(*startPC)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -pc address %q: %v\n", *startPC, err)
			os.Exit(1)
		}
		regs := c.Registers()
		regs.PC = pcVal
		c.SetState(regs)
	}

	// Parse watch-pc addresses
	watchSet := map[uint32]bool{}
	if *watchPCs != "" {
		for _, s := range strings.Split(*watchPCs, ",") {
			addr, err := debug.ParseHexAddr(strings.TrimSpace(s))
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid -watch-pc address %q: %v\n", s, err)
				os.Exit(1)
			}
			watchSet[addr] = true
		}
	}

	regs := c.Registers()

	fmt.Println("=== Initial State ===")
	debug.PrintRegs(regs)
	fmt.Println()

	// VBlank simulation constants derived from K2GE timing.
	// 515 CPU clocks per scanline, 199 lines per frame (152 active + 47 VBlank).
	const (
		clocksPerScanline = 515
		activeScanlines   = 152
		totalScanlines    = 199
		clocksPerFrame    = clocksPerScanline * totalScanlines // 102,485
	)

	z80cpu := mem.Z80CPU()

	var (
		prevCycles uint64
		frameClock int // cycles within current frame (0 to clocksPerFrame-1)
		prevLine   int // previous scanline number
		z80Budget  int // accumulated Z80 cycles to run
		vblankNum  int // VBlank frame counter
		inputHeld  int // frames remaining to hold simulated input
	)

	step := 0
	for maxSteps == 0 || step < maxSteps {
		before := c.Registers()
		if watchSet[before.PC] {
			fmt.Printf("  [WATCH] step %d: PC=$%06X\n", step, before.PC)
		}
		c.Step()
		mem.Tick()

		afterPC := c.Registers().PC
		if afterPC < 0xFF0000 && afterPC != 0 {
			fmt.Printf("\n=== BIOS EXIT at step %d ===\n", step)
			fmt.Printf("PC=$%06X (left BIOS ROM range)\n\n", afterPC)
			debug.DumpExitState(mem, c, *dumpFile)
			break
		}
		if afterPC == 0 {
			fmt.Printf("\n=== CRASH at step %d ===\n", step)
			fmt.Printf("PC=$000000 (invalid jump to zero)\n\n")
			debug.DumpExitState(mem, c, "")
			break
		}

		// Advance VBlank simulation based on elapsed CPU cycles.
		nowCycles := c.Cycles()
		elapsed := int(nowCycles - prevCycles)
		prevCycles = nowCycles

		// Step Z80 sound CPU at half the main CPU clock rate.
		if mem.Z80Active() {
			z80Budget += elapsed / 2

			// Deliver pending Timer 3 (TO3) IRQs before normal stepping.
			pending := mem.Z80IRQPending()
			for pending > 0 && z80Budget > 0 {
				z80cpu.INT(true, 0xFF)
				for z80Budget > 0 {
					z80Budget -= z80cpu.Step()
					if !z80cpu.Registers().IFF1 {
						break
					}
				}
				z80cpu.INT(false, 0xFF)
				pending--
			}

			for z80Budget > 0 {
				z80Budget -= z80cpu.Step()
			}
		}

		frameClock += elapsed
		for frameClock >= clocksPerFrame {
			frameClock -= clocksPerFrame
		}

		curLine := frameClock / clocksPerScanline
		inVBlank := curLine >= activeScanlines
		wasInVBlank := prevLine >= activeScanlines
		mem.SetVBlankStatus(inVBlank)

		if inVBlank && !wasInVBlank {
			vblankNum++
			mem.RequestINT4()
			mem.Tick()

			if path, ok := renderFrames[vblankNum]; ok {
				debug.WritePNGFrame(mem, path, vblankNum)
			}

			if *traceSound {
				b9 := mem.Read8(0xB9)
				b8 := mem.Read8(0xB8)
				da2 := mem.Read8(0x6DA2)
				da0 := mem.Read8(0x6DA0)
				da1 := mem.Read8(0x6DA1)
				bc := mem.Read8(0xBC)
				fmt.Printf("  [VBlank %d] step=%d B8=$%02X B9=$%02X $6DA2=$%02X head=$%02X tail=$%02X $BC=$%02X\n",
					vblankNum, step, b8, b9, da2, da0, da1, bc)
			}

			// Auto-input: simulate A button press to advance past
			// setup screen input waits.
			if *autoInput && inputHeld == 0 && vblankNum >= 30 && vblankNum%30 == 0 {
				mem.SetInput(0x10)
				inputHeld = 3
			}
			if inputHeld > 0 {
				inputHeld--
				if inputHeld == 0 {
					mem.SetInput(0x00) // release
				}
			}
		}
		prevLine = curLine

		if c.Halted() {
			mem.RequestINT0()
			mem.Tick()
			logging := *logEnabled && step >= logStart && step < logEnd
			if logging {
				fmt.Println("  [HALT - firing INT0]")
			}
		}

		logging := *logEnabled && step >= logStart && step < logEnd
		if logging {
			d := tlcs900h.Disasm(mem, before.PC)
			var hexBytes []string
			for _, b := range d.Bytes {
				hexBytes = append(hexBytes, fmt.Sprintf("%02X", b))
			}
			bytesStr := strings.Join(hexBytes, " ")
			fmt.Printf("$%06X: %-12s %s\n", d.Addr, bytesStr, d.Text)
			after := c.Registers()
			debug.PrintDiff(before, after)
		}

		step++
	}

	fmt.Println()
	fmt.Println("=== Final State ===")
	debug.PrintRegs(c.Registers())
}
