package core

import "testing"

func TestTimerBasicOverflow(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Configure Timer 0: oT1 clock (every divT1 CPU cycles), TREG0=10
	tm.WriteReg(0x22, 10)   // TREG0 = 10
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1)
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	// 9 T1 ticks, should not overflow
	tm.Tick(9 * divT1)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not be pending after 9 T1 ticks")
	}

	// 1 more T1 tick, total 10 T1 ticks -> overflow
	tm.Tick(divT1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after 10 T1 ticks")
	}
}

func TestTimerT4Clock(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Configure Timer 0: oT4 clock, TREG0=5
	tm.WriteReg(0x22, 5)    // TREG0 = 5
	tm.WriteReg(0x24, 0x02) // T01MOD: T0CLK=10(oT4)
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	// 4 T4 ticks, should not overflow
	tm.Tick(4 * divT4)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not be pending after 4 T4 ticks")
	}

	// 1 more T4 tick -> overflow
	tm.Tick(divT4)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after 5 T4 ticks")
	}
}

func TestTimerCascade(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 0: oT1 clock, TREG0=4 (overflows every 4 T1 ticks)
	// Timer 1: cascade from T0 (T1CLK=00), TREG1=3 (overflows after 3 T0 overflows)
	tm.WriteReg(0x22, 4)    // TREG0 = 4
	tm.WriteReg(0x23, 3)    // TREG1 = 3
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1), T1CLK=00(cascade)
	tm.WriteReg(0x20, 0x83) // TRUN: T0+T1 run + prescaler

	// 2 T0 overflows = 8 T1 ticks. T1 counter=2, no overflow.
	// Add extra cycles that don't complete a third T1 tick boundary for T0
	tm.Tick(8*divT1 + 3*divT1) // 11 T1 ticks. T0: ovf at 4,8 (2 ovf), counter=3
	if ic.regs[3]&0x80 != 0 {
		t.Fatal("INTT1 should not be pending after 2 T0 overflows")
	}

	// 1 more T1 tick (total 12 T1 ticks).
	// T0 overflows at tick 12 (3rd overflow). T1 counter=3 -> overflow.
	tm.Tick(divT1)
	if ic.regs[3]&0x80 == 0 {
		t.Fatal("INTT1 should be pending after 12 T1 ticks (3 T0 overflows)")
	}
}

func TestTimerT23Pair(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 3: oT1 clock, TREG3=20
	tm.WriteReg(0x27, 20)   // TREG3 = 20
	tm.WriteReg(0x28, 0x04) // T23MOD: T3CLK=01(oT1)
	tm.WriteReg(0x20, 0x88) // TRUN: T3 run + prescaler

	// 19 T1 ticks
	tm.Tick(19 * divT1)
	if ic.regs[4]&0x80 != 0 {
		t.Fatal("INTT3 should not be pending after 19 T1 ticks")
	}

	// 1 more T1 tick
	tm.Tick(divT1)
	if ic.regs[4]&0x80 == 0 {
		t.Fatal("INTT3 should be pending after 20 T1 ticks")
	}
}

func TestTimerPrescalerNotRunning(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 0 running but prescaler off
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x20, 0x01) // T0 run, NO prescaler

	tm.Tick(10000)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not fire with prescaler off")
	}
}

func TestTimerTREGZero(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// TREG0=0 means period of 256 T1 ticks
	tm.WriteReg(0x22, 0)
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1)
	tm.WriteReg(0x20, 0x81) // T0 run + prescaler

	// 255 T1 ticks
	tm.Tick(255 * divT1)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not be pending after 255 T1 ticks with TREG0=0")
	}

	// 1 more T1 tick
	tm.Tick(divT1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after 256 T1 ticks with TREG0=0")
	}
}

func TestTimerReset(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	tm.WriteReg(0x22, 10)
	tm.WriteReg(0x20, 0x81)
	tm.Tick(5 * divT1)

	tm.Reset()

	if tm.trun != 0 || tm.counter8[0] != 0 {
		t.Fatal("Reset should clear all state")
	}
}

