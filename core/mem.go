package core

import (
	"errors"
	"fmt"
	"hash/crc32"

	z80 "github.com/user-none/go-chip-z80"

	"github.com/user-none/ecolr/core/tlcs900h"
)

const (
	biosROMSize  = 0x10000  // 64 KB
	workRAMSize  = 0x3000   // 12 KB ($4000-$6FFF)
	z80RAMSize   = 0x1000   // 4 KB ($7000-$7FFF)
	k2geSize     = 0x4000   // 16 KB ($8000-$BFFF)
	maxCartSize  = 0x400000 // 4 MB
	sfrSize      = 0x80     // $00-$7F
	customIOSize = 0x80     // $80-$FF

	// Memory map base addresses (TLCS-900/H 24-bit address space)
	workRAMStart = 0x004000
	workRAMEnd   = workRAMStart + workRAMSize - 1
	z80RAMStart  = 0x007000
	z80RAMEnd    = z80RAMStart + z80RAMSize - 1
	k2geStart    = 0x008000
	k2geEnd      = k2geStart + k2geSize - 1
	cartCS0Start = 0x200000
	cartCS0End   = 0x3FFFFF
	cartCS0Size  = 0x200000
	cartCS1Start = 0x800000
	cartCS1End   = 0x9FFFFF
	biosROMStart = 0xFF0000
	biosROMEnd   = biosROMStart + biosROMSize - 1
)

// Memory implements tlcs900h.Bus for the Neo Geo Pocket Color memory map.
// Peripherals are ticked at instruction boundaries via the Tick method.
//
// Address map (TLCS-900/H view, 24-bit):
//
//	$000000-$0000FF  SFR + custom I/O registers (byte-level dispatch)
//	$004000-$006FFF  Work RAM (12 KB)
//	$007000-$007FFF  Z80 shared RAM (4 KB)
//	$008000-$00BFFF  K2GE video (16 KB buffer)
//	$200000-$3FFFFF  Cartridge CS0
//	$800000-$9FFFFF  Cartridge CS1 (maps to cart offset $200000+)
//	$FF0000-$FFFFFF  BIOS ROM (64 KB)
type Memory struct {
	cpu       *tlcs900h.CPU
	lastCycle uint64
	bios      []byte
	cart      []byte
	origCart  []byte
	cs0       *Flash
	cs1       *Flash
	workRAM   [workRAMSize]byte
	z80RAM    [z80RAMSize]byte
	k2ge      [k2geSize]byte
	sfr       [sfrSize]byte
	customIO  [customIOSize]byte
	intc      IntC
	adc       *ADC
	timers    *Timers
	rtc       *RTC
	inputB0   uint8

	dacL uint8 // $A2: left DAC register
	dacR uint8 // $A3: right DAC register

	psg         *T6W28
	soundChipEn uint8 // $B8: $55=on, $AA=off
	z80Active   uint8 // $B9: $55=on, $AA=off

	z80bus *Z80Bus
	z80cpu *z80.CPU

	clockGear uint8 // clock gear value (0-4), written by BIOS to $80

	k2geModeLocked bool // K2GE mode register ($87E2) write protection
}

// NewMemory creates a Memory with the given cartridge ROM.
// Cart may be nil for BIOS-only inspection.
// BIOS is provided after construction via SetBIOS.
func NewMemory(cart []byte, psg *T6W28) (*Memory, error) {
	if cart != nil && len(cart) > maxCartSize {
		return nil, fmt.Errorf("cart must be at most %d bytes, got %d", maxCartSize, len(cart))
	}
	if cart != nil && len(cart) == 0 {
		return nil, errors.New("cart must be nil or non-empty")
	}

	m := &Memory{
		psg:            psg,
		soundChipEn:    0xAA,
		z80Active:      0xAA,
		dacL:           128,
		dacR:           128,
		k2geModeLocked: true,
	}
	m.adc = NewADC(&m.intc)
	m.timers = NewTimers(&m.intc)
	m.rtc = NewRTC(&m.intc, NGPCTiming.CPUClockHz)
	m.z80bus = &Z80Bus{
		ram:  &m.z80RAM,
		psg:  psg,
		comm: &m.customIO,
		intc: &m.intc,
	}
	m.z80cpu = z80.New(m.z80bus)
	m.z80cpu.SetState(z80.Registers{})
	m.resetK2GERegisters()

	if cart != nil {
		m.cart = make([]byte, len(cart))
		copy(m.cart, cart)
		m.origCart = make([]byte, len(cart))
		copy(m.origCart, cart)
		m.initFlash()
	}

	return m, nil
}

