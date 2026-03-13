package core

import "github.com/user-none/ecolr/core/tlcs900h"

// intcRegCount is the number of interrupt enable registers ($70-$7A).
const intcRegCount = 11

// irqSource maps an interrupt enable register field to a vector offset.
type irqSource struct {
	reg    int   // index into regs[] (0=INTE0AD, 1=INTE45, ...)
	mask   uint8 // pending bit mask: 0x08 for low, 0x80 for high
	vector uint8 // vector offset from $FFFF00 (e.g. $28 for INT0)
}

// irqSources lists all maskable interrupt sources in the TMP95C061,
// ordered from lowest to highest hardware priority. When multiple
// sources share the same software priority level, later entries
// (higher hardware priority) win.
var irqSources = []irqSource{
	{0, 0x08, 0x28}, // INT0
	{1, 0x08, 0x2C}, // INT4
	{1, 0x80, 0x30}, // INT5
	{2, 0x08, 0x34}, // INT6
	{2, 0x80, 0x38}, // INT7
	// 0x3C reserved
	{3, 0x08, 0x40},  // INTT0
	{3, 0x80, 0x44},  // INTT1
	{4, 0x08, 0x48},  // INTT2
	{4, 0x80, 0x4C},  // INTT3
	{5, 0x08, 0x50},  // INTTR4
	{5, 0x80, 0x54},  // INTTR5
	{6, 0x08, 0x58},  // INTTR6
	{6, 0x80, 0x5C},  // INTTR7
	{7, 0x08, 0x60},  // INTRX0
	{7, 0x80, 0x64},  // INTTX0
	{8, 0x08, 0x68},  // INTRX1
	{8, 0x80, 0x6C},  // INTTX1
	{0, 0x80, 0x70},  // INTAD
	{9, 0x08, 0x74},  // INTTC0
	{9, 0x80, 0x78},  // INTTC1
	{10, 0x08, 0x7C}, // INTTC2
	{10, 0x80, 0x80}, // INTTC3
}

// IntC is the TMP95C061 interrupt controller. It owns the interrupt
// enable registers ($70-$7A) and routes pending interrupts to the CPU.
//
// Each register holds two 4-bit fields:
//
//	Bit 7:   high source pending flag
//	Bits 6-4: high source priority (0=disabled, 1-6=level)
//	Bit 3:   low source pending flag
//	Bits 2-0: low source priority (0=disabled, 1-6=level)
//
// Peripherals set pending flags via SetPending. CheckInterrupts scans
// all sources and fires the highest priority pending interrupt on the CPU.
type IntC struct {
	regs       [intcRegCount]uint8
	hasPending bool
}

// ReadReg returns the value of interrupt enable register at index
// (0=INTE0AD through 10=INTETC23).
func (ic *IntC) ReadReg(idx int) uint8 {
	if idx < 0 || idx >= intcRegCount {
		return 0
	}
	return ic.regs[idx]
}

// WriteReg sets the interrupt enable register at index. Pending flag
// bits (3 and 7) can only be cleared by writing 0, not set by writing 1.
func (ic *IntC) WriteReg(idx int, val uint8) {
	if idx < 0 || idx >= intcRegCount {
		return
	}
	old := ic.regs[idx]
	// Pending bits can only be cleared, not set, by software writes.
	// Priority bits are freely writable.
	pending := old & 0x88
	cleared := ^val & 0x88
	pending &^= cleared
	ic.regs[idx] = pending | (val & 0x77)

	if cleared != 0 {
		ic.hasPending = ic.anyPending()
	}
}

// SetPending sets the pending flag for a source. reg is the register
// index (0-10), high selects the upper (true) or lower (false) source.
func (ic *IntC) SetPending(reg int, high bool) {
	if reg < 0 || reg >= intcRegCount {
		return
	}
	if high {
		ic.regs[reg] |= 0x80
	} else {
		ic.regs[reg] |= 0x08
	}
	ic.hasPending = true
}

// CheckInterrupts scans pending interrupts and fires the highest
// priority one on the CPU. Returns true if an interrupt was requested.
func (ic *IntC) CheckInterrupts(c *tlcs900h.CPU) bool {
	if !ic.hasPending {
		return false
	}

	bestPri := uint8(0)
	bestIdx := -1

	for i, src := range irqSources {
		r := ic.regs[src.reg]
		if r&src.mask == 0 {
			continue
		}
		var pri uint8
		if src.mask == 0x80 {
			pri = (r >> 4) & 0x07
		} else {
			pri = r & 0x07
		}
		if pri == 0 {
			continue
		}
		// Higher or equal priority wins; later entries (higher hw
		// priority) replace earlier ones at the same level.
		if pri >= bestPri {
			bestPri = pri
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		ic.hasPending = false
		return false
	}

	src := irqSources[bestIdx]
	vecIdx := src.vector / 4

	c.RequestInterrupt(bestPri, vecIdx)

	// Clear the pending flag now that we've committed to delivering it.
	ic.regs[src.reg] &^= src.mask

	// Recheck if any pending bits remain.
	ic.hasPending = ic.anyPending()

	return true
}

// anyPending returns true if any register has a pending bit set.
func (ic *IntC) anyPending() bool {
	for i := 0; i < intcRegCount; i++ {
		if ic.regs[i]&0x88 != 0 {
			return true
		}
	}
	return false
}

// Reset clears all interrupt enable registers.
func (ic *IntC) Reset() {
	ic.regs = [intcRegCount]uint8{}
	ic.hasPending = false
}
