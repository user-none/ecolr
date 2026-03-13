package core

import (
	"github.com/user-none/ecolr/core/tlcs900h"
)

// InitSystemState writes post-boot hardware state to SFR, K2GE, system RAM,
// K1GE palettes, and GE mode. This represents the state after the real BIOS
// completes boot and is shared by both HLE and real BIOS fast boot paths.
func (m *Memory) InitSystemState() {
	m.initSFR()
	m.initK2GE()
	m.initSystemRAM()
	m.initK1GEPalettes()
	m.setGEMode()
}

// CartEntryPoint returns the cart entry point from the header at offset $1C
// (24-bit little-endian). Falls back to the cart base ($200000) if no cart
// is loaded.
func (m *Memory) CartEntryPoint() uint32 {
	if len(m.cart) > 0x1E {
		return uint32(m.cart[0x1C]) |
			uint32(m.cart[0x1D])<<8 |
			uint32(m.cart[0x1E])<<16
	}
	return 0x200000
}

// initSFR writes post-boot SFR and custom I/O register values.
// Values from memdump.bin at cart handoff point.
func (m *Memory) initSFR() {
	// Port registers
	m.writeIOByte(0x06, 0xFF) // P5FC
	m.writeIOByte(0x09, 0xFF) // P7
	m.writeIOByte(0x0D, 0x09) // P8CR
	m.writeIOByte(0x10, 0x34) // PACR
	m.writeIOByte(0x11, 0x3C) // PAFC
	m.writeIOByte(0x13, 0xFF) // PBCR

	// Memory controller
	m.writeIOByte(0x15, 0x3F) // MSAR0
	m.writeIOByte(0x18, 0x3F) // MSAR2
	m.writeIOByte(0x1A, 0x2D) // MAMR2
	m.writeIOByte(0x1B, 0x01) // DTEFCR
	m.writeIOByte(0x1E, 0x0F) // BEXCS
	m.writeIOByte(0x1F, 0xB2) // BCS

	// Timer registers
	m.writeIOByte(0x20, 0x80) // TRUN: prescaler enabled
	m.writeIOByte(0x22, 0x01) // TREG0
	m.writeIOByte(0x23, 0x90) // TREG1
	m.writeIOByte(0x24, 0x03) // T01MOD
	m.writeIOByte(0x25, 0xB0) // TFFCR
	m.writeIOByte(0x26, 0x90) // TREG2
	m.writeIOByte(0x27, 0x62) // TREG3
	m.writeIOByte(0x28, 0x05) // T23MOD
	m.writeIOByte(0x2C, 0x0C) // Timer capture
	m.writeIOByte(0x2D, 0x0C) // Timer capture
	m.writeIOByte(0x2E, 0x4C) // Timer flip-flop control 2
	m.writeIOByte(0x2F, 0x4C) // Timer flip-flop control 2
	m.writeIOByte(0x38, 0x30) // T4MOD
	m.writeIOByte(0x3C, 0x20) // TREG8L
	m.writeIOByte(0x3D, 0xFF) // TREG8H
	m.writeIOByte(0x3E, 0x80) // TREG9L
	m.writeIOByte(0x3F, 0x7F) // TREG9H
	m.writeIOByte(0x48, 0x30) // T89MOD

	// Serial
	m.writeIOByte(0x51, 0x20) // SC0BUF
	m.writeIOByte(0x52, 0x69) // SC0MOD
	m.writeIOByte(0x53, 0x15) // BR0CR

	// I/O function control
	m.writeIOByte(0x5C, 0xFF) // IOFC
	m.writeIOByte(0x5D, 0xFF) // Reserved
	m.writeIOByte(0x5E, 0xFF) // Reserved
	m.writeIOByte(0x5F, 0xFF) // Reserved

	// ADC result registers (conversion residuals from BIOS battery sampling)
	m.writeIOByte(0x60, 0xFF)
	m.writeIOByte(0x61, 0xFF)
	m.writeIOByte(0x62, 0x3F)
	m.writeIOByte(0x64, 0x3F)
	m.writeIOByte(0x66, 0x3F)

	// ADC mode registers
	m.writeIOByte(0x68, 0x17) // ADMOD1
	m.writeIOByte(0x69, 0x17) // ADMOD2
	m.writeIOByte(0x6A, 0x03) // ADREG0L
	m.writeIOByte(0x6B, 0x03) // ADREG0H
	m.writeIOByte(0x6C, 0x02) // ADREG1L

	// Watchdog
	m.writeIOByte(0x6E, 0xF0) // WDMOD
	m.writeIOByte(0x6F, 0x4E) // WDCR

	// Interrupt priorities
	m.writeIOByte(0x70, 0x02) // INTE0AD: INT0 priority = 2
	m.writeIOByte(0x71, 0x54) // INTE45: INT4=4, INT5=5
	m.writeIOByte(0x74, 0x80) // INTET23: INTT3 pending
	m.writeIOByte(0x7B, 0x04)

	// System control
	m.writeIOByte(0x90, 0xE0)
	m.writeIOByte(0x91, 0x99)
	m.writeIOByte(0x92, 0x01)
	m.writeIOByte(0x93, 0x01)
	m.writeIOByte(0x97, 0x20)

	// DAC
	m.writeIOByte(0xA0, 0xFF)
	m.writeIOByte(0xA1, 0xFF)
	m.writeIOByte(0xA2, 0x80) // DACL (silence midpoint)
	m.writeIOByte(0xA3, 0x80) // DACR (silence midpoint)

	// Custom I/O
	m.writeIOByte(0xB0, 0x00) // Input port (active-high, no buttons)
	m.writeIOByte(0xB1, 0x02)
	m.writeIOByte(0xB2, 0x01) // RTS disabled
	m.writeIOByte(0xB3, 0x04) // NMI enable (power button)
	m.writeIOByte(0xB4, 0x0A) // Custom I/O state
	m.writeIOByte(0xB6, 0x05) // Power state
	m.writeIOByte(0xB8, 0xAA) // T6W28 disabled
	m.writeIOByte(0xB9, 0xAA) // Z80 disabled
	m.writeIOByte(0xBC, 0xFE) // Custom I/O state
}