// initFlash creates flash chip(s) from the cartridge ROM data.
// CS0 covers the first 2 MB, CS1 covers the second 2 MB (4 MB ROMs only).
func (m *Memory) initFlash() {
	if m.cart == nil {
		return
	}
	cs0Size := len(m.cart)
	if cs0Size > cartCS0Size {
		cs0Size = cartCS0Size
	}
	m.cs0 = NewFlash(m.cart[:cs0Size])

	if len(m.cart) > cartCS0Size {
		m.cs1 = NewFlash(m.cart[cartCS0Size:])
	}
}

// Reset zeroes all RAM and I/O registers. ROM data is preserved.
func (m *Memory) Reset() {
	m.workRAM = [workRAMSize]byte{}
	m.z80RAM = [z80RAMSize]byte{}
	m.resetK2GERegisters()
	m.sfr = [sfrSize]byte{}
	m.customIO = [customIOSize]byte{}
	m.intc.Reset()
	m.adc.Reset()
	m.timers.Reset()
	m.rtc.Reset()
	m.k2geModeLocked = true
	m.lastCycle = 0
	m.dacL = 128
	m.dacR = 128
	m.inputB0 = 0x00
	m.soundChipEn = 0xAA
	m.z80Active = 0xAA
	m.z80cpu.Reset()
	m.z80cpu.SetState(z80.Registers{})
	if m.cs0 != nil {
		m.cs0.Reset()
	}
	if m.cs1 != nil {
		m.cs1.Reset()
	}
}

// resetK2GERegisters zeroes the K2GE register space and sets hardware
// power-on defaults.
func (m *Memory) resetK2GERegisters() {
	m.k2ge = [k2geSize]byte{}
	m.k2ge[0x0004] = 0xFF // WSI.H: window horizontal size
	m.k2ge[0x0005] = 0xFF // WSI.V: window vertical size
	m.k2ge[0x0006] = 0xC6 // REF: frame rate register
	m.k2ge[0x0402] = 0x80 // LED_FLC: LED flash control
}

// Read8 returns a byte from the specified address.
func (m *Memory) Read8(addr uint32) uint8 {
	switch {
	case addr <= 0x0000FF:
		return m.readIOByte(addr)
	case addr >= workRAMStart && addr <= workRAMEnd:
		return m.workRAM[addr-workRAMStart]
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		return m.z80RAM[addr-z80RAMStart]
	case addr >= k2geStart && addr <= k2geEnd:
		return m.readK2GEByte(addr - k2geStart)
	case addr >= cartCS0Start && addr <= cartCS0End:
		return m.readFlashByte(m.cs0, addr-cartCS0Start)
	case addr >= cartCS1Start && addr <= cartCS1End:
		return m.readFlashByte(m.cs1, addr-cartCS1Start)
	case addr >= biosROMStart && addr <= biosROMEnd:
		return m.bios[addr-biosROMStart]
	}
	return 0
}

// Read16 returns a 16-bit little-endian value from the specified address.
func (m *Memory) Read16(addr uint32) uint16 {
	switch {
	case addr <= 0x0000FF:
		return uint16(m.readIOByte(addr)) | uint16(m.readIOByte(addr+1))<<8
	case addr >= workRAMStart && addr <= workRAMEnd:
		off := addr - workRAMStart
		return uint16(m.workRAM[off]) | uint16(m.workRAM[off+1])<<8
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		off := addr - z80RAMStart
		return uint16(m.z80RAM[off]) | uint16(m.z80RAM[off+1])<<8
	case addr >= k2geStart && addr <= k2geEnd:
		off := addr - k2geStart
		return uint16(m.readK2GEByte(off)) | uint16(m.readK2GEByte(off+1))<<8
	case addr >= cartCS0Start && addr <= cartCS0End:
		off := addr - cartCS0Start
		return uint16(m.readFlashByte(m.cs0, off)) | uint16(m.readFlashByte(m.cs0, off+1))<<8
	case addr >= cartCS1Start && addr <= cartCS1End:
		off := addr - cartCS1Start
		return uint16(m.readFlashByte(m.cs1, off)) | uint16(m.readFlashByte(m.cs1, off+1))<<8
	case addr >= biosROMStart && addr <= biosROMEnd:
		off := addr - biosROMStart
		return uint16(m.bios[off]) | uint16(m.bios[off+1])<<8
	}
	return 0
}

