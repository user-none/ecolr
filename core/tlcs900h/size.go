package tlcs900h

// Size represents an operand width.
type Size int

const (
	Byte Size = 1
	Word Size = 2
	Long Size = 4
)

// Mask returns a bitmask covering valid bits for the size.
func (s Size) Mask() uint32 {
	switch s {
	case Byte:
		return 0xFF
	case Word:
		return 0xFFFF
	case Long:
		return 0xFFFFFFFF
	default:
		return 0
	}
}

// MSB returns the most-significant bit for the size.
func (s Size) MSB() uint32 {
	switch s {
	case Byte:
		return 0x80
	case Word:
		return 0x8000
	case Long:
		return 0x80000000
	default:
		return 0
	}
}

// Bits returns the number of bits for the size.
func (s Size) Bits() uint32 {
	return uint32(s) * 8
}

// String returns a human-readable name.
func (s Size) String() string {
	switch s {
	case Byte:
		return "Byte"
	case Word:
		return "Word"
	case Long:
		return "Long"
	default:
		return "?"
	}
}
