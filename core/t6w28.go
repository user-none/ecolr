package core

import (
	"encoding/binary"
	"errors"
	"math"

	sn76489 "github.com/user-none/go-chip-sn76489"
)

// T6W28 emulates the Toshiba T6W28 sound chip used in the Neo Geo Pocket Color.
// It uses two SN76489 instances: tonePSG receives left-port writes (tone
// generators and left volumes) and noisePSG receives right-port writes (noise
// generator and right volumes).
type T6W28 struct {
	tonePSG  *sn76489.SN76489
	noisePSG *sn76489.SN76489

	channelBuffersL [4][]float32
	channelBuffersR [4][]float32
	mixBufferL      []float32
	mixBufferR      []float32
	bufferPos       int

	clocksPerSample float64
	clockCounter    float64
	gain            float32
	volumeTable     []float32
}

// NewT6W28 creates a new T6W28 instance.
// clockFreq is the chip clock frequency, sampleRate is the audio output rate,
// and bufferSize is the number of stereo sample pairs per buffer.
func NewT6W28(clockFreq, sampleRate, bufferSize int) *T6W28 {
	cfg := sn76489.Config{
		LFSRBits:       16,
		WhiteNoiseTaps: 0x0009,
		ToneZero:       sn76489.ToneZeroAsOne,
		LFSRInit:       0xFFFE,
	}

	t := &T6W28{
		tonePSG:         sn76489.New(clockFreq, sampleRate, bufferSize, cfg),
		noisePSG:        sn76489.New(clockFreq, sampleRate, bufferSize, cfg),
		clocksPerSample: float64(clockFreq) / float64(sampleRate),
		gain:            0.25,
		volumeTable:     sn76489.GetVolumeTable(),
		mixBufferL:      make([]float32, bufferSize),
		mixBufferR:      make([]float32, bufferSize),
	}

	for ch := 0; ch < 4; ch++ {
		t.channelBuffersL[ch] = make([]float32, bufferSize)
		t.channelBuffersR[ch] = make([]float32, bufferSize)
	}

	return t
}

// WriteLeft writes a value to the left channel port (tone generators, left volumes).
func (t *T6W28) WriteLeft(value uint8) {
	t.tonePSG.Write(value)
}

// WriteRight writes a value to the right channel port (noise generator, right volumes).
func (t *T6W28) WriteRight(value uint8) {
	t.noisePSG.Write(value)
}

// Run advances the chip by the given number of clocks, accumulating stereo
// samples into the per-channel L/R buffers. Returns the number of samples
// dropped due to buffer overflow.
func (t *T6W28) Run(clocks int) int {
	dropped := 0
	bufSize := len(t.channelBuffersL[0])

	for i := 0; i < clocks; i++ {
		t.tonePSG.Clock()
		t.noisePSG.Clock()
		t.clockCounter++
		if t.clockCounter >= t.clocksPerSample {
			t.clockCounter -= t.clocksPerSample
			if t.bufferPos < bufSize {
				for ch := 0; ch < 3; ch++ {
					if t.tonePSG.GetToneOutput(ch) {
						t.channelBuffersL[ch][t.bufferPos] = t.volumeTable[t.tonePSG.GetVolume(ch)]
						t.channelBuffersR[ch][t.bufferPos] = t.volumeTable[t.noisePSG.GetVolume(ch)]
					} else {
						t.channelBuffersL[ch][t.bufferPos] = 0
						t.channelBuffersR[ch][t.bufferPos] = 0
					}
				}
				if t.noisePSG.GetNoiseOutput() {
					t.channelBuffersL[3][t.bufferPos] = t.volumeTable[t.tonePSG.GetVolume(3)]
					t.channelBuffersR[3][t.bufferPos] = t.volumeTable[t.noisePSG.GetVolume(3)]
				} else {
					t.channelBuffersL[3][t.bufferPos] = 0
					t.channelBuffersR[3][t.bufferPos] = 0
				}
				t.bufferPos++
			} else {
				dropped++
			}
		}
	}
	return dropped
}

// BufferPos returns the current buffer position (number of samples generated).
func (t *T6W28) BufferPos() int {
	return t.bufferPos
}

// ResetBuffer resets the internal buffer position to 0.
func (t *T6W28) ResetBuffer() {
	t.bufferPos = 0
}