// Read32 returns a 32-bit little-endian value from the specified address.
func (m *Memory) Read32(addr uint32) uint32 {
	switch {
	case addr <= 0x0000FF:
		return uint32(m.readIOByte(addr)) | uint32(m.readIOByte(addr+1))<<8 |
			uint32(m.readIOByte(addr+2))<<16 | uint32(m.readIOByte(addr+3))<<24
	case addr >= workRAMStart && addr <= workRAMEnd:
		off := addr - workRAMStart
		return uint32(m.workRAM[off]) | uint32(m.workRAM[off+1])<<8 |
			uint32(m.workRAM[off+2])<<16 | uint32(m.workRAM[off+3])<<24
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		off := addr - z80RAMStart
		return uint32(m.z80RAM[off]) | uint32(m.z80RAM[off+1])<<8 |
			uint32(m.z80RAM[off+2])<<16 | uint32(m.z80RAM[off+3])<<24
	case addr >= k2geStart && addr <= k2geEnd:
		off := addr - k2geStart
		return uint32(m.readK2GEByte(off)) | uint32(m.readK2GEByte(off+1))<<8 |
			uint32(m.readK2GEByte(off+2))<<16 | uint32(m.readK2GEByte(off+3))<<24
	case addr >= cartCS0Start && addr <= cartCS0End:
		off := addr - cartCS0Start
		return uint32(m.readFlashByte(m.cs0, off)) | uint32(m.readFlashByte(m.cs0, off+1))<<8 |
			uint32(m.readFlashByte(m.cs0, off+2))<<16 | uint32(m.readFlashByte(m.cs0, off+3))<<24
	case addr >= cartCS1Start && addr <= cartCS1End:
		off := addr - cartCS1Start
		return uint32(m.readFlashByte(m.cs1, off)) | uint32(m.readFlashByte(m.cs1, off+1))<<8 |
			uint32(m.readFlashByte(m.cs1, off+2))<<16 | uint32(m.readFlashByte(m.cs1, off+3))<<24
	case addr >= biosROMStart && addr <= biosROMEnd:
		off := addr - biosROMStart
		return uint32(m.bios[off]) | uint32(m.bios[off+1])<<8 |
			uint32(m.bios[off+2])<<16 | uint32(m.bios[off+3])<<24
	}
	return 0
}

// Write8 stores a byte at the specified address.
func (m *Memory) Write8(addr uint32, val uint8) {
	switch {
	case addr <= 0x0000FF:
		m.writeIOByte(addr, val)
	case addr >= workRAMStart && addr <= workRAMEnd:
		m.workRAM[addr-workRAMStart] = val
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		m.z80RAM[addr-z80RAMStart] = val
	case addr >= k2geStart && addr <= k2geEnd:
		m.writeK2GEByte(addr-k2geStart, val)
	case addr >= cartCS0Start && addr <= cartCS0End:
		m.writeFlashByte(m.cs0, addr-cartCS0Start, val)
	case addr >= cartCS1Start && addr <= cartCS1End:
		m.writeFlashByte(m.cs1, addr-cartCS1Start, val)
	}
}

