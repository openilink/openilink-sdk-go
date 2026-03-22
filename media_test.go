package ilink

import "testing"

func TestIsMediaItem(t *testing.T) {
	tests := []struct {
		typ  MessageItemType
		want bool
	}{
		{ItemNone, false},
		{ItemText, false},
		{ItemImage, true},
		{ItemVoice, true},
		{ItemFile, true},
		{ItemVideo, true},
	}

	for _, tt := range tests {
		item := &MessageItem{Type: tt.typ}
		got := IsMediaItem(item)
		if got != tt.want {
			t.Errorf("IsMediaItem(type=%d) = %v, want %v", tt.typ, got, tt.want)
		}
	}
}

func TestMediaAESKey(t *testing.T) {
	// mediaAESKey should base64-encode the hex string
	hexKey := "0123456789abcdef0123456789abcdef"
	got := mediaAESKey(hexKey)
	// base64("0123456789abcdef0123456789abcdef")
	if got == "" {
		t.Fatal("empty result")
	}
	// Verify it's valid base64 and decodes back to the hex string
	decoded, err := decodeBase64Flexible(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != hexKey {
		t.Errorf("round-trip mismatch: got %q, want %q", string(decoded), hexKey)
	}
}
