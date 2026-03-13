package core

import (
	"encoding/binary"
	"errors"
	"hash/crc32"

	z80 "github.com/user-none/go-chip-z80"

	"github.com/user-none/ecolr/core/tlcs900h"
)

// Save state format constants
const (
	stateVersion    = 1
	stateMagic      = "eColrSState\x00"
	stateHeaderSize = 22 // magic(12) + version(2) + romCRC(4) + dataCRC(4)
)

// Fixed serialization sizes for inline components
const (
	// Memory state: workRAM + z80RAM + k2ge + customIO + scalar fields
	memorySerializeSize = workRAMSize + z80RAMSize + k2geSize + customIOSize +
		1 + 1 + 1 + 1 + 1 + 1 + 1 // inputB0, dacL, dacR, soundChipEn, z80Active, clockGear, k2geModeLocked

	// IntC: 11 registers
	intcSerializeSize = intcRegCount

	// ADC: admod(1) + result(4*2) + cyclesLeft(4) + an(4*2) = 21
	adcSerializeSize = 1 + 4*2 + 4 + 4*2

	// Timers: trun(1) + treg(4) + t01mod(1) + tffcr(1) + t23mod(1) + trdc(1)
	//       + treg16(4*2) + cap(4*2) + t4mod(1) + t4ffcr(1) + t45cr(1) + t5mod(1) + t5ffcr(1)
	//       + counter8(4) + counter16(4*2) + prescaler(4) + to3(1) + z80IRQPend(4) = 51
	timersSerializeSize = 1 + 4 + 1 + 1 + 1 + 1 +
		4*2 + 4*2 + 1 + 1 + 1 + 1 + 1 +
		4 + 4*2 + 4 + 1 + 4

	// RTC: ctrl(1) + regs(7) + latched(7) + isLatched(1) + cyclesLeft(4) = 20
	rtcSerializeSize = 1 + 7 + 7 + 1 + 4

	// Flash: cs0.state(1) + cs1.state(1) = 2
	flashSerializeSize = 2

	// Memory.lastCycle: uint64 = 8
	lastCycleSerializeSize = 8
)

// SerializeSize returns the total size in bytes needed for a save state.
func SerializeSize() int {
	return stateHeaderSize +
		tlcs900h.SerializeSize +
		z80.SerializeSize +
		SerializeT6W28Size +
		memorySerializeSize +
		intcSerializeSize +
		adcSerializeSize +
		timersSerializeSize +
		rtcSerializeSize +
		flashSerializeSize +
		lastCycleSerializeSize
}

// boolByte converts a bool to a uint8 (0 or 1).
func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// Serialize captures the complete emulator state.
func (e *Emulator) Serialize() ([]byte, error) {
	size := SerializeSize()
	data := make([]byte, size)

	// Write header
	copy(data[0:12], stateMagic)
	binary.LittleEndian.PutUint16(data[12:14], stateVersion)
	binary.LittleEndian.PutUint32(data[14:18], e.mem.GetROMCRC32())

	offset := stateHeaderSize

	// TLCS-900/H CPU
	if err := e.cpu.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += tlcs900h.SerializeSize

	// Z80 CPU
	z80cpu := e.mem.Z80CPU()
	if err := z80cpu.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += z80.SerializeSize

	// T6W28 PSG
	if err := e.psg.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += SerializeT6W28Size

	// Memory state
	offset = serializeMemory(e.mem, data, offset)

	// IntC
	offset = serializeIntC(&e.mem.intc, data, offset)

	// ADC
	offset = serializeADC(e.mem.adc, data, offset)

	// Timers
	offset = serializeTimers(e.mem.timers, data, offset)

	// RTC
	offset = serializeRTC(e.mem.rtc, data, offset)

	// Flash state
	offset = serializeFlash(e.mem, data, offset)

	// lastCycle
	binary.LittleEndian.PutUint64(data[offset:], e.mem.lastCycle)

	// Calculate and write data CRC32 (over everything after header)
	dataCRC := crc32.ChecksumIEEE(data[stateHeaderSize:])
	binary.LittleEndian.PutUint32(data[18:22], dataCRC)

	return data, nil
}

// Deserialize restores emulator state from previously serialized data.
func (e *Emulator) Deserialize(data []byte) error {
	if err := e.VerifyState(data); err != nil {
		return err
	}

	offset := stateHeaderSize

	// TLCS-900/H CPU
	if err := e.cpu.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += tlcs900h.SerializeSize

	// Z80 CPU
	z80cpu := e.mem.Z80CPU()
	if err := z80cpu.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += z80.SerializeSize

	// T6W28 PSG
	if err := e.psg.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += SerializeT6W28Size

	// Memory state
	offset = deserializeMemory(e.mem, data, offset)

	// IntC
	offset = deserializeIntC(&e.mem.intc, data, offset)

	// ADC
	offset = deserializeADC(e.mem.adc, data, offset)

	// Timers
	offset = deserializeTimers(e.mem.timers, data, offset)

	// RTC
	offset = deserializeRTC(e.mem.rtc, data, offset)

	// Flash state
	offset = deserializeFlash(e.mem, data, offset)

	// lastCycle
	e.mem.lastCycle = binary.LittleEndian.Uint64(data[offset:])

	return nil
}

