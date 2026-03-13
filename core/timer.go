package core

// Timers emulates the TMP95C061 timer system: four 8-bit timers (T0-T3)
// and four 16-bit timers (T4-T7).
//
// 8-bit timer register map:
//
//	$20: TRUN   - Timer run control
//	$22: TREG0  - Timer 0 compare value (8-bit)
//	$23: TREG1  - Timer 1 compare value (8-bit)
//	$24: T01MOD - Timer 0/1 mode
//	$25: TFFCR  - Timer flip-flop control
//	$26: TREG2  - Timer 2 compare value (8-bit)
//	$27: TREG3  - Timer 3 compare value (8-bit)
//	$28: T23MOD - Timer 2/3 mode
//	$29: TRDC   - Timer 2/3 flip-flop control
//
// 16-bit timer register map:
//
//	$30/$31: TREG4L/H - Timer 4 compare value (16-bit)
//	$32/$33: TREG5L/H - Timer 5 compare value (16-bit)
//	$34-$37: CAP1/CAP2 - Capture registers (read-only)
//	$38: T4MOD  - Timer 4/5 mode
//	$39: T4FFCR - Timer 4/5 flip-flop control
//	$3A: T45CR  - Timer 4/5 control
//	$40/$41: TREG6L/H - Timer 6 compare value (16-bit)
//	$42/$43: TREG7L/H - Timer 7 compare value (16-bit)
//	$44-$47: CAP3/CAP4 - Capture registers (read-only)
//	$48: T5MOD  - Timer 6/7 mode
//	$49: T5FFCR - Timer 6/7 flip-flop control
//
// TRUN bit layout:
//
//	Bit 0: T0 run
//	Bit 1: T1 run
//	Bit 2: T2 run
//	Bit 3: T3 run
//	Bit 4: T4/T5 channel run
//	Bit 5: T6/T7 channel run
//	Bit 7: PRRUN (prescaler run)
//
// 8-bit TxMOD clock source encoding:
//
//	T0 (T01MOD bits 1-0): 00=TI0(ext), 01=oT1, 10=oT4, 11=oT16
//	T1 (T01MOD bits 3-2): 00=cascade,  01=oT1, 10=oT16, 11=oT256
//	T2 (T23MOD bits 1-0): 00=-(rsvd),  01=oT1, 10=oT4, 11=oT16
//	T3 (T23MOD bits 3-2): 00=cascade,  01=oT1, 10=oT16, 11=oT256
//
// 16-bit TxMOD clock source encoding (bits 1-0):
//
//	00=external (treated as stopped), 01=T1, 10=T4, 11=T16
//
// Prescaler tap periods (in CPU cycles, derived from fc):
//
//	T1   = fc/8    = 8 CPU cycles per tick
//	T4   = fc/32   = 32 CPU cycles per tick
//	T16  = fc/128  = 128 CPU cycles per tick
//	T256 = fc/2048 = 2048 CPU cycles per tick
//
// Each 8-bit timer counts up from 0. When counter matches TREGn, the
// timer overflows (resets to 0) and fires the corresponding interrupt.
// Period = TREGn ticks (or 256 when TREGn == 0).
//
// Each 16-bit timer counts up similarly. Period = TREGn ticks (or
// 65536 when TREGn == 0).
type Timers struct {
	ic *IntC

	// 8-bit timer registers
	trun   uint8
	treg   [4]uint8
	t01mod uint8
	tffcr  uint8
	t23mod uint8
	trdc   uint8

	// 16-bit timer registers
	treg16 [4]uint16 // TREG4-TREG7
	cap    [4]uint16 // CAP1-CAP4 (read-only capture registers)
	t4mod  uint8     // T4MOD ($38)
	t4ffcr uint8     // T4FFCR ($39)
	t45cr  uint8     // T45CR ($3A)
	t5mod  uint8     // T5MOD ($48)
	t5ffcr uint8     // T5FFCR ($49)

	// Internal state
	counter8   [4]uint8  // 8-bit timer counters
	counter16  [4]uint16 // 16-bit timer counters
	prescaler  uint32    // free-running prescaler counter
	to3        bool      // Timer 3 flip-flop output state
	z80IRQPend int       // pending Z80 IRQ count (TO3 rising edges)
}

