package core

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

// TestT6W28_SilentOnInit verifies all volumes start at 0x0F (silent)
func TestT6W28_SilentOnInit(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	for ch := 0; ch < 4; ch++ {
		if got := chip.GetVolumeL(ch); got != 0x0F {
			t.Errorf("Left channel %d initial volume: expected 0x0F, got 0x%02X", ch, got)
		}
		if got := chip.GetVolumeR(ch); got != 0x0F {
			t.Errorf("Right channel %d initial volume: expected 0x0F, got 0x%02X", ch, got)
		}
	}

	// Output should be silent
	chip.GenerateSamples(10000)
	bufL, bufR, count := chip.GetBuffers()
	if count == 0 {
		t.Fatal("No samples generated")
	}
	for i := 0; i < count; i++ {
		if bufL[i] != 0 {
			t.Errorf("Initial left output should be silent, got non-zero at index %d: %f", i, bufL[i])
			break
		}
		if bufR[i] != 0 {
			t.Errorf("Initial right output should be silent, got non-zero at index %d: %f", i, bufR[i])
			break
		}
	}
}

// TestT6W28_WriteLeftSetsLeftOnly verifies WriteLeft sets left volume only
func TestT6W28_WriteLeftSetsLeftOnly(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set channel 0 left volume to 5
	chip.WriteLeft(0x95) // 1 00 1 0101 = ch0 volume 5

	if got := chip.GetVolumeL(0); got != 0x05 {
		t.Errorf("Left volume: expected 0x05, got 0x%02X", got)
	}
	if got := chip.GetVolumeR(0); got != 0x0F {
		t.Errorf("Right volume should be unchanged at 0x0F, got 0x%02X", got)
	}
}

// TestT6W28_WriteRightSetsRightOnly verifies WriteRight sets right volume only
func TestT6W28_WriteRightSetsRightOnly(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set channel 1 right volume to 3
	chip.WriteRight(0xB3) // 1 01 1 0011 = ch1 volume 3

	if got := chip.GetVolumeR(1); got != 0x03 {
		t.Errorf("Right volume: expected 0x03, got 0x%02X", got)
	}
	if got := chip.GetVolumeL(1); got != 0x0F {
		t.Errorf("Left volume should be unchanged at 0x0F, got 0x%02X", got)
	}
}

// TestT6W28_SharedToneFrequency verifies both ports share tone frequency via inner PSG
func TestT6W28_SharedToneFrequency(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Write tone frequency via left port
	chip.WriteLeft(0x85) // Latch ch0 tone, low nibble = 5
	chip.WriteLeft(0x02) // High bits = 2, toneReg = 0x25

	// Write a different volume via right port (should not affect tone)
	chip.WriteRight(0x90) // ch0 right volume = 0 (max)

	// Set left volume to max too
	chip.WriteLeft(0x90) // ch0 left volume = 0 (max)

	// Generate samples and verify both channels have non-zero output
	chip.GenerateSamples(10000)
	bufL, bufR, count := chip.GetBuffers()
	if count == 0 {
		t.Fatal("No samples generated")
	}

	hasNonZeroL := false
	hasNonZeroR := false
	for i := 0; i < count; i++ {
		if bufL[i] != 0 {
			hasNonZeroL = true
		}
		if bufR[i] != 0 {
			hasNonZeroR = true
		}
		if hasNonZeroL && hasNonZeroR {
			break
		}
	}
	if !hasNonZeroL {
		t.Error("Left should be non-zero with max volume and active tone")
	}
	if !hasNonZeroR {
		t.Error("Right should be non-zero with max volume and active tone")
	}
}

// TestT6W28_IndependentLatchState verifies each port maintains its own latch
func TestT6W28_IndependentLatchState(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Latch left to ch0 volume
	chip.WriteLeft(0x95) // ch0 left volume = 5

	// Latch right to ch2 volume
	chip.WriteRight(0xD3) // ch2 right volume = 3

	// Data byte via left should go to ch0 left volume
	chip.WriteLeft(0x02) // data byte, low 4 bits = 2
	if got := chip.GetVolumeL(0); got != 0x02 {
		t.Errorf("Left ch0 volume after data byte: expected 0x02, got 0x%02X", got)
	}

	// Data byte via right should go to ch2 right volume
	chip.WriteRight(0x07) // data byte, low 4 bits = 7
	if got := chip.GetVolumeR(2); got != 0x07 {
		t.Errorf("Right ch2 volume after data byte: expected 0x07, got 0x%02X", got)
	}
}

