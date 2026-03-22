package ilink

import (
	"context"
	"encoding/binary"
	"fmt"
)

// SILKDecoder decodes SILK v3 audio data to raw PCM (signed 16-bit LE, mono).
// sampleRate is typically 24000 for WeChat voice messages.
//
// Users can plug in any SILK decoder implementation. For example, using
// github.com/youthlin/silk:
//
//	client := ilink.NewClient(token, ilink.WithSILKDecoder(
//	    func(data []byte, sampleRate int) ([]byte, error) {
//	        return silk.Decode(bytes.NewReader(data), silk.WithSampleRate(sampleRate))
//	    },
//	))
type SILKDecoder func(silkData []byte, sampleRate int) (pcm []byte, err error)

// WithSILKDecoder sets the SILK audio decoder used by [Client.DownloadVoice].
func WithSILKDecoder(dec SILKDecoder) Option { return func(c *Client) { c.silkDecoder = dec } }

// DefaultVoiceSampleRate is the sample rate used by WeChat voice messages.
const DefaultVoiceSampleRate = 24000

// DownloadVoice downloads a voice message from CDN, decrypts it, decodes
// the audio, and returns a WAV file. Requires a SILK decoder to be
// configured via [WithSILKDecoder].
//
// The sample rate is read from voice.SampleRate; if zero, [DefaultVoiceSampleRate]
// (24000) is used as fallback.
func (c *Client) DownloadVoice(ctx context.Context, voice *VoiceItem) ([]byte, error) {
	if c.silkDecoder == nil {
		return nil, fmt.Errorf("ilink: no SILK decoder configured; use WithSILKDecoder")
	}
	if voice == nil || voice.Media == nil {
		return nil, fmt.Errorf("ilink: voice item or media is nil")
	}

	// 1. Download and decrypt from CDN
	data, err := c.DownloadFile(ctx, voice.Media.EncryptQueryParam, voice.Media.AESKey)
	if err != nil {
		return nil, fmt.Errorf("ilink: download voice: %w", err)
	}

	// 2. Determine sample rate from VoiceItem, fallback to default
	sampleRate := voice.SampleRate
	if sampleRate <= 0 {
		sampleRate = DefaultVoiceSampleRate
	}

	// 3. Decode -> PCM
	pcm, err := c.silkDecoder(data, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("ilink: decode voice: %w", err)
	}

	// 4. Wrap PCM in WAV container
	return BuildWAV(pcm, sampleRate, 1, 16), nil
}

// BuildWAV wraps raw PCM data in a WAV container.
//
// Parameters:
//   - pcm: raw PCM samples (signed 16-bit little-endian)
//   - sampleRate: samples per second (e.g. 24000)
//   - numChannels: 1 for mono, 2 for stereo
//   - bitsPerSample: typically 16
func BuildWAV(pcm []byte, sampleRate, numChannels, bitsPerSample int) []byte {
	dataSize := len(pcm)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8

	// 44-byte header + PCM data
	wav := make([]byte, 44+dataSize)

	// RIFF header
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(36+dataSize))
	copy(wav[8:12], "WAVE")

	// fmt subchunk
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16) // subchunk size
	binary.LittleEndian.PutUint16(wav[20:22], 1)  // audio format (PCM)
	binary.LittleEndian.PutUint16(wav[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(wav[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(wav[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(wav[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(wav[34:36], uint16(bitsPerSample))

	// data subchunk
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))
	copy(wav[44:], pcm)

	return wav
}
