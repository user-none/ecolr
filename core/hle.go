package core

import (
	"time"

	"github.com/user-none/ecolr/core/tlcs900h"
)

// hleBIOS provides High-Level Emulation of the NGPC BIOS when no real BIOS
// ROM is available. It generates a synthetic BIOS ROM and registers an opcode
// handler to intercept system calls.
type hleBIOS struct {
	mem *Memory
}

// newHLEBIOS creates an hleBIOS instance for the given Memory.
func newHLEBIOS(mem *Memory) *hleBIOS {
	return &hleBIOS{
		mem: mem,
	}
}

// install registers the HLE opcode handler with the CPU and sets the initial
// CPU state for HLE boot (PC to cart entry point, SR, XSP).
func (h *hleBIOS) install(c *tlcs900h.CPU) {
	c.RegisterOp(hleOpcode, h.dispatch)
	c.SetPC(h.mem.CartEntryPoint())
}

// hleOpcode is the primary opcode byte claimed by the HLE BIOS layer.
// This must be an undefined slot in the TLCS-900/H first-byte opcode map.
// Other free single-byte opcodes that could be used instead:
//
//	0x1F, 0xC6, 0xD6, 0xE6
const hleOpcode = 0xF6

// Synthetic BIOS ROM layout offsets.
const (
	hleSWI1Offset     = 0x0080 // SWI 1 dispatcher
	hleSysCallBase    = 0x0100 // System call handlers: $0100 + index
	hleSysCallCount   = 27     // Indices $00-$1A
	hleIntBase        = 0x0200 // Interrupt handlers: $0200 + (vector_offset/4)
	hleIntCount       = 33     // 33 vector entries (indices 0-32, through INTTC3)
	hleDefaultHandler = 0x0300 // Default handler (RETI target)
	hleRETIHandler    = 0x0301 // Real RETI opcode (0x07) for default RAM vector target
	hleVectorTable    = 0xFF00 // ROM vector table (33 entries, 4 bytes each)
	hleSWI1JumpTable  = 0xFE00 // SWI 1 jump table (27 entries, 4 bytes each)
)

// CPU address base for BIOS ROM.
const hleBIOSBase = 0xFF0000

// hleSysCallFunc is the signature for system call stub methods.
type hleSysCallFunc func(h *hleBIOS, c *tlcs900h.CPU)

// hleSysCallTable maps system call index to stub function.
var hleSysCallTable = [hleSysCallCount]hleSysCallFunc{
	0x00: (*hleBIOS).hleSysShutdown,
	0x01: (*hleBIOS).hleSysClockGearSet,
	0x02: (*hleBIOS).hleSysRTCGet,
	0x03: (*hleBIOS).hleSysStub03,
	0x04: (*hleBIOS).hleSysIntLvSet,
	0x05: (*hleBIOS).hleSysFontSet,
	0x06: (*hleBIOS).hleSysFlashWrite,
	0x07: (*hleBIOS).hleSysFlashAllErs,
	0x08: (*hleBIOS).hleSysFlashErs,
	0x09: (*hleBIOS).hleSysAlarmSet,
	0x0A: (*hleBIOS).hleSysStub0A,
	0x0B: (*hleBIOS).hleSysAlarmDownSet,
	0x0C: (*hleBIOS).hleSysStub0C,
	0x0D: (*hleBIOS).hleSysFlashProtect,
	0x0E: (*hleBIOS).hleSysGEModeSet,
	0x0F: (*hleBIOS).hleSysStub0F,
	0x10: (*hleBIOS).hleSysComInit,
	0x11: (*hleBIOS).hleSysComSendStart,
	0x12: (*hleBIOS).hleSysComRecvStart,
	0x13: (*hleBIOS).hleSysComCreateData,
	0x14: (*hleBIOS).hleSysComGetData,
	0x15: (*hleBIOS).hleSysComOnRTS,
	0x16: (*hleBIOS).hleSysComOffRTS,
	0x17: (*hleBIOS).hleSysComSendStatus,
	0x18: (*hleBIOS).hleSysComRecvStatus,
	0x19: (*hleBIOS).hleSysComCreateBufData,
	0x1A: (*hleBIOS).hleSysComGetBufData,
}