// VerifyState checks if a save state is valid without loading it.
func (e *Emulator) VerifyState(data []byte) error {
	expectedSize := SerializeSize()
	if len(data) < expectedSize {
		return errors.New("save state too short")
	}

	if string(data[0:12]) != stateMagic {
		return errors.New("invalid save state magic")
	}

	version := binary.LittleEndian.Uint16(data[12:14])
	if version > stateVersion {
		return errors.New("unsupported save state version")
	}

	romCRC := binary.LittleEndian.Uint32(data[14:18])
	if romCRC != e.mem.GetROMCRC32() {
		return errors.New("save state is for a different ROM")
	}

	expectedCRC := binary.LittleEndian.Uint32(data[18:22])
	actualCRC := crc32.ChecksumIEEE(data[stateHeaderSize:])
	if expectedCRC != actualCRC {
		return errors.New("save state data is corrupted")
	}

	return nil
}

// serializeMemory writes Memory RAM and scalar fields to the data buffer.
func serializeMemory(m *Memory, data []byte, offset int) int {
	copy(data[offset:], m.workRAM[:])
	offset += workRAMSize

	copy(data[offset:], m.z80RAM[:])
	offset += z80RAMSize

	copy(data[offset:], m.k2ge[:])
	offset += k2geSize

	copy(data[offset:], m.customIO[:])
	offset += customIOSize

	data[offset] = m.inputB0
	offset++
	data[offset] = m.dacL
	offset++
	data[offset] = m.dacR
	offset++
	data[offset] = m.soundChipEn
	offset++
	data[offset] = m.z80Active
	offset++
	data[offset] = m.clockGear
	offset++
	data[offset] = boolByte(m.k2geModeLocked)
	offset++

	return offset
}

// deserializeMemory reads Memory RAM and scalar fields from the data buffer.
func deserializeMemory(m *Memory, data []byte, offset int) int {
	copy(m.workRAM[:], data[offset:offset+workRAMSize])
	offset += workRAMSize

	copy(m.z80RAM[:], data[offset:offset+z80RAMSize])
	offset += z80RAMSize

	copy(m.k2ge[:], data[offset:offset+k2geSize])
	offset += k2geSize

	copy(m.customIO[:], data[offset:offset+customIOSize])
	offset += customIOSize

	m.inputB0 = data[offset]
	offset++
	m.dacL = data[offset]
	offset++
	m.dacR = data[offset]
	offset++
	m.soundChipEn = data[offset]
	offset++
	m.z80Active = data[offset]
	offset++
	m.clockGear = data[offset]
	offset++
	m.k2geModeLocked = data[offset] != 0
	offset++

	return offset
}

// serializeIntC writes interrupt controller registers to the data buffer.
func serializeIntC(ic *IntC, data []byte, offset int) int {
	copy(data[offset:], ic.regs[:])
	offset += intcRegCount
	return offset
}

// deserializeIntC reads interrupt controller registers from the data buffer.
func deserializeIntC(ic *IntC, data []byte, offset int) int {
	copy(ic.regs[:], data[offset:offset+intcRegCount])
	offset += intcRegCount
	ic.hasPending = ic.anyPending()
	return offset
}

// serializeADC writes ADC state to the data buffer.
func serializeADC(a *ADC, data []byte, offset int) int {
	le := binary.LittleEndian

	data[offset] = a.admod
	offset++

	for i := 0; i < 4; i++ {
		le.PutUint16(data[offset:], a.result[i])
		offset += 2
	}

	le.PutUint32(data[offset:], uint32(int32(a.cyclesLeft)))
	offset += 4

	for i := 0; i < 4; i++ {
		le.PutUint16(data[offset:], a.an[i])
		offset += 2
	}

	return offset
}

// deserializeADC reads ADC state from the data buffer.
func deserializeADC(a *ADC, data []byte, offset int) int {
	le := binary.LittleEndian

	a.admod = data[offset]
	offset++

	for i := 0; i < 4; i++ {
		a.result[i] = le.Uint16(data[offset:])
		offset += 2
	}

	a.cyclesLeft = int(int32(le.Uint32(data[offset:])))
	offset += 4

	for i := 0; i < 4; i++ {
		a.an[i] = le.Uint16(data[offset:])
		offset += 2
	}

	return offset
}