func TestTimerCounterResetOnStart(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Start timer, tick partway, stop, restart
	tm.WriteReg(0x22, 10)
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1)
	tm.WriteReg(0x20, 0x81) // start T0
	tm.Tick(7 * divT1)      // 7 T1 ticks, counter at 7

	// Stop T0, keep prescaler
	tm.WriteReg(0x20, 0x80)
	// Restart T0 - counter should reset
	tm.WriteReg(0x20, 0x81)

	// Should need full 10 T1 ticks again
	tm.Tick(9 * divT1) // 9 T1 ticks
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("Counter should have been reset on restart")
	}
	tm.Tick(divT1) // 10th T1 tick
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("Should overflow after 10 T1 ticks from restart")
	}
}

func TestTimerMultipleOverflows(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// TREG0=5, tick 23 T1 ticks
	// 4 overflows (at 5, 10, 15, 20), counter=3
	tm.WriteReg(0x22, 5)
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1)
	tm.WriteReg(0x20, 0x81)

	tm.Tick(23 * divT1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after multiple overflows")
	}
	if tm.counter8[0] != 3 {
		t.Fatalf("Counter should be 3 after 23 T1 ticks with TREG=5, got %d", tm.counter8[0])
	}
}

func TestTimerT23Cascade(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 2: oT1 clock, TREG2=2
	// Timer 3: cascade from T2 (T3CLK=00), TREG3=3
	tm.WriteReg(0x26, 2)    // TREG2 = 2
	tm.WriteReg(0x27, 3)    // TREG3 = 3
	tm.WriteReg(0x28, 0x01) // T23MOD: T2CLK=01(oT1), T3CLK=00(cascade)
	tm.WriteReg(0x20, 0x8C) // TRUN: T2+T3 run + prescaler

	// T2 overflows at 2, 4 T1 ticks (2 overflows in 4 T1 ticks)
	// T3 reaches 2 after 4 T1 ticks
	tm.Tick(5 * divT1) // 5 T1 ticks: T2 ovf at 2,4 (2 ovf), counter=1. T3=2.
	if ic.regs[4]&0x80 != 0 {
		t.Fatal("INTT3 should not be pending after 5 T1 ticks")
	}

	// T2 overflows at 6 T1 ticks (3rd overflow). T3 reaches 3 -> overflow.
	tm.Tick(divT1) // 1 more T1 tick = total 6
	if ic.regs[4]&0x80 == 0 {
		t.Fatal("INTT3 should be pending after 6 T1 ticks (3 T2 overflows)")
	}
}

func TestTimer16BasicOverflow(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Configure T4: T1 clock (bits 1-0 = 01), TREG4=100
	tm.WriteReg(0x30, 100)  // TREG4L = 100
	tm.WriteReg(0x31, 0)    // TREG4H = 0 -> TREG4 = 100
	tm.WriteReg(0x38, 0x01) // T4MOD: T4CLK=T1
	tm.WriteReg(0x20, 0x90) // TRUN: T4/T5 run + prescaler

	// 99 T1 ticks
	tm.Tick(99 * divT1)
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("INTTR4 should not be pending after 99 T1 ticks")
	}

	// 1 more T1 tick
	tm.Tick(divT1)
	if ic.regs[5]&0x08 == 0 {
		t.Fatal("INTTR4 should be pending after 100 T1 ticks")
	}
}

func TestTimer16HighCompare(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// TREG4 = $0104 = 260
	tm.WriteReg(0x30, 0x04) // TREG4L
	tm.WriteReg(0x31, 0x01) // TREG4H
	tm.WriteReg(0x38, 0x01) // T4MOD: T4CLK=T1
	tm.WriteReg(0x20, 0x90) // TRUN: T4/T5 run + prescaler

	// 259 T1 ticks
	tm.Tick(259 * divT1)
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("INTTR4 should not be pending after 259 T1 ticks")
	}

	tm.Tick(divT1)
	if ic.regs[5]&0x08 == 0 {
		t.Fatal("INTTR4 should be pending after 260 T1 ticks")
	}
}

