package core

// ADC emulates the TMP95C061 10-bit A/D converter.
//
// Registers:
//
//	$60-$67: ADREG0-3 (result registers, read-only, 10-bit)
//	$6D: ADMOD (mode/control register)
//
// ADMOD bit layout:
//
//	Bit 7: EOCF - End of conversion flag (read-only, set when done)
//	Bit 6: BUSY - Conversion in progress (read-only)
//	Bit 5: REPEAT - Repeat mode (restart after completion)
//	Bit 4: SCAN - Channel scan mode (sweep channels 0..N)
//	Bit 3: SPEED - 0=160 cycles, 1=320 cycles
//	Bit 2: START - Write 1 to start conversion (auto-clears)
//	Bits 1-0: CH - Channel select (0=AN0, 1=AN1, 2=AN2, 3=AN3)
//
// On the NGPC, AN0 is connected to the battery voltage divider.
// The result is a 10-bit value where $3FF = full battery.
type ADC struct {
	admod      uint8
	result     [4]uint16
	cyclesLeft int
	an         [4]uint16 // analog input values (10-bit)
	ic         *IntC
}

// NewADC creates an A/D converter that signals conversions via ic.
func NewADC(ic *IntC) *ADC {
	a := &ADC{ic: ic}
	// Default AN0 to full battery ($3FF)
	a.an[0] = 0x3FF
	return a
}

// ReadADREG reads an A/D result register byte. offset is 0-7
// corresponding to $60-$67.
//
// Even offsets (low byte): bits 9-2 of result in upper 8 bits,
// lower 6 bits padded with 1s. Clears INTAD pending flag.
// Odd offsets (high byte): bits 9-8 in lower 2 bits.
// Any read clears EOCF (bit 7 of ADMOD).
func (a *ADC) ReadADREG(offset int) uint8 {
	if offset < 0 || offset > 7 {
		return 0
	}
	a.admod &^= 0x80 // clear EOCF on any ADREG read
	ch := offset >> 1
	if offset&1 != 0 {
		// High byte: upper 2 bits of 10-bit result
		return uint8(a.result[ch] >> 2)
	}
	// Low byte: lower 8 bits shifted, clears INTAD pending
	if a.ic != nil {
		a.ic.regs[0] &^= 0x80 // clear INTAD pending (reg 0, high)
	}
	return uint8(a.result[ch]<<6) | 0x3F
}

// ReadADMOD returns the current ADMOD register value.
func (a *ADC) ReadADMOD() uint8 {
	return a.admod
}

// WriteADMOD writes the ADMOD register. If bit 2 (START) is set,
// begins a conversion.
func (a *ADC) WriteADMOD(val uint8) {
	// Preserve read-only bits (EOCF and BUSY)
	val = (a.admod & 0xC0) | (val & 0x3F)

	if val&0x04 != 0 {
		// Start conversion
		val &^= 0x04 // clear START bit
		val |= 0x40  // set BUSY
		if val&0x08 != 0 {
			a.cyclesLeft = 320
		} else {
			a.cyclesLeft = 160
		}
	}

	a.admod = val
}

// Tick advances the A/D converter by the given number of CPU cycles.
// When a conversion completes, the result is stored and an interrupt
// is signaled via the interrupt controller.
func (a *ADC) Tick(cycles int) {
	if a.cyclesLeft <= 0 {
		return
	}

	a.cyclesLeft -= cycles
	if a.cyclesLeft > 0 {
		return
	}
	a.cyclesLeft = 0

	ch := a.admod & 0x03
	if a.admod&0x10 == 0 {
		// Fixed channel mode
		a.result[ch] = a.an[ch] & 0x3FF
	} else {
		// Scan mode: convert channels 0..ch
		for i := uint8(0); i <= ch; i++ {
			a.result[i] = a.an[i] & 0x3FF
		}
	}

	// Conversion done: clear BUSY, set EOCF
	a.admod &^= 0x40
	a.admod |= 0x80

	// Signal INTAD pending
	if a.ic != nil {
		a.ic.SetPending(0, true) // INTAD is reg 0 (INTE0AD), high source
	}

	// Repeat mode: restart conversion
	if a.admod&0x20 != 0 {
		if a.admod&0x08 != 0 {
			a.cyclesLeft = 320
		} else {
			a.cyclesLeft = 160
		}
	}
}

// SetAN sets the analog input value for a channel (0-3).
// The value is 10-bit (0-$3FF).
func (a *ADC) SetAN(ch int, val uint16) {
	if ch >= 0 && ch < 4 {
		a.an[ch] = val & 0x3FF
	}
}

// Reset clears the A/D converter state.
func (a *ADC) Reset() {
	a.admod = 0
	a.result = [4]uint16{}
	a.cyclesLeft = 0
	// Preserve analog input values - they represent hardware state
}