// initK2GE writes post-boot K2GE register values.
// Values from memdump.bin at cart handoff point.
func (m *Memory) initK2GE() {
	m.Write(tlcs900h.Byte, 0x8000, 0xC0) // VBlank+HBlank IRQ enabled
	m.Write(tlcs900h.Byte, 0x8006, 0xC6) // Frame rate control
	m.Write(tlcs900h.Byte, 0x8118, 0x80) // Background color on
	m.Write(tlcs900h.Byte, 0x8400, 0xFF) // LED on
	m.Write(tlcs900h.Byte, 0x87F4, 0x80) // K2GE config

	// Monochrome palette registers from memdump.bin.
	// These represent a user choice from the BIOS setup screen.
	m.Write(tlcs900h.Byte, 0x8101, 0x00) // Sprite palette 1
	m.Write(tlcs900h.Byte, 0x8102, 0x04)
	m.Write(tlcs900h.Byte, 0x8103, 0x07)
	m.Write(tlcs900h.Byte, 0x8105, 0x07) // Sprite palette 2
	m.Write(tlcs900h.Byte, 0x8106, 0x07)
	m.Write(tlcs900h.Byte, 0x8107, 0x00)
	m.Write(tlcs900h.Byte, 0x8109, 0x07) // Scroll 1 palette 1
	m.Write(tlcs900h.Byte, 0x810A, 0x07)
	m.Write(tlcs900h.Byte, 0x810B, 0x07)
	m.Write(tlcs900h.Byte, 0x810D, 0x00) // Scroll 1 palette 2
	m.Write(tlcs900h.Byte, 0x810E, 0x02)
	m.Write(tlcs900h.Byte, 0x810F, 0x06)
	m.Write(tlcs900h.Byte, 0x8111, 0x00) // Scroll 2 palette 1
	m.Write(tlcs900h.Byte, 0x8112, 0x00)
	m.Write(tlcs900h.Byte, 0x8113, 0x00)
	m.Write(tlcs900h.Byte, 0x8115, 0x07) // Scroll 2 palette 2
	m.Write(tlcs900h.Byte, 0x8116, 0x07)
	m.Write(tlcs900h.Byte, 0x8117, 0x07)

	// Sprite tables: $9000-$91FF and $9800-$99FF filled with $0020
	// (64 sprite entries per table, 4 bytes each, $0020 = default tile)
	for i := uint32(0); i < 0x200; i += 2 {
		m.Write(tlcs900h.Word, 0x9000+i, 0x0020)
		m.Write(tlcs900h.Word, 0x9800+i, 0x0020)
	}
}

