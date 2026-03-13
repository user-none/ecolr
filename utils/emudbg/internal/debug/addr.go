package debug

import (
	"strconv"
	"strings"
)

// ParseHexAddr parses a hex address string, stripping optional "$", "0x", or
// "0X" prefixes.
func ParseHexAddr(s string) (uint32, error) {
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	v, err := strconv.ParseUint(s, 16, 32)
	return uint32(v), err
}