// Prescaler tap divisors in CPU cycles.
// The raw fc-derived divisors from the datasheet are fc/8, fc/32, fc/128,
// fc/2048. A base rate multiplier accounts for the relationship between
// CPU states and prescaler input ticks. Tuning this value to match
// observed audio behavior.
const (
	timerBaseRate = 16
	divT1         = 8 * timerBaseRate
	divT4         = 32 * timerBaseRate
	divT16        = 128 * timerBaseRate
	divT256       = 2048 * timerBaseRate
)

// NewTimers creates a timer peripheral that signals interrupts via ic.
func NewTimers(ic *IntC) *Timers {
	return &Timers{ic: ic}
}

// Z80IRQPending returns the number of pending Z80 IRQs from TO3
// rising edges and resets the counter to zero.
func (t *Timers) Z80IRQPending() int {
	n := t.z80IRQPend
	t.z80IRQPend = 0
	return n
}

// ClearZ80IRQPending discards any accumulated Z80 IRQs.
// Called when the Z80 is activated to prevent stale edges from
// before the Z80 was enabled from being delivered as a burst.
func (t *Timers) ClearZ80IRQPending() {
	t.z80IRQPend = 0
}

// ReadReg returns the value of a timer SFR register.
func (t *Timers) ReadReg(addr uint32) uint8 {
	switch addr {
	// TRUN
	case 0x20:
		return t.trun
	// 8-bit timers
	case 0x22:
		return t.treg[0]
	case 0x23:
		return t.treg[1]
	case 0x24:
		return t.t01mod
	case 0x25:
		return t.tffcr
	case 0x26:
		return t.treg[2]
	case 0x27:
		return t.treg[3]
	case 0x28:
		return t.t23mod
	case 0x29:
		return t.trdc
	// T4/T5 channel
	case 0x30:
		return uint8(t.treg16[0])
	case 0x31:
		return uint8(t.treg16[0] >> 8)
	case 0x32:
		return uint8(t.treg16[1])
	case 0x33:
		return uint8(t.treg16[1] >> 8)
	case 0x34:
		return uint8(t.cap[0])
	case 0x35:
		return uint8(t.cap[0] >> 8)
	case 0x36:
		return uint8(t.cap[1])
	case 0x37:
		return uint8(t.cap[1] >> 8)
	case 0x38:
		return t.t4mod
	case 0x39:
		return t.t4ffcr
	case 0x3A:
		return t.t45cr
	// T6/T7 channel
	case 0x40:
		return uint8(t.treg16[2])
	case 0x41:
		return uint8(t.treg16[2] >> 8)
	case 0x42:
		return uint8(t.treg16[3])
	case 0x43:
		return uint8(t.treg16[3] >> 8)
	case 0x44:
		return uint8(t.cap[2])
	case 0x45:
		return uint8(t.cap[2] >> 8)
	case 0x46:
		return uint8(t.cap[3])
	case 0x47:
		return uint8(t.cap[3] >> 8)
	case 0x48:
		return t.t5mod
	case 0x49:
		return t.t5ffcr
	}
	return 0
}