// generateBIOS builds a 64 KB synthetic BIOS ROM with trap bytes and tables.
func (h *hleBIOS) generateBIOS() []byte {
	rom := make([]byte, biosROMSize)

	// Place 0xF6 at SWI 1 dispatcher offset.
	rom[hleSWI1Offset] = hleOpcode

	// Place 0xF6 at each system call handler offset.
	for i := 0; i < hleSysCallCount; i++ {
		rom[hleSysCallBase+i] = hleOpcode
	}

	// Place 0xF6 at each interrupt handler offset.
	// Entry 0 is reset (vector offset $00), entries 1-32 for offsets $04-$80.
	for i := 0; i < hleIntCount; i++ {
		rom[hleIntBase+i] = hleOpcode
	}

	// Place 0xF6 at default handler offset.
	rom[hleDefaultHandler] = hleOpcode

	// Place a real RETI opcode (0x07) at hleRETIHandler for use as
	// the default RAM vector table target.
	rom[hleRETIHandler] = 0x07

	// Populate ROM vector table at offset $FF00.
	// 33 entries, 4 bytes each (24-bit LE address + pad byte).
	// Entry 0: reset vector (points to default handler for HLE).
	writeLE24(rom[hleVectorTable:], hleBIOSBase+hleDefaultHandler)
	// Entry 1: SWI 1 handler.
	writeLE24(rom[hleVectorTable+4:], hleBIOSBase+hleSWI1Offset)
	// Entries 2-32: interrupt handlers.
	for i := 2; i < hleIntCount; i++ {
		offset := hleVectorTable + i*4
		addr := hleBIOSBase + hleIntBase + i
		writeLE24(rom[offset:], uint32(addr))
	}

	// Populate SWI 1 jump table at offset $FE00.
	// 27 entries, 4 bytes each (24-bit LE address + pad byte).
	for i := 0; i < hleSysCallCount; i++ {
		offset := hleSWI1JumpTable + i*4
		addr := hleBIOSBase + hleSysCallBase + i
		writeLE24(rom[offset:], uint32(addr))
	}

	return rom
}

// writeLE24 writes a 24-bit little-endian value to buf (4 bytes, high byte zeroed).
func writeLE24(buf []byte, val uint32) {
	buf[0] = byte(val)
	buf[1] = byte(val >> 8)
	buf[2] = byte(val >> 16)
	buf[3] = 0
}

// initVectorTable initializes the RAM interrupt vector table at $6FB8.
// 18 entries (4 bytes each, $6FB8-$6FFF). Each is set to the address
// of the RETI opcode in the synthetic BIOS so unhandled interrupts
// return cleanly.
func (h *hleBIOS) initVectorTable(mem *Memory) {
	retiAddr := uint32(hleBIOSBase + hleRETIHandler)
	for i := 0; i < 18; i++ {
		addr := uint32(0x6FB8) + uint32(i)*4
		mem.Write(tlcs900h.Long, addr, retiAddr)
	}
}

// dispatch is the opcode handler called by RegisterOp. It reads PC-1
// to determine which handler address was hit and dispatches accordingly.
func (h *hleBIOS) dispatch(c *tlcs900h.CPU) {
	// PC already advanced past the 0xF6 byte, so the handler address is PC-1.
	addr := (c.Registers().PC - 1) & 0xFFFFFF

	switch {
	case addr == hleBIOSBase+hleSWI1Offset:
		h.hleSWI1Dispatch(c)
	case addr >= hleBIOSBase+hleSysCallBase && addr < hleBIOSBase+hleSysCallBase+hleSysCallCount:
		// Direct call into syscall stub (game read jump table and
		// called through it). Execute the handler and RET to caller.
		idx := int(addr - (hleBIOSBase + hleSysCallBase))
		if hleSysCallTable[idx] != nil {
			hleSysCallTable[idx](h, c)
		} else {
			c.AddCycles(1)
		}
		// Pop return address (4 bytes pushed by CALL).
		regs := c.Registers()
		regs.PC = h.mem.Read(tlcs900h.Long, regs.XSP) & 0xFFFFFF
		regs.XSP += 4
		c.SetState(regs)
	case addr >= hleBIOSBase+hleIntBase && addr < hleBIOSBase+hleIntBase+hleIntCount:
		vecOffset := int(addr-(hleBIOSBase+hleIntBase)) * 4
		h.hleIntDispatch(c, vecOffset)
	case addr == hleBIOSBase+hleDefaultHandler:
		c.AddCycles(1)
	default:
		c.AddCycles(1)
	}
}

