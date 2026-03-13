package core

import "time"

// RTC emulates the real-time clock peripheral at custom I/O $90-$97.
//
// Registers:
//
//	$90: Control (bit 0 = enable, bit 1 = mode)
//	$91: Year (BCD, 00-99)
//	$92: Month (BCD, 01-12)
//	$93: Day (BCD, 01-31)
//	$94: Hour (BCD, 00-23)
//	$95: Minute (BCD, 00-59)
//	$96: Second (BCD, 00-59)
//	$97: Leap year offset (bits 7-4) / day of week (bits 3-0), read-only
//
// When enabled, the RTC ticks once per second and fires INT0.
// Reading $91 latches all time registers for atomic reads.
type RTC struct {
	ctrl         uint8
	regs         [7]uint8 // [0]=year, [1]=month, [2]=day, [3]=hour, [4]=min, [5]=sec, [6]=ldow
	latched      [7]uint8
	isLatched    bool
	cyclesLeft   int
	cyclesPerSec int
	ic           *IntC
}

// NewRTC creates an RTC peripheral clocked at cpuClockHz that signals
// INT0 via ic on each second tick.
func NewRTC(ic *IntC, cpuClockHz int) *RTC {
	r := &RTC{
		ic:           ic,
		cyclesPerSec: cpuClockHz,
		cyclesLeft:   cpuClockHz,
	}
	r.initFromHost()
	return r
}

// initFromHost sets the RTC registers from the host system clock.
func (r *RTC) initFromHost() {
	now := time.Now()
	r.regs[0] = toBCD(now.Year() % 100)
	r.regs[1] = toBCD(int(now.Month()))
	r.regs[2] = toBCD(now.Day())
	r.regs[3] = toBCD(now.Hour())
	r.regs[4] = toBCD(now.Minute())
	r.regs[5] = toBCD(now.Second())
	r.regs[6] = uint8(now.Year()%4)<<4 | uint8(now.Weekday())
}

// ReadReg reads an RTC register. addr is $90-$97.
// Reading $91 latches all registers for atomic multi-register reads.
func (r *RTC) ReadReg(addr uint32) uint8 {
	switch addr {
	case 0x90:
		return r.ctrl
	case 0x91:
		r.latched = r.regs
		r.isLatched = true
		return r.latched[0]
	case 0x92, 0x93, 0x94, 0x95, 0x96, 0x97:
		idx := addr - 0x91
		if r.isLatched {
			return r.latched[idx]
		}
		return r.regs[idx]
	}
	return 0
}

// WriteReg writes an RTC register. addr is $90-$97.
// $97 is read-only and writes are ignored.
func (r *RTC) WriteReg(addr uint32, val uint8) {
	switch addr {
	case 0x90:
		r.ctrl = val
	case 0x91, 0x92, 0x93, 0x94, 0x95, 0x96:
		r.regs[addr-0x91] = val
	}
}

// Tick advances the RTC by the given number of CPU cycles.
// When a full second elapses, the time is advanced and INT0 is fired.
func (r *RTC) Tick(cpuCycles int) {
	if r.ctrl&0x01 == 0 {
		return
	}

	r.cyclesLeft -= cpuCycles
	for r.cyclesLeft <= 0 {
		r.cyclesLeft += r.cyclesPerSec
		r.advanceSecond()
	}
}

// advanceSecond increments the time by one second with BCD carry
// propagation and fires INT0.
func (r *RTC) advanceSecond() {
	sec, carry := bcdInc(r.regs[5])
	if !carry && sec < 0x60 {
		r.regs[5] = sec
		r.fireINT0()
		return
	}

	// Second rolled over
	r.regs[5] = 0x00
	min, carry := bcdInc(r.regs[4])
	if !carry && min < 0x60 {
		r.regs[4] = min
		r.fireINT0()
		return
	}

	// Minute rolled over
	r.regs[4] = 0x00
	hour, carry := bcdInc(r.regs[3])
	if !carry && hour < 0x24 {
		r.regs[3] = hour
		r.fireINT0()
		return
	}

	// Hour rolled over
	r.regs[3] = 0x00
	r.advanceDay()
	r.fireINT0()
}

// advanceDay increments the day with proper month/year carry.
func (r *RTC) advanceDay() {
	year := fromBCD(r.regs[0])
	month := fromBCD(r.regs[1])
	day := fromBCD(r.regs[2])

	maxDay := daysInMonth(month, year)
	day++
	if day <= maxDay {
		r.regs[2] = toBCD(day)
		r.updateLeapDOW()
		return
	}

	// Day rolled over
	r.regs[2] = 0x01
	month++
	if month <= 12 {
		r.regs[1] = toBCD(month)
		r.updateLeapDOW()
		return
	}

	// Month rolled over
	r.regs[1] = 0x01
	year = (year + 1) % 100
	r.regs[0] = toBCD(year)
	r.updateLeapDOW()
}

// updateLeapDOW recalculates the $97 register (leap year offset and
// day of week) from the current date registers.
func (r *RTC) updateLeapDOW() {
	year := fromBCD(r.regs[0])
	month := fromBCD(r.regs[1])
	day := fromBCD(r.regs[2])

	leap := uint8(year % 4)

	// Reconstruct full year (2000-2099 range) for Weekday calculation
	t := time.Date(2000+year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	dow := uint8(t.Weekday())

	r.regs[6] = leap<<4 | dow
}

// fireINT0 sets the INT0 pending flag.
func (r *RTC) fireINT0() {
	if r.ic != nil {
		r.ic.SetPending(0, false) // INT0 is reg 0 (INTE0AD), low source
	}
}

// Reset clears cycle counters and latch state. Time registers are
// preserved since the RTC is battery-backed on real hardware.
func (r *RTC) Reset() {
	r.cyclesLeft = r.cyclesPerSec
	r.isLatched = false
}

// daysInMonth returns the number of days in the given month (1-12)
// for a 2-digit year (relative to 2000).
func daysInMonth(month, year int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if year%4 == 0 {
			return 29
		}
		return 28
	}
	return 30
}

// bcdInc increments a BCD value by 1. Returns the result and whether
// the low nibble carried (result may exceed valid BCD range; caller
// must check bounds).
func bcdInc(v uint8) (uint8, bool) {
	lo := v & 0x0F
	hi := v >> 4
	lo++
	if lo > 9 {
		lo = 0
		hi++
	}
	if hi > 9 {
		return 0x00, true
	}
	return hi<<4 | lo, false
}

// fromBCD converts a BCD byte to an integer 0-99.
func fromBCD(v uint8) int {
	return int(v>>4)*10 + int(v&0x0F)
}