func TestTimer16T67Channel(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// T6: T4 clock (bits 1-0 = 10), TREG6=50
	tm.WriteReg(0x40, 50)   // TREG6L
	tm.WriteReg(0x41, 0)    // TREG6H
	tm.WriteReg(0x48, 0x02) // T5MOD: T6CLK=T4
	tm.WriteReg(0x20, 0xA0) // TRUN: T6/T7 run + prescaler

	// 49 T4 ticks
	tm.Tick(49 * divT4)
	if ic.regs[6]&0x08 != 0 {
		t.Fatal("INTTR6 should not be pending after 49 T4 ticks")
	}

	// 1 more T4 tick
	tm.Tick(divT4)
	if ic.regs[6]&0x08 == 0 {
		t.Fatal("INTTR6 should be pending after 50 T4 ticks")
	}
}

func TestTimer16PrescalerOff(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	tm.WriteReg(0x30, 1)
	tm.WriteReg(0x31, 0)
	tm.WriteReg(0x38, 0x01)
	tm.WriteReg(0x20, 0x10) // T4/T5 run but NO prescaler

	tm.Tick(10000)
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("INTTR4 should not fire with prescaler off")
	}
}

func TestTimer16ExternalClockIgnored(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// T4MOD clock = 00 (external), should not tick
	tm.WriteReg(0x30, 1)
	tm.WriteReg(0x31, 0)
	tm.WriteReg(0x38, 0x00) // T4MOD: external clock
	tm.WriteReg(0x20, 0x90)

	tm.Tick(10000)
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("INTTR4 should not fire with external clock source")
	}
}

func TestTimer16ZeroCompare(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// TREG4=0 means period of 65536
	tm.WriteReg(0x30, 0)
	tm.WriteReg(0x31, 0)
	tm.WriteReg(0x38, 0x01) // T1 clock
	tm.WriteReg(0x20, 0x90)

	// 65535 T1 ticks
	tm.Tick(65535 * divT1)
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("INTTR4 should not be pending after 65535 T1 ticks")
	}

	tm.Tick(divT1)
	if ic.regs[5]&0x08 == 0 {
		t.Fatal("INTTR4 should be pending after 65536 T1 ticks")
	}
}

func TestTimer16CounterResetOnStart(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	tm.WriteReg(0x30, 10)
	tm.WriteReg(0x31, 0)
	tm.WriteReg(0x38, 0x01)
	tm.WriteReg(0x20, 0x90) // start
	tm.Tick(7 * divT1)      // 7 T1 ticks

	tm.WriteReg(0x20, 0x80) // stop T4/T5
	tm.WriteReg(0x20, 0x90) // restart -> counter resets

	tm.Tick(9 * divT1) // 9 T1 ticks
	if ic.regs[5]&0x08 != 0 {
		t.Fatal("Counter should have reset on restart")
	}
	tm.Tick(divT1) // 10th T1 tick
	if ic.regs[5]&0x08 == 0 {
		t.Fatal("Should overflow after 10 T1 ticks from restart")
	}
}

func TestTimerTO3Z80IRQ(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 3: oT1 clock (T3CLK bits 3-2 = 01), TREG3=10
	tm.WriteReg(0x27, 10)   // TREG3 = 10
	tm.WriteReg(0x28, 0x04) // T23MOD: T3CLK=01(oT1)
	tm.WriteReg(0x20, 0x88) // TRUN: T3 run + prescaler

	// First overflow: TO3 toggles low->high (rising edge), pending=1
	tm.Tick(10 * divT1)
	if n := tm.Z80IRQPending(); n != 1 {
		t.Fatalf("expected 1 pending Z80 IRQ after first overflow, got %d", n)
	}

	// Second overflow: TO3 toggles high->low (falling edge), no new pending
	tm.Tick(10 * divT1)
	if n := tm.Z80IRQPending(); n != 0 {
		t.Fatalf("expected 0 pending Z80 IRQs after falling edge, got %d", n)
	}

	// Third overflow: TO3 toggles low->high again, pending=1
	tm.Tick(10 * divT1)
	if n := tm.Z80IRQPending(); n != 1 {
		t.Fatalf("expected 1 pending Z80 IRQ after third overflow, got %d", n)
	}
}