// hleSWI1Dispatch reads the call number from register bank 3 and dispatches
// to the corresponding system call stub.
func (h *hleBIOS) hleSWI1Dispatch(c *tlcs900h.CPU) {
	idx := int(c.ReadBank3W() & 0x1F)
	if idx < hleSysCallCount && hleSysCallTable[idx] != nil {
		hleSysCallTable[idx](h, c)
	} else {
		c.AddCycles(1)
	}
	c.RETI()
}

// vecToRAMTable maps vector offset to RAM vector table address at $6FB8+.
var vecToRAMTable = map[int]uint32{
	0x0C: 0x6FB8, // SWI 3
	0x10: 0x6FBC, // SWI 4
	0x14: 0x6FC0, // SWI 5
	0x18: 0x6FC4, // SWI 6
	0x28: 0x6FC8, // INT0 (RTC Alarm)
	0x2C: 0x6FCC, // INT4 (VBlank)
	0x30: 0x6FD0, // INT5 (Z80)
	0x40: 0x6FD4, // INTT0
	0x44: 0x6FD8, // INTT1
	0x48: 0x6FDC, // INTT2
	0x4C: 0x6FE0, // INTT3
	0x60: 0x6FE8, // INTRX0 (Serial RX)
	0x64: 0x6FE4, // INTTX0 (Serial TX)
	0x74: 0x6FF0, // INTTC0
	0x78: 0x6FF4, // INTTC1
	0x7C: 0x6FF8, // INTTC2
	0x80: 0x6FFC, // INTTC3
}

// hleIntDispatch reads the user's handler from the RAM vector table
// and redirects execution there. If the handler points to the default
// RETI handler, performs RETI directly to return cleanly.
// VBlank (vector offset $2C) performs housekeeping before dispatch.
func (h *hleBIOS) hleIntDispatch(c *tlcs900h.CPU, vectorOffset int) {
	if vectorOffset == 0x2C {
		h.hleVBlankHousekeeping()
	}

	ramAddr, ok := vecToRAMTable[vectorOffset]
	if !ok {
		c.RETI()
		return
	}

	handler := h.mem.Read(tlcs900h.Long, ramAddr) & 0xFFFFFF
	defaultHandler := uint32(hleBIOSBase + hleRETIHandler)
	if handler == 0 || handler == defaultHandler {
		c.RETI()
		return
	}

	regs := c.Registers()
	regs.PC = handler
	c.SetState(regs)
	c.AddCycles(1)
}

// hleVBlankHousekeeping performs the minimal BIOS VBlank tasks that
// games depend on: input scanning with edge detection and battery
// voltage reporting. See ngpc_bios.md "VBlank Handler" for the full
// real BIOS sequence.
func (h *hleBIOS) hleVBlankHousekeeping() {
	// Input scanning: read $B0 (active-high) and store to $6F82.
	// The real BIOS $FF26FC performs edge detection with $6C5F
	// (previous state) and stores the result in $6F82.
	// $6C5F keeps the previous value for next-frame comparison.
	raw := h.mem.readIOByte(0xB0)
	h.mem.Write(tlcs900h.Byte, 0x6F82, uint32(raw&0x7F))
	h.mem.Write(tlcs900h.Byte, 0x6C5F, uint32(raw))

	// Battery voltage: report full battery
	h.mem.Write(tlcs900h.Word, 0x6F80, 0x03FF)

	// Service watchdog (write $4E to WDCR)
	h.mem.writeIOByte(0x6F, 0x4E)
}

// intLvSetMap maps interrupt source index (0-9) to IntC register index
// and nibble position for hleSysIntLvSet.
var intLvSetMap = [10]struct {
	reg  int
	high bool
}{
	{0, false},  // 0: INT0 -> $70 low
	{1, true},   // 1: INT5 -> $71 high
	{3, false},  // 2: INTT0 -> $73 low
	{3, true},   // 3: INTT1 -> $73 high
	{4, false},  // 4: INTT2 -> $74 low
	{4, true},   // 5: INTT3 -> $74 high
	{9, false},  // 6: INTTC0 -> $79 low
	{9, true},   // 7: INTTC1 -> $79 high
	{10, false}, // 8: INTTC2 -> $7A low
	{10, true},  // 9: INTTC3 -> $7A high
}

