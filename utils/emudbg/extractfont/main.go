package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	fontOffset = 0x8DCF
	fontSize   = 2048    // 256 chars x 8 bytes
	biosSize   = 0x10000 // 64 KB
)

func main() {
	biosPath := flag.String("bios", "", "path to BIOS ROM (64 KB)")
	output := flag.String("output", "ngpc_bios_font.bin", "output file path")
	flag.Parse()

	if *biosPath == "" {
		fmt.Fprintln(os.Stderr, "usage: extractfont -bios <path> [-output <path>]")
		os.Exit(1)
	}

	bios, err := os.ReadFile(*biosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read BIOS: %v\n", err)
		os.Exit(1)
	}

	if len(bios) != biosSize {
		fmt.Fprintf(os.Stderr, "expected %d bytes, got %d\n", biosSize, len(bios))
		os.Exit(1)
	}

	font := bios[fontOffset : fontOffset+fontSize]

	if err := os.WriteFile(*output, font, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Extracted %d bytes from offset $%04X to %s\n", fontSize, fontOffset, *output)
}