// TestT6W28_DifferentLR verifies different L/R volumes produce different output
func TestT6W28_DifferentLR(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set ch0 tone
	chip.WriteLeft(0x8A) // ch0 tone low = 10
	// Left volume max, right volume mid
	chip.WriteLeft(0x90)  // ch0 left volume = 0 (max)
	chip.WriteRight(0x98) // ch0 right volume = 8

	// Generate samples and find a position where both are non-zero
	chip.GenerateSamples(10000)
	bufL, bufR, count := chip.GetBuffers()
	if count == 0 {
		t.Fatal("No samples generated")
	}

	found := false
	for i := 0; i < count; i++ {
		l := bufL[i]
		r := bufR[i]
		if l != 0 && r != 0 {
			if l == r {
				t.Errorf("L and R should differ: L=%f, R=%f", l, r)
			}
			if l <= r {
				t.Errorf("L (max vol) should be greater than R (vol 8): L=%f, R=%f", l, r)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected at least one sample with both L and R non-zero")
	}
}

// TestT6W28_GetBuffers verifies separate L,R output
func TestT6W28_GetBuffers(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set ch0 tone to 1 (constant HIGH), different L/R volumes
	chip.WriteLeft(0x8A)  // ch0 tone = 10
	chip.WriteLeft(0x90)  // ch0 left vol = 0 (max)
	chip.WriteRight(0x9F) // ch0 right vol = 0x0F (silent)

	chip.GenerateSamples(10000)
	bufL, bufR, count := chip.GetBuffers()

	if count == 0 {
		t.Fatal("No samples generated")
	}

	hasNonZeroLeft := false
	for i := 0; i < count; i++ {
		if bufL[i] != 0 {
			hasNonZeroLeft = true
		}
		if bufR[i] != 0 {
			t.Errorf("Right sample %d should be 0 (silent), got %f", i, bufR[i])
			break
		}
	}
	if !hasNonZeroLeft {
		t.Error("Left channel should have non-zero samples")
	}
}

// TestT6W28_BufferPos verifies BufferPos returns the correct count
func TestT6W28_BufferPos(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	chip.ResetBuffer()
	if got := chip.BufferPos(); got != 0 {
		t.Errorf("BufferPos after reset = %d, want 0", got)
	}

	chip.Run(10000)
	pos := chip.BufferPos()
	if pos == 0 {
		t.Error("BufferPos should be non-zero after Run")
	}

	// BufferPos should match GetBuffers count
	_, _, count := chip.GetBuffers()
	if pos != count {
		t.Errorf("BufferPos=%d, GetBuffers count=%d, should match", pos, count)
	}
}

// TestT6W28_RunAccumulates verifies Run accumulates correct sample count
func TestT6W28_RunAccumulates(t *testing.T) {
	clocks := 10000
	split := 4000

	// Reference: single GenerateSamples
	ref := NewT6W28(3072000, 48000, 800)
	ref.WriteLeft(0x8A)
	ref.WriteLeft(0x90)
	ref.GenerateSamples(clocks)
	_, _, refCount := ref.GetBuffers()

	// Test: ResetBuffer + two Run calls
	chip := NewT6W28(3072000, 48000, 800)
	chip.WriteLeft(0x8A)
	chip.WriteLeft(0x90)
	chip.ResetBuffer()
	chip.Run(split)
	chip.Run(clocks - split)
	_, _, runCount := chip.GetBuffers()

	if runCount != refCount {
		t.Errorf("Run accumulated %d samples, GenerateSamples produced %d", runCount, refCount)
	}
}

// TestT6W28_GenerateSamplesMatchesResetRun verifies GenerateSamples = ResetBuffer + Run
func TestT6W28_GenerateSamplesMatchesResetRun(t *testing.T) {
	clocks := 10000

	chip1 := NewT6W28(3072000, 48000, 800)
	chip1.WriteLeft(0x8A)
	chip1.WriteLeft(0x90)
	chip1.GenerateSamples(clocks)
	buf1L, buf1R, count1 := chip1.GetBuffers()

	chip2 := NewT6W28(3072000, 48000, 800)
	chip2.WriteLeft(0x8A)
	chip2.WriteLeft(0x90)
	chip2.ResetBuffer()
	chip2.Run(clocks)
	buf2L, buf2R, count2 := chip2.GetBuffers()

	if count1 != count2 {
		t.Fatalf("Count mismatch: GenerateSamples=%d, ResetBuffer+Run=%d", count1, count2)
	}

	for i := 0; i < count1; i++ {
		if buf1L[i] != buf2L[i] {
			t.Errorf("Left sample %d differs: %f vs %f", i, buf1L[i], buf2L[i])
			break
		}
		if buf1R[i] != buf2R[i] {
			t.Errorf("Right sample %d differs: %f vs %f", i, buf1R[i], buf2R[i])
			break
		}
	}
}

// TestT6W28_Reset verifies Reset clears volumes and latch states
func TestT6W28_Reset(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set various state
	chip.WriteLeft(0x90)  // ch0 left vol = 0
	chip.WriteRight(0xB3) // ch1 right vol = 3
	chip.WriteLeft(0x85)  // ch0 tone = 5
	chip.GenerateSamples(1000)

	chip.Reset()

	for ch := 0; ch < 4; ch++ {
		if got := chip.GetVolumeL(ch); got != 0x0F {
			t.Errorf("After reset: left volume %d expected 0x0F, got 0x%02X", ch, got)
		}
		if got := chip.GetVolumeR(ch); got != 0x0F {
			t.Errorf("After reset: right volume %d expected 0x0F, got 0x%02X", ch, got)
		}
	}

	// Output should be silent after reset
	chip.GenerateSamples(10000)
	bufL, bufR, count := chip.GetBuffers()
	if count == 0 {
		t.Fatal("No samples generated after reset")
	}
	for i := 0; i < count; i++ {
		if bufL[i] != 0 {
			t.Errorf("After reset: left output should be silent, got non-zero at index %d: %f", i, bufL[i])
			break
		}
		if bufR[i] != 0 {
			t.Errorf("After reset: right output should be silent, got non-zero at index %d: %f", i, bufR[i])
			break
		}
	}
}

// TestT6W28_GainScalesBothChannels verifies gain scales both L and R
func TestT6W28_GainScalesBothChannels(t *testing.T) {
	clocks := 10000

	// Generate with gain 0.5
	chip1 := NewT6W28(3072000, 48000, 800)
	chip1.WriteLeft(0x8A)  // ch0 tone = 10
	chip1.WriteLeft(0x90)  // ch0 left vol = 0 (max)
	chip1.WriteRight(0x94) // ch0 right vol = 4
	chip1.SetGain(0.5)
	chip1.GenerateSamples(clocks)
	buf1L, buf1R, count1 := chip1.GetBuffers()

	// Generate with gain 1.0
	chip2 := NewT6W28(3072000, 48000, 800)
	chip2.WriteLeft(0x8A)  // ch0 tone = 10
	chip2.WriteLeft(0x90)  // ch0 left vol = 0 (max)
	chip2.WriteRight(0x94) // ch0 right vol = 4
	chip2.SetGain(1.0)
	chip2.GenerateSamples(clocks)
	buf2L, buf2R, count2 := chip2.GetBuffers()

	if count1 != count2 {
		t.Fatalf("Sample count mismatch: %d vs %d", count1, count2)
	}

	// Find a sample where both have non-zero left to compare ratio
	found := false
	for i := 0; i < count1; i++ {
		l1 := buf1L[i]
		l2 := buf2L[i]
		r1 := buf1R[i]
		r2 := buf2R[i]
		if l1 != 0 && r1 != 0 {
			if math.Abs(float64(l2/l1-2.0)) > 0.001 {
				t.Errorf("Left gain ratio: expected 2.0, got %f", l2/l1)
			}
			if math.Abs(float64(r2/r1-2.0)) > 0.001 {
				t.Errorf("Right gain ratio: expected 2.0, got %f", r2/r1)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("No non-zero samples found to compare gain ratio")
	}

	if got := chip2.GetGain(); got != 1.0 {
		t.Errorf("GetGain() = %f, want 1.0", got)
	}
}

// TestT6W28_SerializeDeserialize verifies round-trip serialization
func TestT6W28_SerializeDeserialize(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	// Set up various state
	chip.WriteLeft(0x8A)  // ch0 tone = 10
	chip.WriteLeft(0x90)  // ch0 left vol = 0
	chip.WriteRight(0x94) // ch0 right vol = 4
	chip.WriteLeft(0xA3)  // ch1 tone = 3
	chip.WriteLeft(0xB5)  // ch1 left vol = 5
	chip.WriteRight(0xB2) // ch1 right vol = 2
	chip.WriteRight(0xE4) // white noise, rate 0
	chip.WriteLeft(0xFA)  // noise left vol = 0xA
	chip.WriteRight(0xF3) // noise right vol = 3

	chip.GenerateSamples(5000)

	buf := make([]byte, SerializeT6W28Size)
	if err := chip.Serialize(buf); err != nil {
		t.Fatal(err)
	}

	chip2 := NewT6W28(3072000, 48000, 800)
	if err := chip2.Deserialize(buf); err != nil {
		t.Fatal(err)
	}

	// Verify volumes match
	for ch := 0; ch < 4; ch++ {
		if chip.GetVolumeL(ch) != chip2.GetVolumeL(ch) {
			t.Errorf("VolumeL[%d]: original=%d, loaded=%d",
				ch, chip.GetVolumeL(ch), chip2.GetVolumeL(ch))
		}
		if chip.GetVolumeR(ch) != chip2.GetVolumeR(ch) {
			t.Errorf("VolumeR[%d]: original=%d, loaded=%d",
				ch, chip.GetVolumeR(ch), chip2.GetVolumeR(ch))
		}
	}

	// Verify round-trip: serialize both and compare
	buf2 := make([]byte, SerializeT6W28Size)
	if err := chip2.Serialize(buf2); err != nil {
		t.Fatal(err)
	}
	for i := range buf {
		if buf[i] != buf2[i] {
			t.Errorf("Serialized byte %d differs: original=0x%02X, round-trip=0x%02X",
				i, buf[i], buf2[i])
			break
		}
	}

	// Verify continuity: generating from deserialized state matches original
	chip.GenerateSamples(10000)
	chip2.GenerateSamples(10000)
	origL, origR, origCount := chip.GetBuffers()
	loadL, loadR, loadCount := chip2.GetBuffers()

	if origCount != loadCount {
		t.Fatalf("Sample count mismatch: original=%d, loaded=%d", origCount, loadCount)
	}

	for i := 0; i < origCount; i++ {
		if origL[i] != loadL[i] {
			t.Errorf("Left sample %d differs: original=%f, loaded=%f", i, origL[i], loadL[i])
			break
		}
		if origR[i] != loadR[i] {
			t.Errorf("Right sample %d differs: original=%f, loaded=%f", i, origR[i], loadR[i])
			break
		}
	}
}

// TestT6W28_SerializeErrors verifies serialization error cases
func TestT6W28_SerializeErrors(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 800)

	if err := chip.Serialize(make([]byte, 10)); err == nil {
		t.Error("Serialize should reject short buffer")
	}

	buf := make([]byte, SerializeT6W28Size)
	if err := chip.Serialize(buf); err != nil {
		t.Fatal(err)
	}

	buf[0] = 99
	if err := chip.Deserialize(buf); err == nil {
		t.Error("Deserialize should reject wrong version")
	}

	if err := chip.Deserialize(make([]byte, 10)); err == nil {
		t.Error("Deserialize should reject short buffer")
	}
}

// TestT6W28_Golden verifies exact output with SHA-256 hash
func TestT6W28_Golden(t *testing.T) {
	chip := NewT6W28(3072000, 48000, 1024)
	chip.SetGain(0.25)

	// Ch0: tone=254 (~440Hz-ish), left vol=0 (max), right vol=4
	chip.WriteLeft(0x8E)  // ch0 tone low = 0xE
	chip.WriteLeft(0x0F)  // ch0 tone high = 0x0F, toneReg = 254
	chip.WriteLeft(0x90)  // ch0 left vol = 0 (max)
	chip.WriteRight(0x94) // ch0 right vol = 4

	// Ch1: tone=127, left vol=8, right vol=0 (max)
	chip.WriteLeft(0xAF)  // ch1 tone low = 0xF
	chip.WriteLeft(0x07)  // ch1 tone high = 0x07, toneReg = 127
	chip.WriteLeft(0xB8)  // ch1 left vol = 8
	chip.WriteRight(0xB0) // ch1 right vol = 0 (max)

	// Silence ch2 and noise
	chip.WriteLeft(0xDF)
	chip.WriteRight(0xDF)
	chip.WriteLeft(0xFF)
	chip.WriteRight(0xFF)

	// Generate one frame worth of clocks (NGPC: 3072000 / 60 = 51200 clocks)
	clocksPerFrame := 3072000 / 60
	chip.GenerateSamples(clocksPerFrame)
	bufL, bufR, count := chip.GetBuffers()

	if count == 0 {
		t.Fatal("No samples generated")
	}

	// Hash L then R buffers (count float32 values each)
	b := make([]byte, count*2*4)
	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(bufL[i]))
	}
	off := count * 4
	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint32(b[off+i*4:], math.Float32bits(bufR[i]))
	}
	hash := sha256.Sum256(b)
	hashStr := fmt.Sprintf("%x", hash)

	expectedHash := "94b22d33c98d8ff19131c713ff974f5510e3347709478e47087aa1705d4fd6cf"
	if hashStr != expectedHash {
		t.Errorf("Golden hash mismatch:\n  got:  %s\n  want: %s", hashStr, expectedHash)
	}
}