func TestTimerTO3PendingAccumulates(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 3: oT1 clock, TREG3=5
	tm.WriteReg(0x27, 5)    // TREG3 = 5
	tm.WriteReg(0x28, 0x04) // T23MOD: T3CLK=01(oT1)
	tm.WriteReg(0x20, 0x88) // TRUN: T3 run + prescaler

	// 20 T1 ticks -> 4 overflows
	// Overflows at 5, 10, 15, 20 T1 ticks
	// TO3: false->true->false->true->false
	// Rising edges at overflow 1 and 3 = 2 pending
	tm.Tick(20 * divT1)
	if n := tm.Z80IRQPending(); n != 2 {
		t.Fatalf("expected 2 pending Z80 IRQs after 4 overflows, got %d", n)
	}
}

func TestTimerT0ClockOT4(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 0 with oT4 clock, TREG0=1
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x24, 0x02) // T01MOD: T0CLK=10(oT4)
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	// divT4-1 CPU cycles = 0 oT4 ticks, no overflow
	tm.Tick(divT4 - 1)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not fire before 1 oT4 tick")
	}

	// 1 more cycle = 1 oT4 tick -> overflow
	tm.Tick(1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should fire after 1 oT4 tick")
	}
}

func TestTimerT0ClockOT16(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 0 with oT16 clock, TREG0=1
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x24, 0x03) // T01MOD: T0CLK=11(oT16)
	tm.WriteReg(0x20, 0x81)

	// divT16-1 CPU cycles = 0 oT16 ticks
	tm.Tick(divT16 - 1)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not fire before 1 oT16 tick")
	}

	tm.Tick(1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should fire after 1 oT16 tick")
	}
}

func TestTimerT1ClockOT16(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 1 with oT16 clock, TREG1=1
	// T1CLK=10 for odd timer = oT16
	tm.WriteReg(0x23, 1)    // TREG1 = 1
	tm.WriteReg(0x24, 0x08) // T01MOD: T1CLK=10(oT16), T0CLK=00
	tm.WriteReg(0x20, 0x82) // TRUN: T1 run + prescaler

	// divT16-1 CPU cycles = 0 oT16 ticks
	tm.Tick(divT16 - 1)
	if ic.regs[3]&0x80 != 0 {
		t.Fatal("INTT1 should not fire before 1 oT16 tick")
	}

	tm.Tick(1)
	if ic.regs[3]&0x80 == 0 {
		t.Fatal("INTT1 should fire after 1 oT16 tick")
	}
}

func TestTimerT1ClockOT256(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 1 with oT256 clock, TREG1=1
	// T1CLK=11 for odd timer = oT256
	tm.WriteReg(0x23, 1)    // TREG1 = 1
	tm.WriteReg(0x24, 0x0C) // T01MOD: T1CLK=11(oT256), T0CLK=00
	tm.WriteReg(0x20, 0x82) // TRUN: T1 run + prescaler

	// divT256-1 CPU cycles = 0 oT256 ticks
	tm.Tick(divT256 - 1)
	if ic.regs[3]&0x80 != 0 {
		t.Fatal("INTT1 should not fire before 1 oT256 tick")
	}

	tm.Tick(1)
	if ic.regs[3]&0x80 == 0 {
		t.Fatal("INTT1 should fire after 1 oT256 tick")
	}
}

