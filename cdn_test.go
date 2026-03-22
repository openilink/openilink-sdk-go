package ilink

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestEncryptDecryptAESECB_RoundTrip(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"exactly one block", bytes.Repeat([]byte("x"), 16)},
		{"two blocks", bytes.Repeat([]byte("y"), 32)},
		{"non-aligned", bytes.Repeat([]byte("z"), 37)},
		{"large", bytes.Repeat([]byte("a"), 4096)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := EncryptAESECB(tt.plaintext, key)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			if len(ciphertext)%aes.BlockSize != 0 {
				t.Fatalf("ciphertext length %d not multiple of block size", len(ciphertext))
			}

			decrypted, err := DecryptAESECB(ciphertext, key)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(decrypted), len(tt.plaintext))
			}
		})
	}
}

func TestEncryptAESECB_InvalidKey(t *testing.T) {
	_, err := EncryptAESECB([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestDecryptAESECB_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 16)
	// Not a multiple of block size
	_, err := DecryptAESECB([]byte("not-aligned-data!x"), key)
	if err == nil {
		t.Fatal("expected error for non-aligned ciphertext")
	}
}

func TestAESECBPaddedSize(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 16},   // ceil((0+1)/16)*16 = 16
		{1, 16},   // ceil((1+1)/16)*16 = 16
		{15, 16},  // ceil((15+1)/16)*16 = 16
		{16, 32},  // ceil((16+1)/16)*16 = 32
		{31, 32},  // ceil((31+1)/16)*16 = 32
		{32, 48},  // ceil((32+1)/16)*16 = 48
		{100, 112},
	}

	for _, tt := range tests {
		got := AESECBPaddedSize(tt.input)
		if got != tt.want {
			t.Errorf("AESECBPaddedSize(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAESECBPaddedSize_MatchesActualEncryption(t *testing.T) {
	key := make([]byte, 16)
	for _, size := range []int{0, 1, 15, 16, 31, 32, 100, 1000} {
		plaintext := make([]byte, size)
		ciphertext, err := EncryptAESECB(plaintext, key)
		if err != nil {
			t.Fatalf("encrypt(%d bytes): %v", size, err)
		}
		predicted := AESECBPaddedSize(size)
		if len(ciphertext) != predicted {
			t.Errorf("size=%d: actual ciphertext=%d, predicted=%d", size, len(ciphertext), predicted)
		}
	}
}

func TestBuildCDNDownloadURL(t *testing.T) {
	url := BuildCDNDownloadURL("https://cdn.example.com/c2c", "abc=123&foo")
	want := "https://cdn.example.com/c2c/download?encrypted_query_param=abc%3D123%26foo"
	if url != want {
		t.Errorf("got %s, want %s", url, want)
	}
}

func TestBuildCDNUploadURL(t *testing.T) {
	url := BuildCDNUploadURL("https://cdn.example.com/c2c", "param=1", "key123")
	want := "https://cdn.example.com/c2c/upload?encrypted_query_param=param%3D1&filekey=key123"
	if url != want {
		t.Errorf("got %s, want %s", url, want)
	}
}

func TestParseAESKey_Raw16Bytes(t *testing.T) {
	raw := make([]byte, 16)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	key, err := ParseAESKey(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(key, raw) {
		t.Fatalf("key mismatch: got %x, want %x", key, raw)
	}
}

func TestParseAESKey_HexEncoded(t *testing.T) {
	raw := make([]byte, 16)
	for i := range raw {
		raw[i] = byte(i + 0xa0)
	}
	hexStr := hex.EncodeToString(raw)
	encoded := base64.StdEncoding.EncodeToString([]byte(hexStr))

	key, err := ParseAESKey(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(key, raw) {
		t.Fatalf("key mismatch: got %x, want %x", key, raw)
	}
}

func TestParseAESKey_RawStdEncoding(t *testing.T) {
	raw := make([]byte, 16)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	// Unpadded base64
	encoded := base64.RawStdEncoding.EncodeToString(raw)

	key, err := ParseAESKey(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(key, raw) {
		t.Fatalf("key mismatch")
	}
}

func TestParseAESKey_Invalid(t *testing.T) {
	_, err := ParseAESKey("!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParseAESKey_WrongLength(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := ParseAESKey(encoded)
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}
