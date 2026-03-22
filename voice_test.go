package ilink

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBuildWAV(t *testing.T) {
	pcm := make([]byte, 480) // 10ms at 24kHz mono 16-bit
	wav := BuildWAV(pcm, 24000, 1, 16)

	// Check total size
	wantSize := 44 + len(pcm)
	if len(wav) != wantSize {
		t.Fatalf("WAV size = %d, want %d", len(wav), wantSize)
	}

	// RIFF header
	if string(wav[0:4]) != "RIFF" {
		t.Error("missing RIFF marker")
	}
	chunkSize := binary.LittleEndian.Uint32(wav[4:8])
	if int(chunkSize) != 36+len(pcm) {
		t.Errorf("chunk size = %d, want %d", chunkSize, 36+len(pcm))
	}
	if string(wav[8:12]) != "WAVE" {
		t.Error("missing WAVE marker")
	}

	// fmt subchunk
	if string(wav[12:16]) != "fmt " {
		t.Error("missing fmt marker")
	}
	subchunk1Size := binary.LittleEndian.Uint32(wav[16:20])
	if subchunk1Size != 16 {
		t.Errorf("subchunk1 size = %d, want 16", subchunk1Size)
	}
	audioFormat := binary.LittleEndian.Uint16(wav[20:22])
	if audioFormat != 1 {
		t.Errorf("audio format = %d, want 1 (PCM)", audioFormat)
	}
	numChannels := binary.LittleEndian.Uint16(wav[22:24])
	if numChannels != 1 {
		t.Errorf("channels = %d, want 1", numChannels)
	}
	sampleRate := binary.LittleEndian.Uint32(wav[24:28])
	if sampleRate != 24000 {
		t.Errorf("sample rate = %d, want 24000", sampleRate)
	}
	byteRate := binary.LittleEndian.Uint32(wav[28:32])
	if byteRate != 48000 { // 24000 * 1 * 2
		t.Errorf("byte rate = %d, want 48000", byteRate)
	}
	blockAlign := binary.LittleEndian.Uint16(wav[32:34])
	if blockAlign != 2 {
		t.Errorf("block align = %d, want 2", blockAlign)
	}
	bitsPerSample := binary.LittleEndian.Uint16(wav[34:36])
	if bitsPerSample != 16 {
		t.Errorf("bits per sample = %d, want 16", bitsPerSample)
	}

	// data subchunk
	if string(wav[36:40]) != "data" {
		t.Error("missing data marker")
	}
	dataSize := binary.LittleEndian.Uint32(wav[40:44])
	if int(dataSize) != len(pcm) {
		t.Errorf("data size = %d, want %d", dataSize, len(pcm))
	}

	// PCM data preserved
	if !bytes.Equal(wav[44:], pcm) {
		t.Error("PCM data mismatch")
	}
}

func TestBuildWAV_Stereo(t *testing.T) {
	pcm := make([]byte, 960) // 10ms at 24kHz stereo 16-bit
	wav := BuildWAV(pcm, 24000, 2, 16)

	numChannels := binary.LittleEndian.Uint16(wav[22:24])
	if numChannels != 2 {
		t.Errorf("channels = %d, want 2", numChannels)
	}
	byteRate := binary.LittleEndian.Uint32(wav[28:32])
	if byteRate != 96000 { // 24000 * 2 * 2
		t.Errorf("byte rate = %d, want 96000", byteRate)
	}
	blockAlign := binary.LittleEndian.Uint16(wav[32:34])
	if blockAlign != 4 {
		t.Errorf("block align = %d, want 4", blockAlign)
	}
}

func TestDownloadVoice_NoDecoder(t *testing.T) {
	client := NewClient("token")
	_, err := client.DownloadVoice(nil, &CDNMedia{})
	if err == nil {
		t.Fatal("expected error without SILK decoder")
	}
}

func TestDownloadVoice_NilMedia(t *testing.T) {
	client := NewClient("token", WithSILKDecoder(
		func(data []byte, sr int) ([]byte, error) { return nil, nil },
	))
	_, err := client.DownloadVoice(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil media")
	}
}
