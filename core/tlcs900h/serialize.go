package tlcs900h

import (
	"encoding/binary"
	"errors"
)

const cpuSerializeVersion = 1

// SerializeSize is the number of bytes required for serialization.
// Version(1) + Banks(4*4*4=64) + XIX(4) + XIY(4) + XIZ(4) + XSP(4) +
// PC(4) + SR(2) + FP(1) + cycles(8) + halted(1) + stopped(1) +
// deficit(4) + pendingLevel(1) + pendingVec(1) + hasPending(1) +
// intNest(2)
// = 107
const SerializeSize = 107

// Serialize writes the CPU state into buf.
func (c *CPU) Serialize(buf []byte) error {
	if len(buf) < SerializeSize {
		return errors.New("tlcs900h: serialize buffer too small")
	}

	le := binary.LittleEndian
	off := 0

	buf[off] = cpuSerializeVersion
	off++

	// Register banks
	for bank := 0; bank < 4; bank++ {
		b := &c.reg.Bank[bank]
		le.PutUint32(buf[off:], b.XWA)
		off += 4
		le.PutUint32(buf[off:], b.XBC)
		off += 4
		le.PutUint32(buf[off:], b.XDE)
		off += 4
		le.PutUint32(buf[off:], b.XHL)
		off += 4
	}

	// Dedicated registers
	le.PutUint32(buf[off:], c.reg.XIX)
	off += 4
	le.PutUint32(buf[off:], c.reg.XIY)
	off += 4
	le.PutUint32(buf[off:], c.reg.XIZ)
	off += 4
	le.PutUint32(buf[off:], c.reg.XSP)
	off += 4
	le.PutUint32(buf[off:], c.reg.PC)
	off += 4
	le.PutUint16(buf[off:], c.reg.SR)
	off += 2
	buf[off] = c.reg.FP
	off++

	// Internal state
	le.PutUint64(buf[off:], c.cycles)
	off += 8

	buf[off] = boolByte(c.halted)
	off++
	buf[off] = boolByte(c.stopped)
	off++

	le.PutUint32(buf[off:], uint32(int32(c.deficit)))
	off += 4

	// Interrupt state
	buf[off] = c.pendingLevel
	off++
	buf[off] = c.pendingVec
	off++
	buf[off] = boolByte(c.hasPending)
	off++

	le.PutUint16(buf[off:], c.intNest)

	return nil
}

// Deserialize restores CPU state from buf.
func (c *CPU) Deserialize(buf []byte) error {
	if len(buf) < SerializeSize {
		return errors.New("tlcs900h: deserialize buffer too small")
	}
	if buf[0] != cpuSerializeVersion {
		return errors.New("tlcs900h: unsupported serialize version")
	}

	le := binary.LittleEndian
	off := 1

	// Register banks
	for bank := 0; bank < 4; bank++ {
		b := &c.reg.Bank[bank]
		b.XWA = le.Uint32(buf[off:])
		off += 4
		b.XBC = le.Uint32(buf[off:])
		off += 4
		b.XDE = le.Uint32(buf[off:])
		off += 4
		b.XHL = le.Uint32(buf[off:])
		off += 4
	}

	// Dedicated registers
	c.reg.XIX = le.Uint32(buf[off:])
	off += 4
	c.reg.XIY = le.Uint32(buf[off:])
	off += 4
	c.reg.XIZ = le.Uint32(buf[off:])
	off += 4
	c.reg.XSP = le.Uint32(buf[off:])
	off += 4
	c.reg.PC = le.Uint32(buf[off:])
	off += 4
	c.reg.SR = le.Uint16(buf[off:])
	off += 2
	c.reg.FP = buf[off]
	off++

	// Internal state
	c.cycles = le.Uint64(buf[off:])
	off += 8

	c.halted = buf[off] != 0
	off++
	c.stopped = buf[off] != 0
	off++

	c.deficit = int(int32(le.Uint32(buf[off:])))
	off += 4

	// Interrupt state
	c.pendingLevel = buf[off]
	off++
	c.pendingVec = buf[off]
	off++
	c.hasPending = buf[off] != 0
	off++

	c.intNest = le.Uint16(buf[off:])

	return nil
}

func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