// initSystemRAM writes post-boot system state to work RAM ($6C00-$6FFF)
// and copies cartridge header fields.
// Values from memdump.bin at cart handoff point.
func (m *Memory) initSystemRAM() {
	cart := m.cart
	// Cart header copy
	if len(cart) > 0x2F {
		// Entry point ($1C-$1F -> $6C00)
		for i := 0; i < 4; i++ {
			m.Write(tlcs900h.Byte, uint32(0x6C00+i), uint32(cart[0x1C+i]))
		}
		// Software ID ($20-$21 -> $6C04, $6E82, $6C69)
		m.Write(tlcs900h.Byte, 0x6C04, uint32(cart[0x20]))
		m.Write(tlcs900h.Byte, 0x6C05, uint32(cart[0x21]))
		m.Write(tlcs900h.Byte, 0x6E82, uint32(cart[0x20]))
		m.Write(tlcs900h.Byte, 0x6E83, uint32(cart[0x21]))
		m.Write(tlcs900h.Byte, 0x6C69, uint32(cart[0x20]))
		m.Write(tlcs900h.Byte, 0x6C6A, uint32(cart[0x21]))
		// Sub-code ($22 -> $6C06, $6E84)
		m.Write(tlcs900h.Byte, 0x6C06, uint32(cart[0x22]))
		m.Write(tlcs900h.Byte, 0x6E84, uint32(cart[0x22]))
		// System code ($23 -> $6F90, $6F91)
		sysCode := uint32(cart[0x23])
		m.Write(tlcs900h.Byte, 0x6F90, sysCode)
		m.Write(tlcs900h.Byte, 0x6F91, sysCode)
		// Title ($24-$2F -> $6C08, $6C6C)
		for i := 0; i < 12; i++ {
			m.Write(tlcs900h.Byte, uint32(0x6C08+i), uint32(cart[0x24+i]))
			m.Write(tlcs900h.Byte, uint32(0x6C6C+i), uint32(cart[0x24+i]))
		}
	}

	// Setup completion checksum. Default matches memdump ($DC).
	// Recomputed by updateSetupChecksum when language or palette changes.
	m.Write(tlcs900h.Byte, 0x6C14, 0xDC) // Checksum low
	m.Write(tlcs900h.Byte, 0x6C15, 0x00) // Checksum high
	m.Write(tlcs900h.Byte, 0x6C24, 0x0A) // INT0/AD priority config
	m.Write(tlcs900h.Byte, 0x6C25, 0xDC) // Setup checksum input

	// A/D converter state
	m.Write(tlcs900h.Word, 0x6C18, 0x01BC) // A/D conversion counter
	m.Write(tlcs900h.Word, 0x6C1A, 0x0258) // A/D conversion interval

	// Boot state flags
	m.Write(tlcs900h.Byte, 0x6C21, 0x02) // Power button state
	m.Write(tlcs900h.Byte, 0x6C46, 0x01) // Cart present
	m.Write(tlcs900h.Byte, 0x6C55, 0x01) // Commercial game loaded
	cs0Type := uint32(flashType(len(cart)))
	m.Write(tlcs900h.Byte, 0x6C58, cs0Type)
	m.Write(tlcs900h.Byte, 0x6F92, cs0Type)

	// CS1 present if cart > 2 MB
	if len(cart) > 0x200000 {
		cs1Type := uint32(flashType(len(cart) - 0x200000))
		m.Write(tlcs900h.Byte, 0x6C59, cs1Type)
		m.Write(tlcs900h.Byte, 0x6C7E, 0x01) // CS1 present flag
		m.Write(tlcs900h.Byte, 0x6F93, cs1Type)
	}

	// Boot markers
	m.Write(tlcs900h.Word, 0x6C7A, 0xA5A5)
	m.Write(tlcs900h.Word, 0x6C7C, 0x5AA5)

	// BIOS state
	m.Write(tlcs900h.Byte, 0x6E85, 0x01)
	m.Write(tlcs900h.Byte, 0x6E86, 0x22)
	m.Write(tlcs900h.Byte, 0x6E87, 0x01)
	m.Write(tlcs900h.Byte, 0x6E88, 0x01)

	// Monochrome color scheme configuration
	schemeConfig := []byte{
		0x07, 0x22, 0x22, 0x22, 0x22, 0x2E, 0x2E, 0x2A,
		0x2E, 0x2E, 0x2E, 0x2E, 0x2A, 0x2E, 0x2E, 0x2E,
		0x2E, 0x2A, 0x2E, 0x2E, 0x2E, 0x2E, 0x2A, 0x2E,
		0x2E, 0x2E, 0x2E, 0x2A, 0x22, 0x01, 0x03, 0x01,
	}
	for i, b := range schemeConfig {
		m.Write(tlcs900h.Byte, uint32(0x6D80+i), uint32(b))
	}
	m.Write(tlcs900h.Byte, 0x6DA4, 0xFE)

	// Color scheme indices and palette data
	colorIndices := []byte{
		0xFF, 0xFF, 0x07, 0x01, 0x06, 0x05, 0x02, 0x03,
		0x06, 0x00, 0x02, 0x06, 0x01, 0x02, 0x03, 0xFF,
		0xFF, 0xFF,
	}
	for i, b := range colorIndices {
		m.Write(tlcs900h.Byte, uint32(0x6DC6+i), uint32(b))
	}

	// Standard grayscale ramp palette (8 copies)
	palBlock := []byte{
		0xFF, 0x0F, 0xDD, 0x0D, 0xBB, 0x0B, 0x99, 0x09,
		0x77, 0x07, 0x44, 0x04, 0x33, 0x03, 0x00, 0x00,
	}
	for i := 0; i < 8; i++ {
		base := uint32(0x6DD8) + uint32(i)*16
		for j, b := range palBlock {
			m.Write(tlcs900h.Byte, base+uint32(j), uint32(b))
		}
	}

	// Z80 program area residual
	m.Write(tlcs900h.Byte, 0x6F00, 0xCF)
	m.Write(tlcs900h.Byte, 0x6F01, 0x07)
	m.Write(tlcs900h.Byte, 0x6F02, 0x01)
	m.Write(tlcs900h.Byte, 0x6F03, 0x01)

	// System state
	m.Write(tlcs900h.Word, 0x6F80, 0x03FF) // Battery voltage (full)
	m.Write(tlcs900h.Byte, 0x6F83, 0x40)   // Bit 6 set at cart handoff
	m.Write(tlcs900h.Byte, 0x6F84, 0x40)   // User Boot = Power ON
	m.Write(tlcs900h.Byte, 0x6F87, 0x01)   // Language = English
}

