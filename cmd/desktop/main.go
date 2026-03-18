//go:build !libretro && !ios

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/user-none/eblitui/desktop"
	"github.com/user-none/ecolr/adapter"
)

func main() {
	romPath := flag.String("rom", "", "path to ROM file (opens UI if not provided)")
	biosPath := flag.String("bios", "", "path to BIOS ROM (RunDirect only)")
	firstBoot := flag.Bool("first-boot", false, "run BIOS setup screens")
	flag.Parse()

	factory := adapter.NewFactory()

	if *romPath != "" {
		options := map[string]string{}
		if *firstBoot {
			options["first_boot"] = "true"
		}
		var biosMap map[string][]byte
		if *biosPath != "" {
			data, err := os.ReadFile(*biosPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading bios: %v\n", err)
				os.Exit(1)
			}
			biosMap = map[string][]byte{"system_bios": data}
		}
		if err := desktop.RunDirect(factory, *romPath, options, biosMap); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := desktop.Run(factory); err != nil {
		log.Fatal(err)
	}
}