// Write16 stores a 16-bit little-endian value at the specified address.
func (m *Memory) Write16(addr uint32, val uint16) {
	switch {
	case addr <= 0x0000FF:
		m.writeIOByte(addr, uint8(val))
		m.writeIOByte(addr+1, uint8(val>>8))
	case addr >= workRAMStart && addr <= workRAMEnd:
		off := addr - workRAMStart
		m.workRAM[off] = uint8(val)
		m.workRAM[off+1] = uint8(val >> 8)
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		off := addr - z80RAMStart
		m.z80RAM[off] = uint8(val)
		m.z80RAM[off+1] = uint8(val >> 8)
	case addr >= k2geStart && addr <= k2geEnd:
		m.writeK2GE16(addr-k2geStart, val)
	case addr >= cartCS0Start && addr <= cartCS0End:
		off := addr - cartCS0Start
		m.writeFlashByte(m.cs0, off, uint8(val))
		m.writeFlashByte(m.cs0, off+1, uint8(val>>8))
	case addr >= cartCS1Start && addr <= cartCS1End:
		off := addr - cartCS1Start
		m.writeFlashByte(m.cs1, off, uint8(val))
		m.writeFlashByte(m.cs1, off+1, uint8(val>>8))
	}
}

// Write32 stores a 32-bit little-endian value at the specified address.
func (m *Memory) Write32(addr uint32, val uint32) {
	switch {
	case addr <= 0x0000FF:
		m.writeIOByte(addr, uint8(val))
		m.writeIOByte(addr+1, uint8(val>>8))
		m.writeIOByte(addr+2, uint8(val>>16))
		m.writeIOByte(addr+3, uint8(val>>24))
	case addr >= workRAMStart && addr <= workRAMEnd:
		off := addr - workRAMStart
		m.workRAM[off] = uint8(val)
		m.workRAM[off+1] = uint8(val >> 8)
		m.workRAM[off+2] = uint8(val >> 16)
		m.workRAM[off+3] = uint8(val >> 24)
	case addr >= z80RAMStart && addr <= z80RAMEnd:
		off := addr - z80RAMStart
		m.z80RAM[off] = uint8(val)
		m.z80RAM[off+1] = uint8(val >> 8)
		m.z80RAM[off+2] = uint8(val >> 16)
		m.z80RAM[off+3] = uint8(val >> 24)
	case addr >= k2geStart && addr <= k2geEnd:
		off := addr - k2geStart
		m.writeK2GEByte(off, uint8(val))
		m.writeK2GEByte(off+1, uint8(val>>8))
		m.writeK2GEByte(off+2, uint8(val>>16))
		m.writeK2GEByte(off+3, uint8(val>>24))
	case addr >= cartCS0Start && addr <= cartCS0End:
		off := addr - cartCS0Start
		m.writeFlashByte(m.cs0, off, uint8(val))
		m.writeFlashByte(m.cs0, off+1, uint8(val>>8))
		m.writeFlashByte(m.cs0, off+2, uint8(val>>16))
		m.writeFlashByte(m.cs0, off+3, uint8(val>>24))
	case addr >= cartCS1Start && addr <= cartCS1End:
		off := addr - cartCS1Start
		m.writeFlashByte(m.cs1, off, uint8(val))
		m.writeFlashByte(m.cs1, off+1, uint8(val>>8))
		m.writeFlashByte(m.cs1, off+2, uint8(val>>16))
		m.writeFlashByte(m.cs1, off+3, uint8(val>>24))
	}
}

// readIOByte reads a single byte from an I/O register.
func (m *Memory) readIOByte(addr uint32) uint8 {
	if addr > 0xFF {
		return 0
	}
	switch {
	case addr == 0x20, addr >= 0x22 && addr <= 0x29,
		addr >= 0x30 && addr <= 0x3A,
		addr >= 0x40 && addr <= 0x49:
		return m.timers.ReadReg(addr)
	case addr >= 0x60 && addr <= 0x67:
		return m.adc.ReadADREG(int(addr - 0x60))
	case addr == 0x6D:
		return m.adc.ReadADMOD()
	case addr >= 0x70 && addr <= 0x7A:
		return m.intc.ReadReg(int(addr - 0x70))
	case addr >= 0x90 && addr <= 0x97:
		return m.rtc.ReadReg(addr)
	case addr == 0xA2:
		return m.dacL
	case addr == 0xA3:
		return m.dacR
	case addr == 0xB0:
		return m.inputB0
	case addr == 0xB1:
		return 0x02 // Power status (sub-battery OK)
	case addr == 0xB8:
		return m.soundChipEn
	case addr == 0xB9:
		return m.z80Active
	}
	if addr < sfrSize {
		return m.sfr[addr]
	}
	return m.customIO[addr-sfrSize]
}