// WriteReg writes a timer SFR register.
func (t *Timers) WriteReg(addr uint32, val uint8) {
	switch addr {
	case 0x20:
		// When an 8-bit timer is newly started (bit goes 0->1), reset counter.
		for i := uint(0); i < 4; i++ {
			if val&(1<<i) != 0 && t.trun&(1<<i) == 0 {
				t.counter8[i] = 0
			}
		}
		// When a 16-bit channel is newly started, reset its counter.
		if val&0x10 != 0 && t.trun&0x10 == 0 {
			t.counter16[0] = 0
			t.counter16[1] = 0
		}
		if val&0x20 != 0 && t.trun&0x20 == 0 {
			t.counter16[2] = 0
			t.counter16[3] = 0
		}
		t.trun = val
	// 8-bit timers
	case 0x22:
		t.treg[0] = val
	case 0x23:
		t.treg[1] = val
	case 0x24:
		t.t01mod = val
	case 0x25:
		t.tffcr = val
	case 0x26:
		t.treg[2] = val
	case 0x27:
		t.treg[3] = val
	case 0x28:
		t.t23mod = val
	case 0x29:
		t.trdc = val
	// T4/T5 channel
	case 0x30:
		t.treg16[0] = (t.treg16[0] & 0xFF00) | uint16(val)
	case 0x31:
		t.treg16[0] = (t.treg16[0] & 0x00FF) | uint16(val)<<8
	case 0x32:
		t.treg16[1] = (t.treg16[1] & 0xFF00) | uint16(val)
	case 0x33:
		t.treg16[1] = (t.treg16[1] & 0x00FF) | uint16(val)<<8
	case 0x34, 0x35, 0x36, 0x37:
		// Capture registers are read-only
	case 0x38:
		t.t4mod = val
	case 0x39:
		// Bits 7-6 and 1-0 forced to 11 on write
		t.t4ffcr = val | 0xC3
	case 0x3A:
		t.t45cr = val
	// T6/T7 channel
	case 0x40:
		t.treg16[2] = (t.treg16[2] & 0xFF00) | uint16(val)
	case 0x41:
		t.treg16[2] = (t.treg16[2] & 0x00FF) | uint16(val)<<8
	case 0x42:
		t.treg16[3] = (t.treg16[3] & 0xFF00) | uint16(val)
	case 0x43:
		t.treg16[3] = (t.treg16[3] & 0x00FF) | uint16(val)<<8
	case 0x44, 0x45, 0x46, 0x47:
		// Capture registers are read-only
	case 0x48:
		t.t5mod = val
	case 0x49:
		// Bits 7-6 and 1-0 forced to 11 on write
		t.t5ffcr = val | 0xC3
	}
}

// TickTI0 delivers external clock ticks to Timer 0 when T0CLK=00.
// Used by HBlank to drive Timer 0 from the K2GE horizontal blank signal.
func (t *Timers) TickTI0(ticks uint32) {
	if t.trun&0x80 == 0 {
		return // prescaler not running
	}
	t0clk := t.t01mod & 0x03
	if t0clk != 0 {
		return // T0 not using external clock
	}
	t0ovf := t.tickTimer8(0, ticks)
	t1clk := (t.t01mod >> 2) & 0x03
	if t1clk == 0 {
		t.tickTimer8(1, t0ovf)
	}
}

// Tick advances all timers by the given number of CPU cycles.
func (t *Timers) Tick(cpuCycles int) {
	if t.trun&0x80 == 0 {
		return // prescaler not running
	}

	oldPre := t.prescaler
	t.prescaler += uint32(cpuCycles)

	// Calculate how many ticks each prescaler tap produced.
	t1ticks := t.prescaler/divT1 - oldPre/divT1
	t4ticks := t.prescaler/divT4 - oldPre/divT4
	t16ticks := t.prescaler/divT16 - oldPre/divT16
	t256ticks := t.prescaler/divT256 - oldPre/divT256

	// Even timer ticks: 0=ext(stopped), 1=oT1, 2=oT4, 3=oT16
	ticksEven := [4]uint32{0, t1ticks, t4ticks, t16ticks}
	// Odd timer ticks: 0=cascade(separate), 1=oT1, 2=oT16, 3=oT256
	ticksOdd := [4]uint32{0, t1ticks, t16ticks, t256ticks}

	// 8-bit Timer 0/1 pair
	t0clk := t.t01mod & 0x03
	t1clk := (t.t01mod >> 2) & 0x03
	t0ovf := t.tickTimer8(0, ticksEven[t0clk])
	if t1clk == 0 {
		t.tickTimer8(1, t0ovf)
	} else {
		t.tickTimer8(1, ticksOdd[t1clk])
	}

	// 8-bit Timer 2/3 pair
	t2clk := t.t23mod & 0x03
	t3clk := (t.t23mod >> 2) & 0x03
	t2ovf := t.tickTimer8(2, ticksEven[t2clk])
	if t3clk == 0 {
		t.tickTimer8(3, t2ovf)
	} else {
		t.tickTimer8(3, ticksOdd[t3clk])
	}

	// 16-bit ticks (index 0 = external = 0 ticks)
	ticks16 := [4]uint32{0, t1ticks, t4ticks, t16ticks}

	// 16-bit T4/T5 channel (TRUN bit 4)
	if t.trun&0x10 != 0 {
		t4clk := t.t4mod & 0x03
		t.tickTimer16(0, ticks16[t4clk])
		t.tickTimer16(1, ticks16[t4clk])
	}

	// 16-bit T6/T7 channel (TRUN bit 5)
	if t.trun&0x20 != 0 {
		t6clk := t.t5mod & 0x03
		t.tickTimer16(2, ticks16[t6clk])
		t.tickTimer16(3, ticks16[t6clk])
	}
}