// flashType returns the flash chip type code for the given ROM size.
// 0=none, 1=4Mbit (512KB), 2=8Mbit (1MB), 3=16Mbit (2MB).
func flashType(size int) uint8 {
	switch {
	case size <= 0:
		return 0
	case size <= 0x80000: // 512 KB
		return 1
	case size <= 0x100000: // 1 MB
		return 2
	default: // 2 MB
		return 3
	}
}

// initK1GEPalettes initializes the K1GE compatibility palette areas
// ($8380-$83DF) with the grayscale ramp for monochrome (NGP) games.
// Color games (system code >= $10) do not use these palettes.
// Each area (sprite, plane 1, plane 2) gets two 8-shade palette
// groups matching the grayscale ramp stored in system RAM at $6DD8.
func (m *Memory) initK1GEPalettes() {
	cart := m.cart
	if len(cart) <= 0x23 || cart[0x23] >= 0x10 {
		return
	}

	// 16-bit LE grayscale palette entries (8 shades, brightest to darkest)
	shades := [8]uint32{
		0x0FFF, 0x0DDD, 0x0BBB, 0x0999,
		0x0777, 0x0444, 0x0333, 0x0000,
	}
	m.WriteK1GEPalettes(shades, 0x00)
}

// setGEMode configures K2GE mono/color mode from system code at $6F91.
func (m *Memory) setGEMode() {
	code := uint8(m.Read(tlcs900h.Byte, 0x6F91))

	m.Write(tlcs900h.Byte, 0x87F0, 0xAA)
	if code < 0x10 {
		m.Write(tlcs900h.Byte, 0x87E2, 0x80)
		m.Write(tlcs900h.Byte, 0x6F95, 0x00)
	} else {
		m.Write(tlcs900h.Byte, 0x87E2, 0x00)
		m.Write(tlcs900h.Byte, 0x6F95, 0x10)
	}
	m.Write(tlcs900h.Byte, 0x87F0, 0x55)
}