// writeIOByte writes a single byte to an I/O register.
func (m *Memory) writeIOByte(addr uint32, val uint8) {
	if addr > 0xFF {
		return
	}
	switch {
	case addr == 0x20, addr >= 0x22 && addr <= 0x29,
		addr >= 0x30 && addr <= 0x3A,
		addr >= 0x40 && addr <= 0x49:
		m.timers.WriteReg(addr, val)
		return
	case addr >= 0x60 && addr <= 0x67:
		return // A/D result registers are read-only
	case addr == 0x6D:
		m.adc.WriteADMOD(val)
		return
	case addr >= 0x70 && addr <= 0x7A:
		m.intc.WriteReg(int(addr-0x70), val)
		return
	case addr >= 0x90 && addr <= 0x97:
		m.rtc.WriteReg(addr, val)
		return
	case addr == 0xA0:
		if m.psg != nil && m.soundChipEn == 0x55 && m.z80Active == 0xAA {
			m.psg.WriteRight(val)
		}
		m.customIO[addr-sfrSize] = val
		return
	case addr == 0xA1:
		if m.psg != nil && m.soundChipEn == 0x55 && m.z80Active == 0xAA {
			m.psg.WriteLeft(val)
		}
		m.customIO[addr-sfrSize] = val
		return
	case addr == 0xA2:
		m.dacL = val
		return
	case addr == 0xA3:
		m.dacR = val
		return
	case addr == 0xB0:
		return // Input port read-only
	case addr == 0xB1:
		return // Power status read-only
	case addr == 0xB8:
		m.soundChipEn = val
		return
	case addr == 0xB9:
		m.z80Active = val
		if val == 0x55 {
			m.z80cpu.Reset()
			m.z80cpu.SetState(z80.Registers{})
			m.timers.ClearZ80IRQPending()
		}
		return
	case addr == 0xBA:
		if m.z80Active == 0x55 {
			m.z80cpu.NMI()
		}
		return
	case addr == 0x80:
		if val > 4 {
			val = 4
		}
		m.clockGear = val
		m.customIO[addr-sfrSize] = val
		return
	}
	if addr < sfrSize {
		m.sfr[addr] = val
	} else {
		m.customIO[addr-sfrSize] = val
	}
}

// Z80CPU returns the Z80 sound CPU.
func (m *Memory) Z80CPU() *z80.CPU {
	return m.z80cpu
}

// Z80Active reports whether the Z80 is enabled ($B9 == $55).
func (m *Memory) Z80Active() bool {
	return m.z80Active == 0x55
}

// ClockGearDivisor returns the current clock gear divisor (1, 2, 4, 8, or 16).
func (m *Memory) ClockGearDivisor() int {
	return 1 << m.clockGear
}

// Z80IRQPending returns and clears the number of pending Z80 IRQs
// accumulated from Timer 3 (TO3) rising edges.
func (m *Memory) Z80IRQPending() int {
	return m.timers.Z80IRQPending()
}

// ReadZ80RAM reads a byte from Z80 shared RAM for diagnostics.
func (m *Memory) ReadZ80RAM(addr uint16) uint8 {
	if addr < z80RAMSize {
		return m.z80RAM[addr]
	}
	return 0
}

// DACValues returns the current left and right DAC register values.
func (m *Memory) DACValues() (uint8, uint8) {
	return m.dacL, m.dacR
}

// SetInput updates the input register ($B0) value.
func (m *Memory) SetInput(val uint8) {
	m.inputB0 = val
}

// SetCPU stores the CPU reference needed by Tick.
// Call after tlcs900h.New(mem) returns, since the CPU cannot be passed at
// Memory construction time.
func (m *Memory) SetCPU(c *tlcs900h.CPU) {
	m.cpu = c
}