func TestTimerT3ClockOT256(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 3 with oT256 clock, TREG3=1
	// T3CLK=11 for odd timer = oT256
	tm.WriteReg(0x27, 1)    // TREG3 = 1
	tm.WriteReg(0x28, 0x0C) // T23MOD: T3CLK=11(oT256), T2CLK=00
	tm.WriteReg(0x20, 0x88) // TRUN: T3 run + prescaler

	// divT256-1 CPU cycles = 0 oT256 ticks
	tm.Tick(divT256 - 1)
	if ic.regs[4]&0x80 != 0 {
		t.Fatal("INTT3 should not fire before 1 oT256 tick")
	}

	tm.Tick(1)
	if ic.regs[4]&0x80 == 0 {
		t.Fatal("INTT3 should fire after 1 oT256 tick")
	}
}

func TestTimerT0ExternalClockStopped(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Timer 0 with external clock (T0CLK=00), should not tick
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x24, 0x00) // T01MOD: T0CLK=00(external)
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	tm.Tick(10000)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not fire with external clock source")
	}
}

func TestTimerT0TI0Basic(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Configure Timer 0: external clock (T0CLK=00), TREG0=3
	tm.WriteReg(0x22, 3)    // TREG0 = 3
	tm.WriteReg(0x24, 0x00) // T01MOD: T0CLK=00(external)
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	// 2 TI0 ticks should not overflow
	tm.TickTI0(2)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not be pending after 2 TI0 ticks")
	}

	// 1 more TI0 tick -> total 3 -> overflow
	tm.TickTI0(1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after 3 TI0 ticks")
	}
}

func TestTimerT0TI0Cascade(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// T0: external clock, TREG0=1 (overflows every TI0 tick)
	// T1: cascade from T0 (T1CLK=00), TREG1=3
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x23, 3)    // TREG1 = 3
	tm.WriteReg(0x24, 0x00) // T01MOD: T0CLK=00(ext), T1CLK=00(cascade)
	tm.WriteReg(0x20, 0x83) // TRUN: T0+T1 run + prescaler

	// 1st TI0: T0 overflows (counter 0->1=TREG), T1 gets 1 overflow
	tm.TickTI0(1)
	if ic.regs[3]&0x08 == 0 {
		t.Fatal("INTT0 should be pending after 1st TI0 tick")
	}
	if ic.regs[3]&0x80 != 0 {
		t.Fatal("INTT1 should not be pending after 1 T0 overflow")
	}

	// 2nd TI0: T0 overflows again, T1 counter=2
	tm.TickTI0(1)
	if ic.regs[3]&0x80 != 0 {
		t.Fatal("INTT1 should not be pending after 2 T0 overflows")
	}

	// 3rd TI0: T0 overflows, T1 counter=3 -> overflow
	tm.TickTI0(1)
	if ic.regs[3]&0x80 == 0 {
		t.Fatal("INTT1 should be pending after 3 T0 overflows (cascade)")
	}
}

func TestTimerT0TI0NotExternalIgnored(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	// Configure Timer 0: oT1 clock (NOT external), TREG0=1
	tm.WriteReg(0x22, 1)    // TREG0 = 1
	tm.WriteReg(0x24, 0x01) // T01MOD: T0CLK=01(oT1), not external
	tm.WriteReg(0x20, 0x81) // TRUN: T0 run + prescaler

	// TickTI0 should be ignored since T0CLK != 00
	tm.TickTI0(10)
	if ic.regs[3]&0x08 != 0 {
		t.Fatal("INTT0 should not fire from TickTI0 when T0CLK != 00")
	}
}

func TestTimerFFCRForcedBits(t *testing.T) {
	ic := &IntC{}
	tm := NewTimers(ic)

	tm.WriteReg(0x39, 0x00) // T4FFCR
	if tm.t4ffcr != 0xC3 {
		t.Fatalf("T4FFCR should have bits 7-6 and 1-0 forced to 1, got $%02X", tm.t4ffcr)
	}

	tm.WriteReg(0x49, 0x10) // T5FFCR
	if tm.t5ffcr != 0xD3 {
		t.Fatalf("T5FFCR should have bits 7-6 and 1-0 forced to 1, got $%02X", tm.t5ffcr)
	}
}
