package core

import (
	"testing"

	"github.com/user-none/ecolr/core/tlcs900h"
)

func newTestIntCCPU(t *testing.T) (*IntC, *tlcs900h.CPU) {
	t.Helper()
	mem, err := NewMemory(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := tlcs900h.New(mem)
	// Set IFF to 0 so all interrupts are accepted
	regs := c.Registers()
	regs.SR &= 0x8FFF
	c.SetState(regs)
	ic := &IntC{}
	return ic, c
}

func TestIntC_SetPending_Low(t *testing.T) {
	ic := &IntC{}
	ic.SetPending(0, false) // INT0 pending (reg 0, low)
	if ic.regs[0]&0x08 == 0 {
		t.Error("expected low pending bit set")
	}
}

func TestIntC_SetPending_High(t *testing.T) {
	ic := &IntC{}
	ic.SetPending(0, true) // INTAD pending (reg 0, high)
	if ic.regs[0]&0x80 == 0 {
		t.Error("expected high pending bit set")
	}
}

func TestIntC_WriteReg_CannotSetPending(t *testing.T) {
	ic := &IntC{}
	// Try to set pending bits via WriteReg - should not work
	ic.WriteReg(0, 0x88)
	if ic.regs[0]&0x88 != 0 {
		t.Errorf("pending bits should not be settable via WriteReg, got %02X", ic.regs[0])
	}
}

func TestIntC_WriteReg_CanClearPending(t *testing.T) {
	ic := &IntC{}
	ic.SetPending(0, true)  // set high pending
	ic.SetPending(0, false) // set low pending
	if ic.regs[0]&0x88 != 0x88 {
		t.Fatalf("setup: both pending bits should be set, got %02X", ic.regs[0])
	}
	// Write with bit 7 clear to clear high pending, keep low pending
	ic.WriteReg(0, 0x08)
	if ic.regs[0]&0x80 != 0 {
		t.Error("high pending should have been cleared")
	}
	if ic.regs[0]&0x08 == 0 {
		t.Error("low pending should still be set")
	}
}

func TestIntC_WriteReg_SetsPriority(t *testing.T) {
	ic := &IntC{}
	ic.WriteReg(0, 0x35) // high pri=3, low pri=5
	if ic.regs[0]&0x77 != 0x35 {
		t.Errorf("priority bits = %02X, want $35", ic.regs[0]&0x77)
	}
}

func TestIntC_CheckInterrupts_NoPending(t *testing.T) {
	ic, c := newTestIntCCPU(t)
	ic.WriteReg(0, 0x55) // priorities set but no pending
	if ic.CheckInterrupts(c) {
		t.Error("should not fire with no pending interrupts")
	}
}

func TestIntC_CheckInterrupts_FiresINT0(t *testing.T) {
	ic, c := newTestIntCCPU(t)
	ic.WriteReg(0, 0x05)    // INT0 priority=5, no INTAD priority
	ic.SetPending(0, false) // INT0 pending
	if !ic.CheckInterrupts(c) {
		t.Error("should fire INT0")
	}
	// INT0 vector offset is $28, index = $28/4 = 10
	// Can't directly check what was requested, but pending should be cleared
	if ic.regs[0]&0x08 != 0 {
		t.Error("INT0 pending should be cleared after firing")
	}
}

func TestIntC_CheckInterrupts_FiresINTAD(t *testing.T) {
	ic, c := newTestIntCCPU(t)
	ic.WriteReg(0, 0x50)   // INTAD priority=5, INT0 disabled
	ic.SetPending(0, true) // INTAD pending
	if !ic.CheckInterrupts(c) {
		t.Error("should fire INTAD")
	}
	if ic.regs[0]&0x80 != 0 {
		t.Error("INTAD pending should be cleared after firing")
	}
}

func TestIntC_CheckInterrupts_HigherPriorityWins(t *testing.T) {
	ic, c := newTestIntCCPU(t)
	// INT0 at priority 3, INTAD at priority 5
	ic.WriteReg(0, 0x53)
	ic.SetPending(0, false) // INT0 pending
	ic.SetPending(0, true)  // INTAD pending
	ic.CheckInterrupts(c)
	// INTAD (higher priority) should be cleared, INT0 should remain
	if ic.regs[0]&0x80 != 0 {
		t.Error("INTAD pending should be cleared (it was fired)")
	}
	if ic.regs[0]&0x08 == 0 {
		t.Error("INT0 pending should still be set (lower priority)")
	}
}

func TestIntC_CheckInterrupts_DisabledPriorityNotFired(t *testing.T) {
	ic, c := newTestIntCCPU(t)
	ic.WriteReg(0, 0x00) // both priorities disabled
	ic.SetPending(0, false)
	ic.SetPending(0, true)
	if ic.CheckInterrupts(c) {
		t.Error("should not fire with priority 0 (disabled)")
	}
	// Pending bits should remain since nothing was fired
	if ic.regs[0]&0x88 != 0x88 {
		t.Errorf("pending bits should remain, got %02X", ic.regs[0])
	}
}

func TestIntC_Reset(t *testing.T) {
	ic := &IntC{}
	ic.WriteReg(0, 0x55)
	ic.SetPending(0, true)
	ic.Reset()
	for i := 0; i < intcRegCount; i++ {
		if ic.regs[i] != 0 {
			t.Errorf("reg[%d] = %02X after reset, want 0", i, ic.regs[i])
		}
	}
}
