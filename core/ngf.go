package core

import (
	"encoding/binary"
	"errors"
)

// NGF save format constants.
const (
	ngfMagic      = 0x0053
	ngfHeaderSize = 8
	ngfBlockHdr   = 8 // 4-byte address + 4-byte length per block record
	ngfMaxBlocks  = 256
)

// ngfBlock is a single block record for NGF serialization.
type ngfBlock struct {
	addr uint32 // NGPC address
	data []byte
}

// buildNGF compares orig and current cart data and returns an NGF-format
// byte slice containing only the modified flash blocks. Returns nil if
// no blocks have changed.
func buildNGF(orig, current []byte) []byte {
	if len(orig) != len(current) || len(orig) == 0 {
		return nil
	}

	var records []ngfBlock

	// Walk CS0 (first 2 MB or less)
	cs0Size := len(orig)
	if cs0Size > cartCS0Size {
		cs0Size = cartCS0Size
	}
	records = appendChangedBlocks(records, orig[:cs0Size], current[:cs0Size], cs0Size, cartCS0Start)

	// Walk CS1 (second 2 MB, 4 MB carts only)
	if len(orig) > cartCS0Size {
		cs1Orig := orig[cartCS0Size:]
		cs1Cur := current[cartCS0Size:]
		records = appendChangedBlocks(records, cs1Orig, cs1Cur, len(cs1Orig), cartCS1Start)
	}

	if len(records) == 0 {
		return nil
	}

	// Calculate total file size
	totalSize := ngfHeaderSize
	for _, r := range records {
		totalSize += ngfBlockHdr + len(r.data)
	}

	buf := make([]byte, totalSize)
	binary.LittleEndian.PutUint16(buf[0:2], ngfMagic)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(records)))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(totalSize))

	off := ngfHeaderSize
	for _, r := range records {
		binary.LittleEndian.PutUint32(buf[off:off+4], r.addr)
		binary.LittleEndian.PutUint32(buf[off+4:off+8], uint32(len(r.data)))
		copy(buf[off+ngfBlockHdr:], r.data)
		off += ngfBlockHdr + len(r.data)
	}

	return buf
}

// appendChangedBlocks walks flash blocks in a chip region and appends
// records for any blocks that differ between orig and current.
func appendChangedBlocks(records []ngfBlock, orig, current []byte, chipSize int, baseAddr uint32) []ngfBlock {
	off := 0
	for off < chipSize {
		start, end := chipBlockBounds(chipSize, uint32(off))
		if start >= end {
			break
		}
		if end > chipSize {
			end = chipSize
		}

		changed := false
		for i := start; i < end; i++ {
			if orig[i] != current[i] {
				changed = true
				break
			}
		}

		if changed {
			blockData := make([]byte, end-start)
			copy(blockData, current[start:end])
			records = append(records, ngfBlock{
				addr: baseAddr + uint32(start),
				data: blockData,
			})
		}

		off = end
	}
	return records
}

// chipBlockBounds returns the start (inclusive) and end (exclusive) offsets
// for the flash block containing offset within a chip of the given size.
// This mirrors Flash.blockBounds but operates on chip size alone.
func chipBlockBounds(chipSize int, offset uint32) (int, int) {
	if chipSize <= 0 {
		return 0, 0
	}

	bootStart := chipSize - 0x10000
	if bootStart < 0 {
		bootStart = 0
	}

	if int(offset) < bootStart {
		blk := int(offset) & ^0xFFFF
		end := blk + 0x10000
		if end > bootStart {
			end = bootStart
		}
		return blk, end
	}

	bootOff := int(offset) - bootStart
	switch {
	case bootOff < 0x8000:
		return bootStart, bootStart + 0x8000
	case bootOff < 0xA000:
		return bootStart + 0x8000, bootStart + 0xA000
	case bootOff < 0xC000:
		return bootStart + 0xA000, bootStart + 0xC000
	default:
		return bootStart + 0xC000, chipSize
	}
}

// applyNGF parses an NGF byte slice and overlays block records onto dst.
// dst should be a copy of the original ROM data; blocks are written at
// their NGPC addresses translated to file offsets.
func applyNGF(dst []byte, ngf []byte) error {
	if len(ngf) == 0 {
		return nil
	}

	if len(ngf) < ngfHeaderSize {
		return errors.New("ngf: data too short for header")
	}

	magic := binary.LittleEndian.Uint16(ngf[0:2])
	if magic != ngfMagic {
		return errors.New("ngf: invalid magic")
	}

	blockCount := int(binary.LittleEndian.Uint16(ngf[2:4]))
	fileLen := binary.LittleEndian.Uint32(ngf[4:8])

	if uint32(len(ngf)) < fileLen {
		return errors.New("ngf: data shorter than declared file length")
	}

	if blockCount > ngfMaxBlocks {
		return errors.New("ngf: block count exceeds maximum")
	}

	off := ngfHeaderSize
	for i := 0; i < blockCount; i++ {
		if off+ngfBlockHdr > len(ngf) {
			return errors.New("ngf: truncated block header")
		}

		addr := binary.LittleEndian.Uint32(ngf[off : off+4])
		length := int(binary.LittleEndian.Uint32(ngf[off+4 : off+8]))
		off += ngfBlockHdr

		if off+length > len(ngf) {
			return errors.New("ngf: truncated block data")
		}

		fileOffset := ngfAddrToFileOffset(addr)
		if fileOffset < 0 || fileOffset+length > len(dst) {
			off += length
			continue
		}

		copy(dst[fileOffset:fileOffset+length], ngf[off:off+length])
		off += length
	}

	return nil
}

// ngfAddrToFileOffset converts an NGPC address to a file offset within
// the cart buffer. Returns -1 for addresses outside the cart range.
func ngfAddrToFileOffset(addr uint32) int {
	switch {
	case addr >= cartCS0Start && addr <= cartCS0End:
		return int(addr - cartCS0Start)
	case addr >= cartCS1Start && addr <= cartCS1End:
		return int(addr-cartCS1Start) + cartCS0Size
	default:
		return -1
	}
}