// tickTimer8 advances an 8-bit timer by the given number of source ticks.
// Returns the number of overflows (for cascade chaining).
func (t *Timers) tickTimer8(idx int, srcTicks uint32) uint32 {
	if srcTicks == 0 || t.trun&(1<<uint(idx)) == 0 {
		return 0
	}

	compare := uint32(t.treg[idx])
	if compare == 0 {
		compare = 256
	}

	counter := uint32(t.counter8[idx])
	total := counter + srcTicks

	if total < compare {
		t.counter8[idx] = uint8(total)
		return 0
	}

	remaining := total - compare
	overflows := uint32(1) + remaining/compare
	t.counter8[idx] = uint8(remaining % compare)

	t.fireInterrupt8(idx)
	if idx == 3 {
		for range overflows {
			t.toggleTO3()
		}
	}

	return overflows
}

// toggleTO3 inverts the Timer 3 flip-flop output. On rising edges,
// increments the pending Z80 IRQ counter.
func (t *Timers) toggleTO3() {
	t.to3 = !t.to3
	if t.to3 {
		t.z80IRQPend++
	}
}

// tickTimer16 advances a 16-bit timer by the given number of source ticks.
func (t *Timers) tickTimer16(idx int, srcTicks uint32) {
	if srcTicks == 0 {
		return
	}

	compare := uint32(t.treg16[idx])
	if compare == 0 {
		compare = 65536
	}

	counter := uint32(t.counter16[idx])
	total := counter + srcTicks

	if total < compare {
		t.counter16[idx] = uint16(total)
		return
	}

	remaining := total - compare
	t.counter16[idx] = uint16(remaining % compare)

	t.fireInterrupt16(idx)
}

// fireInterrupt8 sets the pending flag for an 8-bit timer interrupt.
//
//	INTT0: IntC reg 3, low source
//	INTT1: IntC reg 3, high source
//	INTT2: IntC reg 4, low source
//	INTT3: IntC reg 4, high source
func (t *Timers) fireInterrupt8(idx int) {
	if t.ic == nil {
		return
	}
	switch idx {
	case 0:
		t.ic.SetPending(3, false)
	case 1:
		t.ic.SetPending(3, true)
	case 2:
		t.ic.SetPending(4, false)
	case 3:
		t.ic.SetPending(4, true)
	}
}

// fireInterrupt16 sets the pending flag for a 16-bit timer interrupt.
//
//	INTTR4: IntC reg 5, low source
//	INTTR5: IntC reg 5, high source
//	INTTR6: IntC reg 6, low source
//	INTTR7: IntC reg 6, high source
func (t *Timers) fireInterrupt16(idx int) {
	if t.ic == nil {
		return
	}
	switch idx {
	case 0:
		t.ic.SetPending(5, false)
	case 1:
		t.ic.SetPending(5, true)
	case 2:
		t.ic.SetPending(6, false)
	case 3:
		t.ic.SetPending(6, true)
	}
}

// Reset clears all timer state.
func (t *Timers) Reset() {
	*t = Timers{ic: t.ic}
}