// SetBIOS injects a BIOS ROM into Memory. Used by the HLE layer to provide
// a synthetic BIOS after Memory construction.
func (m *Memory) SetBIOS(data []byte) {
	m.bios = make([]byte, biosROMSize)
	copy(m.bios, data)
}

// GetROMCRC32 returns the CRC32-IEEE checksum of the cartridge ROM data.
// Returns 0 if no cartridge is loaded.
func (m *Memory) GetROMCRC32() uint32 {
	if m.cart == nil {
		return 0
	}
	return crc32.ChecksumIEEE(m.cart)
}

// WriteK1GEPalettes writes 8 shade colors to the K1GE compatibility palette
// areas (sprite $8380, plane 1 $83A0, plane 2 $83C0), sets the background
// and window colors to white, and stores the palette index at $6F94.
func (m *Memory) WriteK1GEPalettes(shades [8]uint32, index uint32) {
	// Fill sprite, plane 1, and plane 2 K1GE compat palette areas.
	// Each area has 2 palette groups of 8 shades (16 bytes per group).
	// Palette RAM requires word-size writes.
	bases := [3]uint32{0x8380, 0x83A0, 0x83C0}
	for _, base := range bases {
		for group := uint32(0); group < 2; group++ {
			for i, val := range shades {
				m.Write16(base+group*16+uint32(i)*2, uint16(val))
			}
		}
	}

	// Background and window colors default to white (shade 0).
	m.Write16(0x83E0, 0x0FFF)
	m.Write16(0x83F0, 0x0FFF)

	// Update the BIOS palette index at $6F94.
	m.Write8(0x6F94, uint8(index))
}

// Tick catches up peripheral state to the current CPU cycle count.
// Called after StepCycles to handle any remaining cycles (including
// halt periods where no bus access occurs).
//
// The delta is computed from cpu.Cycles() rather than the StepCycles
// return value because Step executes complete instructions atomically.
// When an instruction overshoots the scanline budget, StepCycles caps
// its return value and records a deficit, but all cycles already ran.
// Reading cpu.Cycles() ensures peripherals advance by the true cost
// immediately, and the subsequent deficit-draining StepCycles call
// produces a zero delta here since no new cycles occurred.
func (m *Memory) Tick() {
	if m.cpu == nil {
		return
	}
	cycle := m.cpu.Cycles()
	delta := int(cycle - m.lastCycle)
	if delta > 0 {
		m.adc.Tick(delta)
		// Prescaler runs from fc (oscillator/4), independent of clock gear.
		// Convert geared CPU cycles back to ungeared fc-scale cycles.
		fcDelta := delta * (1 << m.clockGear)
		m.timers.Tick(fcDelta)
		// RTC uses sub-oscillator on real hardware, also independent of clock gear.
		m.rtc.Tick(fcDelta)
		m.lastCycle = cycle
	}
	m.intc.CheckInterrupts(m.cpu)
}

// RequestINT0 sets the INT0 external interrupt pending flag.
// The interrupt will fire on the next Tick if its priority is
// configured and exceeds the CPU's IFF mask.
func (m *Memory) RequestINT0() {
	m.intc.SetPending(0, false) // INT0 is reg 0 (INTE0AD), low source
}

// RequestINT4 sets the INT4 (VBlank) interrupt pending flag.
// The interrupt will fire on the next Tick if its priority is
// configured and exceeds the CPU's IFF mask.
func (m *Memory) RequestINT4() {
	m.intc.SetPending(1, false) // INT4 is reg 1 (INTE45), low source
}

// HBlank delivers one TI0 tick to timers when the K2GE HBlank
// enable bit ($8000 bit 6) is set.
func (m *Memory) HBlank() {
	if m.k2ge[0x0000]&0x40 == 0 {
		return
	}
	m.timers.TickTI0(1)
	m.intc.CheckInterrupts(m.cpu)
}

// CheckInterrupts evaluates pending interrupts and delivers any that
// meet the priority threshold. Use this instead of Tick when no CPU
// cycles have elapsed and only interrupt dispatch is needed.
func (m *Memory) CheckInterrupts() {
	m.intc.CheckInterrupts(m.cpu)
}