// toBCD converts an integer (0-99) to BCD format.
func toBCD(v int) uint8 {
	return uint8((v/10)<<4 | (v % 10))
}

// $00: Shutdown - halt the CPU.
func (h *hleBIOS) hleSysShutdown(c *tlcs900h.CPU) {
	h.mem.Write(tlcs900h.Byte, 0xB6, 0x50)
	c.Halt()
	c.AddCycles(1)
}

// $01: ClockGearSet - set CPU clock gear via $80 register.
func (h *hleBIOS) hleSysClockGearSet(c *tlcs900h.CPU) {
	gear := c.ReadBank3RB() & 0x07
	if gear > 4 {
		gear = 4
	}
	h.mem.writeIOByte(0x80, gear)
	c.AddCycles(1)
}

// $02: RTCGet - write current time as 7 BCD bytes to destination pointer.
func (h *hleBIOS) hleSysRTCGet(c *tlcs900h.CPU) {
	ptr := c.ReadBank3XHL()
	now := time.Now()

	h.mem.Write(tlcs900h.Byte, ptr+0, uint32(toBCD(now.Year()%100)))
	h.mem.Write(tlcs900h.Byte, ptr+1, uint32(toBCD(int(now.Month()))))
	h.mem.Write(tlcs900h.Byte, ptr+2, uint32(toBCD(now.Day())))
	h.mem.Write(tlcs900h.Byte, ptr+3, uint32(toBCD(now.Hour())))
	h.mem.Write(tlcs900h.Byte, ptr+4, uint32(toBCD(now.Minute())))
	h.mem.Write(tlcs900h.Byte, ptr+5, uint32(toBCD(now.Second())))
	leapYears := uint32(now.Year() % 4)
	h.mem.Write(tlcs900h.Byte, ptr+6, leapYears<<4|uint32(now.Weekday()))
	c.AddCycles(1)
}

// $03: Pure no-op.
func (h *hleBIOS) hleSysStub03(c *tlcs900h.CPU) { c.AddCycles(1) }

// $04: IntLvSet - set interrupt priority level for a source.
func (h *hleBIOS) hleSysIntLvSet(c *tlcs900h.CPU) {
	source := c.ReadBank3RC()
	level := c.ReadBank3RB()

	if source >= 10 {
		c.AddCycles(1)
		return
	}

	entry := intLvSetMap[source]
	val := h.mem.intc.ReadReg(entry.reg)

	if entry.high {
		val = (val & 0x8F) | (level << 4)
	} else {
		val = (val & 0xF8) | level
	}

	h.mem.intc.WriteReg(entry.reg, val)
	c.AddCycles(1)
}

// $05: FontSet - convert 1bpp font to 2bpp and write to K2GE character RAM.
// RA3 bits 0-1 = font color index, bits 4-5 = background color index.
func (h *hleBIOS) hleSysFontSet(c *tlcs900h.CPU) {
	ra := c.ReadBank3RA()
	fontColor := ra & 0x03
	bgColor := (ra >> 4) & 0x03
	fontSet(h.mem, fontColor, bgColor)
	c.AddCycles(1)
}

