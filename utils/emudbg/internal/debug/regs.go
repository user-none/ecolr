package debug

import (
	"fmt"
	"strings"

	"github.com/user-none/ecolr/core/tlcs900h"
)

// FormatFlags returns a 6-character string showing the S, Z, H, V, N, C flags.
func FormatFlags(f uint8) string {
	flags := [6]struct {
		mask uint8
		ch   byte
	}{
		{0x80, 'S'}, // Sign
		{0x40, 'Z'}, // Zero
		{0x10, 'H'}, // Half-carry
		{0x04, 'V'}, // Overflow
		{0x02, 'N'}, // Subtract
		{0x01, 'C'}, // Carry
	}
	buf := make([]byte, 6)
	for i, fl := range flags {
		if f&fl.mask != 0 {
			buf[i] = fl.ch
		} else {
			buf[i] = '-'
		}
	}
	return string(buf)
}

// PrintRegs prints all CPU registers in a human-readable format.
func PrintRegs(r tlcs900h.Registers) {
	bank := int((r.SR & 0x0700) >> 8)
	b := r.Bank[bank]
	fmt.Printf("Bank %d: XWA=%08X XBC=%08X XDE=%08X XHL=%08X\n",
		bank, b.XWA, b.XBC, b.XDE, b.XHL)
	fmt.Printf("        XIX=%08X XIY=%08X XIZ=%08X XSP=%08X\n",
		r.XIX, r.XIY, r.XIZ, r.XSP)

	iff := (r.SR >> 12) & 0x07
	rfp := (r.SR >> 8) & 0x07
	sysm := r.SR >> 15
	max := (r.SR >> 11) & 1

	fmt.Printf("PC=%06X SR=%04X [S=%d MAX=%d RFP=%d IFF=%d]  F=%s F'=%s\n",
		r.PC, r.SR, sysm, max, rfp, iff,
		FormatFlags(uint8(r.SR)), FormatFlags(r.FP))
}

// PrintDiff prints any register changes between before and after states.
func PrintDiff(before, after tlcs900h.Registers) {
	var changes []string

	if after.PC != before.PC {
		changes = append(changes, fmt.Sprintf("PC=%06X", after.PC))
	}
	if after.SR != before.SR {
		changes = append(changes, fmt.Sprintf("SR=%04X", after.SR))
	}
	if after.XSP != before.XSP {
		changes = append(changes, fmt.Sprintf("XSP=%08X", after.XSP))
	}
	if after.XIX != before.XIX {
		changes = append(changes, fmt.Sprintf("XIX=%08X", after.XIX))
	}
	if after.XIY != before.XIY {
		changes = append(changes, fmt.Sprintf("XIY=%08X", after.XIY))
	}
	if after.XIZ != before.XIZ {
		changes = append(changes, fmt.Sprintf("XIZ=%08X", after.XIZ))
	}
	if after.FP != before.FP {
		changes = append(changes, fmt.Sprintf("FP=%02X", after.FP))
	}

	for i := 0; i < 4; i++ {
		if after.Bank[i].XWA != before.Bank[i].XWA {
			changes = append(changes, fmt.Sprintf("Bank%d.XWA=%08X", i, after.Bank[i].XWA))
		}
		if after.Bank[i].XBC != before.Bank[i].XBC {
			changes = append(changes, fmt.Sprintf("Bank%d.XBC=%08X", i, after.Bank[i].XBC))
		}
		if after.Bank[i].XDE != before.Bank[i].XDE {
			changes = append(changes, fmt.Sprintf("Bank%d.XDE=%08X", i, after.Bank[i].XDE))
		}
		if after.Bank[i].XHL != before.Bank[i].XHL {
			changes = append(changes, fmt.Sprintf("Bank%d.XHL=%08X", i, after.Bank[i].XHL))
		}
	}

	if len(changes) > 0 {
		fmt.Printf("  -> %s\n", strings.Join(changes, " "))
	}
}
