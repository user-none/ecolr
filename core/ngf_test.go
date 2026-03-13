package core

import (
	"encoding/binary"
	"testing"
)

func TestBuildNGFNoChanges(t *testing.T) {
	orig := make([]byte, 512*1024)
	for i := range orig {
		orig[i] = byte(i)
	}
	current := make([]byte, len(orig))
	copy(current, orig)

	ngf := buildNGF(orig, current)
	if ngf != nil {
		t.Errorf("expected nil for no changes, got %d bytes", len(ngf))
	}
}

func TestBuildNGFSingleBlock(t *testing.T) {
	size := 512 * 1024
	orig := make([]byte, size)
	current := make([]byte, size)
	copy(current, orig)

	// Modify one byte in the first 64KB block
	current[0x100] = 0xAB

	ngf := buildNGF(orig, current)
	if ngf == nil {
		t.Fatal("expected non-nil NGF for modified block")
	}

	// Check header
	magic := binary.LittleEndian.Uint16(ngf[0:2])
	if magic != ngfMagic {
		t.Errorf("magic = 0x%04X, want 0x%04X", magic, ngfMagic)
	}
	blockCount := binary.LittleEndian.Uint16(ngf[2:4])
	if blockCount != 1 {
		t.Errorf("block count = %d, want 1", blockCount)
	}
	fileLen := binary.LittleEndian.Uint32(ngf[4:8])
	if fileLen != uint32(len(ngf)) {
		t.Errorf("file length = %d, want %d", fileLen, len(ngf))
	}

	// Check block record
	addr := binary.LittleEndian.Uint32(ngf[8:12])
	if addr != cartCS0Start {
		t.Errorf("block addr = 0x%06X, want 0x%06X", addr, cartCS0Start)
	}
	length := binary.LittleEndian.Uint32(ngf[12:16])
	if length != 0x10000 {
		t.Errorf("block length = %d, want %d", length, 0x10000)
	}

	// Verify block data contains the modification
	blockData := ngf[16 : 16+length]
	if blockData[0x100] != 0xAB {
		t.Errorf("block data[0x100] = 0x%02X, want 0xAB", blockData[0x100])
	}
}

func TestBuildNGFMultipleBlocksCS0CS1(t *testing.T) {
	// 4 MB cart: CS0 (2MB) + CS1 (2MB)
	size := 4 * 1024 * 1024
	orig := make([]byte, size)
	current := make([]byte, size)
	copy(current, orig)

	// Modify a byte in CS0 block 0
	current[0x50] = 0x11
	// Modify a byte in CS1 (offset 0x200000+ in file, block 0 of CS1)
	current[cartCS0Size+0x50] = 0x22

	ngf := buildNGF(orig, current)
	if ngf == nil {
		t.Fatal("expected non-nil NGF")
	}

	blockCount := binary.LittleEndian.Uint16(ngf[2:4])
	if blockCount != 2 {
		t.Errorf("block count = %d, want 2", blockCount)
	}

	// First block should be CS0
	addr0 := binary.LittleEndian.Uint32(ngf[8:12])
	if addr0 != cartCS0Start {
		t.Errorf("block 0 addr = 0x%06X, want 0x%06X", addr0, cartCS0Start)
	}

	// Second block should be CS1
	len0 := int(binary.LittleEndian.Uint32(ngf[12:16]))
	off2 := ngfHeaderSize + ngfBlockHdr + len0
	addr1 := binary.LittleEndian.Uint32(ngf[off2 : off2+4])
	if addr1 != cartCS1Start {
		t.Errorf("block 1 addr = 0x%06X, want 0x%06X", addr1, cartCS1Start)
	}
}

