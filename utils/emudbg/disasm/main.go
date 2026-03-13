package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/user-none/ecolr/core"
	"github.com/user-none/ecolr/utils/emudbg/internal/debug"
)

func main() {
	biosPath := flag.String("bios", "", "path to BIOS ROM (64 KB)")
	cartPath := flag.String("cart", "", "path to cartridge ROM (optional)")
	addrStr := flag.String("addr", "", "hex address to disassemble at (e.g. FF204A)")
	count := flag.Int("count", 20, "number of instructions to disassemble")
	flag.Parse()

	if *biosPath == "" && *cartPath == "" {
		fmt.Fprintln(os.Stderr, "usage: disasm [-bios <path>] [-cart <path>] [-addr <hex>] [-count N]")
		fmt.Fprintln(os.Stderr, "at least one of -bios or -cart is required")
		os.Exit(1)
	}

	var bios []byte
	if *biosPath != "" {
		var err error
		bios, err = os.ReadFile(*biosPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading bios: %v\n", err)
			os.Exit(1)
		}
	}

	var cart []byte
	if *cartPath != "" {
		var err error
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
	if bios != nil {
		mem.SetBIOS(bios)
	}

	var addr uint32
	if *addrStr != "" {
		var err error
		addr, err = debug.ParseHexAddr(*addrStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -addr address %q: %v\n", *addrStr, err)
			os.Exit(1)
		}
	}

	debug.Disassemble(mem, addr, *count)
}
