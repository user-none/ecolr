package debug

import (
	"fmt"
	"strings"

	"github.com/user-none/ecolr/core/tlcs900h"
)

// Disassemble prints count disassembled instructions starting at addr.
func Disassemble(bus tlcs900h.Bus, addr uint32, count int) {
	for i := 0; i < count; i++ {
		d := tlcs900h.Disasm(bus, addr)
		var hexBytes []string
		for _, b := range d.Bytes {
			hexBytes = append(hexBytes, fmt.Sprintf("%02X", b))
		}
		bytesStr := strings.Join(hexBytes, " ")
		fmt.Printf("$%06X: %-12s %s\n", d.Addr, bytesStr, d.Text)
		addr += uint32(len(d.Bytes))
	}
}