func TestApplyNGFRoundTrip(t *testing.T) {
	size := 1024 * 1024
	orig := make([]byte, size)
	for i := range orig {
		orig[i] = byte(i)
	}
	modified := make([]byte, size)
	copy(modified, orig)

	// Modify several blocks
	modified[0x00010] = 0xFF // block 0
	modified[0x20000] = 0xEE // block 2
	modified[0xF5000] = 0xDD // boot block region

	ngf := buildNGF(orig, modified)
	if ngf == nil {
		t.Fatal("expected non-nil NGF")
	}

	// Apply to a fresh copy of orig
	restored := make([]byte, size)
	copy(restored, orig)
	if err := applyNGF(restored, ngf); err != nil {
		t.Fatalf("applyNGF: %v", err)
	}

	// Should match modified
	for i := range modified {
		if restored[i] != modified[i] {
			t.Errorf("restored[0x%X] = 0x%02X, want 0x%02X", i, restored[i], modified[i])
			break
		}
	}
}

func TestApplyNGFEmpty(t *testing.T) {
	dst := make([]byte, 1024)
	if err := applyNGF(dst, nil); err != nil {
		t.Errorf("applyNGF(nil) = %v, want nil", err)
	}
	if err := applyNGF(dst, []byte{}); err != nil {
		t.Errorf("applyNGF(empty) = %v, want nil", err)
	}
}

func TestApplyNGFZeroBlocks(t *testing.T) {
	// Valid header with 0 blocks
	ngf := make([]byte, ngfHeaderSize)
	binary.LittleEndian.PutUint16(ngf[0:2], ngfMagic)
	binary.LittleEndian.PutUint16(ngf[2:4], 0)
	binary.LittleEndian.PutUint32(ngf[4:8], ngfHeaderSize)

	dst := make([]byte, 1024)
	if err := applyNGF(dst, ngf); err != nil {
		t.Errorf("applyNGF(0 blocks) = %v, want nil", err)
	}
}

func TestApplyNGFInvalidMagic(t *testing.T) {
	ngf := make([]byte, ngfHeaderSize)
	binary.LittleEndian.PutUint16(ngf[0:2], 0xFFFF)

	dst := make([]byte, 1024)
	if err := applyNGF(dst, ngf); err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestApplyNGFTruncatedHeader(t *testing.T) {
	dst := make([]byte, 1024)
	if err := applyNGF(dst, []byte{0x53, 0x00}); err == nil {
		t.Error("expected error for truncated header")
	}
}

func TestApplyNGFTruncatedBlockHeader(t *testing.T) {
	ngf := make([]byte, ngfHeaderSize+4) // only 4 bytes of block header
	binary.LittleEndian.PutUint16(ngf[0:2], ngfMagic)
	binary.LittleEndian.PutUint16(ngf[2:4], 1)
	binary.LittleEndian.PutUint32(ngf[4:8], uint32(len(ngf)))

	dst := make([]byte, 1024)
	if err := applyNGF(dst, ngf); err == nil {
		t.Error("expected error for truncated block header")
	}
}

func TestApplyNGFTruncatedBlockData(t *testing.T) {
	ngf := make([]byte, ngfHeaderSize+ngfBlockHdr+2) // declares 100 bytes but only 2
	binary.LittleEndian.PutUint16(ngf[0:2], ngfMagic)
	binary.LittleEndian.PutUint16(ngf[2:4], 1)
	binary.LittleEndian.PutUint32(ngf[4:8], uint32(len(ngf)))
	binary.LittleEndian.PutUint32(ngf[8:12], cartCS0Start)
	binary.LittleEndian.PutUint32(ngf[12:16], 100)

	dst := make([]byte, 512*1024)
	if err := applyNGF(dst, ngf); err == nil {
		t.Error("expected error for truncated block data")
	}
}

func TestBuildNGFMismatchedLength(t *testing.T) {
	orig := make([]byte, 1024)
	current := make([]byte, 2048)
	ngf := buildNGF(orig, current)
	if ngf != nil {
		t.Error("expected nil for mismatched lengths")
	}
}

func TestBuildNGFEmptyInputs(t *testing.T) {
	ngf := buildNGF(nil, nil)
	if ngf != nil {
		t.Error("expected nil for nil inputs")
	}
	ngf = buildNGF([]byte{}, []byte{})
	if ngf != nil {
		t.Error("expected nil for empty inputs")
	}
}