// serializeTimers writes timer state to the data buffer.
func serializeTimers(t *Timers, data []byte, offset int) int {
	le := binary.LittleEndian

	// 8-bit timer registers
	data[offset] = t.trun
	offset++
	copy(data[offset:], t.treg[:])
	offset += 4
	data[offset] = t.t01mod
	offset++
	data[offset] = t.tffcr
	offset++
	data[offset] = t.t23mod
	offset++
	data[offset] = t.trdc
	offset++

	// 16-bit timer registers
	for i := 0; i < 4; i++ {
		le.PutUint16(data[offset:], t.treg16[i])
		offset += 2
	}
	for i := 0; i < 4; i++ {
		le.PutUint16(data[offset:], t.cap[i])
		offset += 2
	}
	data[offset] = t.t4mod
	offset++
	data[offset] = t.t4ffcr
	offset++
	data[offset] = t.t45cr
	offset++
	data[offset] = t.t5mod
	offset++
	data[offset] = t.t5ffcr
	offset++

	// Internal state
	copy(data[offset:], t.counter8[:])
	offset += 4
	for i := 0; i < 4; i++ {
		le.PutUint16(data[offset:], t.counter16[i])
		offset += 2
	}
	le.PutUint32(data[offset:], t.prescaler)
	offset += 4
	data[offset] = boolByte(t.to3)
	offset++
	le.PutUint32(data[offset:], uint32(int32(t.z80IRQPend)))
	offset += 4

	return offset
}

// deserializeTimers reads timer state from the data buffer.
func deserializeTimers(t *Timers, data []byte, offset int) int {
	le := binary.LittleEndian

	// 8-bit timer registers
	t.trun = data[offset]
	offset++
	copy(t.treg[:], data[offset:offset+4])
	offset += 4
	t.t01mod = data[offset]
	offset++
	t.tffcr = data[offset]
	offset++
	t.t23mod = data[offset]
	offset++
	t.trdc = data[offset]
	offset++

	// 16-bit timer registers
	for i := 0; i < 4; i++ {
		t.treg16[i] = le.Uint16(data[offset:])
		offset += 2
	}
	for i := 0; i < 4; i++ {
		t.cap[i] = le.Uint16(data[offset:])
		offset += 2
	}
	t.t4mod = data[offset]
	offset++
	t.t4ffcr = data[offset]
	offset++
	t.t45cr = data[offset]
	offset++
	t.t5mod = data[offset]
	offset++
	t.t5ffcr = data[offset]
	offset++

	// Internal state
	copy(t.counter8[:], data[offset:offset+4])
	offset += 4
	for i := 0; i < 4; i++ {
		t.counter16[i] = le.Uint16(data[offset:])
		offset += 2
	}
	t.prescaler = le.Uint32(data[offset:])
	offset += 4
	t.to3 = data[offset] != 0
	offset++
	t.z80IRQPend = int(int32(le.Uint32(data[offset:])))
	offset += 4

	return offset
}

// serializeRTC writes RTC state to the data buffer.
func serializeRTC(r *RTC, data []byte, offset int) int {
	data[offset] = r.ctrl
	offset++

	copy(data[offset:], r.regs[:])
	offset += 7

	copy(data[offset:], r.latched[:])
	offset += 7

	data[offset] = boolByte(r.isLatched)
	offset++

	binary.LittleEndian.PutUint32(data[offset:], uint32(int32(r.cyclesLeft)))
	offset += 4

	return offset
}

// deserializeRTC reads RTC state from the data buffer.
func deserializeRTC(r *RTC, data []byte, offset int) int {
	r.ctrl = data[offset]
	offset++

	copy(r.regs[:], data[offset:offset+7])
	offset += 7

	copy(r.latched[:], data[offset:offset+7])
	offset += 7

	r.isLatched = data[offset] != 0
	offset++

	r.cyclesLeft = int(int32(binary.LittleEndian.Uint32(data[offset:])))
	offset += 4

	return offset
}

// serializeFlash writes flash chip state bytes to the data buffer.
func serializeFlash(m *Memory, data []byte, offset int) int {
	if m.cs0 != nil {
		data[offset] = uint8(m.cs0.state)
	}
	offset++

	if m.cs1 != nil {
		data[offset] = uint8(m.cs1.state)
	}
	offset++

	return offset
}

// deserializeFlash reads flash chip state bytes from the data buffer.
func deserializeFlash(m *Memory, data []byte, offset int) int {
	if m.cs0 != nil {
		m.cs0.state = flashState(data[offset])
	}
	offset++

	if m.cs1 != nil {
		m.cs1.state = flashState(data[offset])
	}
	offset++

	return offset
}