// GenerateSamples resets the buffer and runs for the given number of clocks.
// Returns the number of samples dropped due to buffer overflow.
func (t *T6W28) GenerateSamples(clocks int) int {
	t.ResetBuffer()
	return t.Run(clocks)
}

// GetBuffers mixes the per-channel L/R buffers into separate left and right
// output buffers with gain applied. Returns left, right, and the number of
// samples.
func (t *T6W28) GetBuffers() ([]float32, []float32, int) {
	for i := 0; i < t.bufferPos; i++ {
		t.mixBufferL[i] = (t.channelBuffersL[0][i] + t.channelBuffersL[1][i] +
			t.channelBuffersL[2][i] + t.channelBuffersL[3][i]) * t.gain
		t.mixBufferR[i] = (t.channelBuffersR[0][i] + t.channelBuffersR[1][i] +
			t.channelBuffersR[2][i] + t.channelBuffersR[3][i]) * t.gain
	}
	return t.mixBufferL, t.mixBufferR, t.bufferPos
}

// SetGain sets the gain applied to mixed output.
func (t *T6W28) SetGain(gain float32) {
	t.gain = gain
}

// GetGain returns the current gain value.
func (t *T6W28) GetGain() float32 {
	return t.gain
}

// GetVolumeL returns the left volume for the given channel (0-3).
func (t *T6W28) GetVolumeL(ch int) uint8 {
	return t.tonePSG.GetVolume(ch)
}

// GetVolumeR returns the right volume for the given channel (0-3).
func (t *T6W28) GetVolumeR(ch int) uint8 {
	return t.noisePSG.GetVolume(ch)
}

// DebugToneRegs returns the current tone register values for channels 0-2.
func (t *T6W28) DebugToneRegs() [3]uint16 {
	return [3]uint16{
		t.tonePSG.GetToneReg(0),
		t.tonePSG.GetToneReg(1),
		t.tonePSG.GetToneReg(2),
	}
}

// Reset resets the T6W28 to power-on defaults.
func (t *T6W28) Reset() {
	t.tonePSG.Reset()
	t.noisePSG.Reset()
	t.bufferPos = 0
	t.clockCounter = 0
}

// SerializeT6W28Size is the number of bytes needed to serialize the T6W28 state.
// Layout: 1 byte version + 2 * SN76489 state + 8 bytes clock counter.
const SerializeT6W28Size = 1 + sn76489.SerializeSize*2 + 8

const t6w28SerializeVersion = 1

// Serialize writes all mutable T6W28 state into buf.
// Returns an error if len(buf) < SerializeT6W28Size.
func (t *T6W28) Serialize(buf []byte) error {
	if len(buf) < SerializeT6W28Size {
		return errors.New("t6w28: serialize buffer too small")
	}

	buf[0] = t6w28SerializeVersion

	const psgEnd1 = 1 + sn76489.SerializeSize
	const psgEnd2 = psgEnd1 + sn76489.SerializeSize

	if err := t.tonePSG.Serialize(buf[1:psgEnd1]); err != nil {
		return err
	}

	if err := t.noisePSG.Serialize(buf[psgEnd1:psgEnd2]); err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf[psgEnd2:], math.Float64bits(t.clockCounter))

	return nil
}

// Deserialize restores all mutable T6W28 state from buf.
// Returns an error if the buffer is too small or the version is incompatible.
func (t *T6W28) Deserialize(buf []byte) error {
	if len(buf) < SerializeT6W28Size {
		return errors.New("t6w28: deserialize buffer too small")
	}
	if buf[0] != t6w28SerializeVersion {
		return errors.New("t6w28: unsupported serialize version")
	}

	const psgEnd1 = 1 + sn76489.SerializeSize
	const psgEnd2 = psgEnd1 + sn76489.SerializeSize

	if err := t.tonePSG.Deserialize(buf[1:psgEnd1]); err != nil {
		return err
	}

	if err := t.noisePSG.Deserialize(buf[psgEnd1:psgEnd2]); err != nil {
		return err
	}

	t.clockCounter = math.Float64frombits(binary.LittleEndian.Uint64(buf[psgEnd2:]))
	t.bufferPos = 0

	return nil
}