// $06: FlashWrite - copy data from memory to flash chip.
func (h *hleBIOS) hleSysFlashWrite(c *tlcs900h.CPU) {
	bank := c.ReadBank3RA()
	pageCount := c.ReadBank3BC()
	destOffset := c.ReadBank3XDE()
	srcAddr := c.ReadBank3XHL()

	chip := h.selectFlash(bank)
	if chip == nil {
		c.WriteBank3RA(0xFF)
		c.AddCycles(1)
		return
	}

	byteCount := uint32(pageCount) * 256
	data := chip.Data()
	for i := uint32(0); i < byteCount; i++ {
		if int(destOffset+i) >= len(data) {
			break
		}
		val := h.mem.Read(tlcs900h.Byte, srcAddr+i)
		data[destOffset+i] = uint8(val)
	}

	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $07: FlashAllErs - erase entire flash chip.
func (h *hleBIOS) hleSysFlashAllErs(c *tlcs900h.CPU) {
	bank := c.ReadBank3RA()
	chip := h.selectFlash(bank)
	if chip == nil {
		c.WriteBank3RA(0xFF)
		c.AddCycles(1)
		return
	}

	data := chip.Data()
	for i := range data {
		data[i] = 0xFF
	}

	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $08: FlashErs - erase a single flash block by block number.
func (h *hleBIOS) hleSysFlashErs(c *tlcs900h.CPU) {
	bank := c.ReadBank3RA()
	blockNum := c.ReadBank3RB()
	chip := h.selectFlash(bank)
	if chip == nil {
		c.WriteBank3RA(0xFF)
		c.AddCycles(1)
		return
	}

	offset := uint32(blockNum) * 0x10000
	chip.EraseBlock(offset)

	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $09: AlarmSet - return success.
func (h *hleBIOS) hleSysAlarmSet(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $0A: Pure no-op.
func (h *hleBIOS) hleSysStub0A(c *tlcs900h.CPU) { c.AddCycles(1) }

// $0B: AlarmDownSet - return success.
func (h *hleBIOS) hleSysAlarmDownSet(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $0C: Pure no-op.
func (h *hleBIOS) hleSysStub0C(c *tlcs900h.CPU) { c.AddCycles(1) }

// $0D: FlashProtect - return success.
func (h *hleBIOS) hleSysFlashProtect(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $0E: GEModeSet - configure K2GE mono/color mode from system code at $6F91.
func (h *hleBIOS) hleSysGEModeSet(c *tlcs900h.CPU) {
	h.mem.setGEMode()
	c.AddCycles(1)
}

// $0F: Pure no-op.
func (h *hleBIOS) hleSysStub0F(c *tlcs900h.CPU) { c.AddCycles(1) }

// $10: ComInit - return success.
func (h *hleBIOS) hleSysComInit(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $11: ComSendStart - consume 1 cycle.
func (h *hleBIOS) hleSysComSendStart(c *tlcs900h.CPU) { c.AddCycles(1) }

// $12: ComRecvStart - consume 1 cycle.
func (h *hleBIOS) hleSysComRecvStart(c *tlcs900h.CPU) { c.AddCycles(1) }

// $13: ComCreateData - return success.
func (h *hleBIOS) hleSysComCreateData(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $14: ComGetData - return COM_BUF_EMPTY.
func (h *hleBIOS) hleSysComGetData(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x01)
	c.AddCycles(1)
}

// $15: ComOnRTS - write $00 to $B2.
func (h *hleBIOS) hleSysComOnRTS(c *tlcs900h.CPU) {
	h.mem.Write(tlcs900h.Byte, 0xB2, 0x00)
	c.AddCycles(1)
}

// $16: ComOffRTS - write $01 to $B2.
func (h *hleBIOS) hleSysComOffRTS(c *tlcs900h.CPU) {
	h.mem.Write(tlcs900h.Byte, 0xB2, 0x01)
	c.AddCycles(1)
}

// $17: ComSendStatus - return 0 (no pending bytes).
func (h *hleBIOS) hleSysComSendStatus(c *tlcs900h.CPU) {
	c.WriteBank3RWA(0)
	c.AddCycles(1)
}

// $18: ComRecvStatus - return 0 (no available bytes).
func (h *hleBIOS) hleSysComRecvStatus(c *tlcs900h.CPU) {
	c.WriteBank3RWA(0)
	c.AddCycles(1)
}

// $19: ComCreateBufData - return success.
func (h *hleBIOS) hleSysComCreateBufData(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x00)
	c.AddCycles(1)
}

// $1A: ComGetBufData - return COM_BUF_EMPTY.
func (h *hleBIOS) hleSysComGetBufData(c *tlcs900h.CPU) {
	c.WriteBank3RA(0x01)
	c.AddCycles(1)
}

// selectFlash returns the flash chip for the given bank (0=cs0, 1=cs1).
func (h *hleBIOS) selectFlash(bank uint8) *Flash {
	if bank == 0 {
		return h.mem.cs0
	}
	return h.mem.cs1
}
