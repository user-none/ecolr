package tlcs900h

// RegisterBank holds one bank of general-purpose 32-bit registers.
// Each 32-bit register can be accessed as 32-bit (XWA), 16-bit (WA),
// or 8-bit (W, A) sub-widths.
type RegisterBank struct {
	XWA uint32
	XBC uint32
	XDE uint32
	XHL uint32
}

// Registers holds the complete CPU register state.
type Registers struct {
	// 4 banks of general-purpose registers.
	// Active bank selected by RFP field in SR (bits 10-8).
	Bank [4]RegisterBank

	// Dedicated (non-banked) registers.
	XIX uint32 // Index register X
	XIY uint32 // Index register Y
	XIZ uint32 // Index register Z
	XSP uint32 // Stack pointer
	PC  uint32 // Program counter (24-bit effective)
	SR  uint16 // Status register
	FP  uint8  // Alternate flags register (F')
}