// VBlankEnabled reports whether the K2GE VBlank interrupt enable
// bit ($8000 bit 7) is set.
func (m *Memory) VBlankEnabled() bool {
	return m.k2ge[0x0000]&0x80 != 0
}

// SetVBlankStatus sets or clears the K2GE VBlank status flag
// at $8010 bit 6. The BIOS frame delay routine at $FF112F polls
// this bit to detect VBlank transitions.
func (m *Memory) SetVBlankStatus(active bool) {
	if active {
		m.k2ge[0x0010] |= 0x40
	} else {
		m.k2ge[0x0010] &^= 0xC0
	}
}

// readK2GEByte reads a single byte from a K2GE register with per-register
// masking for unmapped or fixed bits.
func (m *Memory) readK2GEByte(offset uint32) uint8 {
	val := m.k2ge[offset]
	switch {
	case offset == 0x0000:
		return val & 0xC0
	case offset == 0x0010:
		return val & 0xC0
	case offset == 0x0012:
		return val & 0xCF
	case offset == 0x0030:
		return val & 0x80
	case offset >= 0x0100 && offset <= 0x0117:
		return val & 0x07
	case offset == 0x0118:
		return val & 0xC7
	case offset >= 0x0200 && offset <= 0x03FF:
		return val
	case offset == 0x0400:
		return val | 0x07
	case offset == 0x07E2:
		return val & 0x80
	case offset >= 0x0C00 && offset <= 0x0C3F:
		return val & 0x0F
	default:
		return val
	}
}

// writeK2GE16 handles word writes to the K2GE register space.
// Palette RAM ($8200-$83FF) requires word-size writes.
func (m *Memory) writeK2GE16(offset uint32, val uint16) {
	if offset >= 0x0200 && offset <= 0x03FE {
		m.k2ge[offset] = uint8(val)
		m.k2ge[offset+1] = uint8(val >> 8)
		return
	}
	m.writeK2GEByte(offset, uint8(val))
	m.writeK2GEByte(offset+1, uint8(val>>8))
}

// writeK2GEByte writes a single byte to a K2GE register with access control.
func (m *Memory) writeK2GEByte(offset uint32, val uint8) {
	switch {
	case offset == 0x0000:
		m.k2ge[offset] = val & 0xC0
	case offset == 0x0006 || offset == 0x0008 ||
		offset == 0x0009 || offset == 0x0010:
		// Read-only registers: silently drop
		return
	case offset == 0x0012:
		m.k2ge[offset] = val & 0xCF
	case offset == 0x0030:
		m.k2ge[offset] = val & 0x80
	case offset == 0x0118:
		m.k2ge[offset] = val & 0xC7
	case offset >= 0x0200 && offset <= 0x03FF:
		m.k2ge[offset] = val
	case offset >= 0x0C00 && offset <= 0x0C3F:
		m.k2ge[offset] = val & 0x0F
	case offset == 0x07E2:
		if m.k2geModeLocked {
			return
		}
		m.k2ge[offset] = val & 0x80
	case offset == 0x07E0:
		if val == 0x52 {
			m.resetK2GERegisters()
		}
		return
	case offset == 0x07F0:
		m.k2geModeLocked = val != 0xAA
		return
	default:
		m.k2ge[offset] = val
	}
}

// SetRasterPosition updates the K2GE raster position registers.
// These are written directly to the array, bypassing the write filter.
func (m *Memory) SetRasterPosition(scanline, clocksRemaining int) {
	m.k2ge[0x0009] = uint8(scanline)
	m.k2ge[0x0008] = uint8(clocksRemaining >> 2)
}

// VRAM returns a pointer to the K2GE VRAM array.
func (m *Memory) VRAM() *[k2geSize]byte {
	return &m.k2ge
}

// readFlashByte reads a single byte from a flash chip.
func (m *Memory) readFlashByte(chip *Flash, offset uint32) uint8 {
	if chip == nil {
		return 0
	}
	return chip.Read(offset)
}

// writeFlashByte writes a single byte to a flash chip.
func (m *Memory) writeFlashByte(chip *Flash, offset uint32, val uint8) {
	if chip == nil {
		return
	}
	chip.Write(offset, val)
}
