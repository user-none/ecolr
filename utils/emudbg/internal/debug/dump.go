package debug

import (
	"fmt"
	"os"

	"github.com/user-none/ecolr/core"
	"github.com/user-none/ecolr/core/tlcs900h"
)

// DumpExitState prints the full system state when the BIOS exits to cart code.
// If dumpPath is non-empty, a binary dump is also written.
func DumpExitState(mem *core.Memory, c *tlcs900h.CPU, dumpPath string) {
	regs := c.Registers()

	fmt.Println("=== CPU Registers ===")
	for bank := 0; bank < 4; bank++ {
		b := regs.Bank[bank]
		fmt.Printf("Bank %d: XWA=%08X XBC=%08X XDE=%08X XHL=%08X\n",
			bank, b.XWA, b.XBC, b.XDE, b.XHL)
	}
	fmt.Printf("XIX=%08X XIY=%08X XIZ=%08X XSP=%08X\n",
		regs.XIX, regs.XIY, regs.XIZ, regs.XSP)

	iff := (regs.SR >> 12) & 0x07
	rfp := (regs.SR >> 8) & 0x07
	sysm := regs.SR >> 15
	max := (regs.SR >> 11) & 1
	fmt.Printf("PC=%06X SR=%04X [S=%d MAX=%d RFP=%d IFF=%d] F=%s F'=%s\n",
		regs.PC, regs.SR, sysm, max, rfp, iff,
		FormatFlags(uint8(regs.SR)), FormatFlags(regs.FP))

	// Interrupt controller registers ($70-$7A)
	fmt.Println("\n=== Interrupt Controller ($70-$7A) ===")
	intcNames := []string{
		"INTE0AD", "INTE45", "INTE67", "INTET01",
		"INTET23", "INTET45", "INTET67", "INTECL01",
		"INTECL23", "INTES0", "INTES1",
	}
	for i := 0; i < 11; i++ {
		addr := uint32(0x70 + i)
		val := mem.Read(1, addr)
		fmt.Printf("  $%02X %-8s = $%02X\n", addr, intcNames[i], val)
	}

	// Timer registers
	fmt.Println("\n=== Timer Registers ===")
	timerRegs := []struct {
		addr uint32
		name string
	}{
		{0x20, "TRUN"}, {0x22, "TREG0"}, {0x23, "TREG1"},
		{0x24, "T01MOD"}, {0x25, "TFFCR"},
		{0x26, "TREG2"}, {0x27, "TREG3"},
		{0x28, "T23MOD"}, {0x29, "TRDC"},
		{0x30, "TREG4L"}, {0x31, "TREG4H"},
		{0x32, "TREG5L"}, {0x33, "TREG5H"},
		{0x38, "T4MOD"}, {0x39, "T4FFCR"}, {0x3A, "T45CR"},
	}
	for _, tr := range timerRegs {
		val := mem.Read(1, tr.addr)
		fmt.Printf("  $%02X %-8s = $%02X\n", tr.addr, tr.name, val)
	}

	// Key SFR/IO registers
	fmt.Println("\n=== Key I/O Registers ===")
	ioRegs := []struct {
		addr uint32
		name string
	}{
		{0xA2, "DACL"}, {0xA3, "DACR"},
		{0xB0, "INPUT"}, {0xB1, "POWER"},
		{0xB3, "NMICFG"}, {0xB8, "SNDEN"}, {0xB9, "Z80EN"},
	}
	for _, ir := range ioRegs {
		val := mem.Read(1, ir.addr)
		fmt.Printf("  $%02X %-8s = $%02X\n", ir.addr, ir.name, val)
	}

	// Work RAM key regions
	fmt.Println("\n=== Work RAM - System State ($6C00-$6CFF) ===")
	DumpMemHex(mem, 0x6C00, 0x100)

	fmt.Println("\n=== Work RAM - BIOS State ($6F80-$6FFF) ===")
	DumpMemHex(mem, 0x6F80, 0x80)

	fmt.Println("\n=== RAM Vector Table ($6FB8-$6FFC) ===")
	vecNames := []struct {
		addr uint32
		name string
	}{
		{0x6FB8, "SWI 3"}, {0x6FBC, "SWI 4"},
		{0x6FC0, "SWI 5"}, {0x6FC4, "SWI 6"},
		{0x6FC8, "INT0"}, {0x6FCC, "INT4/VBlank"},
		{0x6FD0, "INT5/Z80"}, {0x6FD4, "INTT0"},
		{0x6FD8, "INTT1"}, {0x6FDC, "INTT2"},
		{0x6FE0, "INTT3"}, {0x6FE4, "SerTX1"},
		{0x6FE8, "SerRX1"}, {0x6FEC, "(rsv)"},
		{0x6FF0, "DMA0"}, {0x6FF4, "DMA1"},
		{0x6FF8, "DMA2"}, {0x6FFC, "DMA3"},
	}
	for _, v := range vecNames {
		handler := mem.Read(4, v.addr)
		fmt.Printf("  $%04X %-12s -> $%06X\n", v.addr, v.name, handler)
	}

	// Stack contents (top 64 bytes)
	sp := regs.XSP
	fmt.Printf("\n=== Stack (SP=$%08X, top 64 bytes) ===\n", sp)
	if sp >= 0x4000 && sp <= 0x6FFF {
		DumpMemHex(mem, sp, 64)
	} else {
		fmt.Printf("  SP outside work RAM range\n")
	}

	// Binary dump to file if requested
	if dumpPath != "" {
		WriteBinaryDump(mem, dumpPath)
	}
}

// DumpMemHex prints a hex dump of memory at the given address and length.
func DumpMemHex(mem *core.Memory, addr uint32, length int) {
	for off := 0; off < length; off += 16 {
		fmt.Printf("  $%06X:", addr+uint32(off))
		for i := 0; i < 16 && off+i < length; i++ {
			val := mem.Read(1, addr+uint32(off+i))
			fmt.Printf(" %02X", val)
		}
		fmt.Println()
	}
}

// WriteBinaryDump writes the full work RAM, Z80 RAM, K2GE, and SFR regions
// to a binary file for offline analysis.
func WriteBinaryDump(mem *core.Memory, path string) {
	// Layout: work RAM (12KB) + Z80 RAM (4KB) + K2GE (16KB) + SFR (128B) + custom IO (128B)
	regions := []struct {
		start uint32
		size  int
	}{
		{0x4000, 0x3000}, // work RAM
		{0x7000, 0x1000}, // Z80 RAM
		{0x8000, 0x4000}, // K2GE
		{0x0000, 0x80},   // SFR
		{0x0080, 0x80},   // custom IO
	}

	var buf []byte
	for _, r := range regions {
		for i := 0; i < r.size; i++ {
			val := mem.Read(1, r.start+uint32(i))
			buf = append(buf, uint8(val))
		}
	}

	if err := os.WriteFile(path, buf, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing dump: %v\n", err)
		return
	}
	fmt.Printf("\nBinary dump written to %s (%d bytes)\n", path, len(buf))
	fmt.Println("Layout: work RAM $4000-$6FFF (12KB) | Z80 RAM $7000-$7FFF (4KB) | K2GE $8000-$BFFF (16KB) | SFR $00-$7F (128B) | IO $80-$FF (128B)")
}
