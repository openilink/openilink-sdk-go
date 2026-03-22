package ilink

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEnsureTrailingSlash(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"https://example.com", "https://example.com/"},
		{"https://example.com/", "https://example.com/"},
		{"", "/"},
	}
	for _, tt := range tests {
		got := ensureTrailingSlash(tt.input)
		if got != tt.want {
			t.Errorf("ensureTrailingSlash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRandomWechatUIN(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		uin := randomWechatUIN()
		if uin == "" {
			t.Fatal("empty UIN")
		}
		seen[uin] = true
	}
	if len(seen) < 90 {
		t.Errorf("expected mostly unique UINs, got %d unique out of 100", len(seen))
	}
}

func TestGenerateClientID(t *testing.T) {
	id := generateClientID()
	if !strings.HasPrefix(id, "sdk-") {
		t.Errorf("expected prefix sdk-, got %s", id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts (sdk-timestamp-hex), got %d: %s", len(parts), id)
	}
	if len(parts[2]) != 8 {
		t.Errorf("expected 8 hex chars, got %d: %s", len(parts[2]), parts[2])
	}

	// Uniqueness
	id2 := generateClientID()
	if id == id2 {
		t.Error("two consecutive IDs should differ")
	}
}

func TestIsTimeoutError(t *testing.T) {
	if !isTimeoutError(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be timeout")
	}
	if !isTimeoutError(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)) {
		t.Error("wrapped DeadlineExceeded should be timeout")
	}
	if isTimeoutError(fmt.Errorf("random error")) {
		t.Error("random error should not be timeout")
	}
	if isTimeoutError(context.Canceled) {
		t.Error("Canceled should not be timeout")
	}

	// net.Error with Timeout
	netErr := &net.OpError{Op: "read", Err: &timeoutErr{}}
	if !isTimeoutError(netErr) {
		t.Error("net timeout error should be timeout")
	}
}

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

func TestSleepCtx_NormalSleep(t *testing.T) {
	start := time.Now()
	ctx := context.Background()
	sleepCtx(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Errorf("slept too short: %v", elapsed)
	}
}

func TestSleepCtx_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	start := time.Now()
	sleepCtx(ctx, 5*time.Second)
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("should return immediately on cancelled context, took %v", elapsed)
	}
}

func TestExtractText_SimpleText(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{Type: ItemText, TextItem: &TextItem{Text: "hello"}},
		},
	}
	if got := ExtractText(msg); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestExtractText_NoText(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{Type: ItemImage, ImageItem: &ImageItem{}},
		},
	}
	if got := ExtractText(msg); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractText_EmptyItemList(t *testing.T) {
	msg := &WeixinMessage{}
	if got := ExtractText(msg); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractText_VoiceToText(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{Type: ItemVoice, VoiceItem: &VoiceItem{Text: "语音转文字"}},
		},
	}
	if got := ExtractText(msg); got != "语音转文字" {
		t.Errorf("got %q, want %q", got, "语音转文字")
	}
}

func TestExtractText_VoiceWithoutText(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{Type: ItemVoice, VoiceItem: &VoiceItem{}},
		},
	}
	if got := ExtractText(msg); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractText_TextPrioritizedOverVoice(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{Type: ItemVoice, VoiceItem: &VoiceItem{Text: "voice"}},
			{Type: ItemText, TextItem: &TextItem{Text: "text"}},
		},
	}
	if got := ExtractText(msg); got != "text" {
		t.Errorf("got %q, want %q", got, "text")
	}
}

func TestExtractText_RefMsg_TextQuote(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{
				Type:     ItemText,
				TextItem: &TextItem{Text: "我的回复"},
				RefMsg: &RefMessage{
					Title: "张三",
					MessageItem: &MessageItem{
						Type:     ItemText,
						TextItem: &TextItem{Text: "原始消息"},
					},
				},
			},
		},
	}
	got := ExtractText(msg)
	want := "[引用: 张三 | 原始消息]\n我的回复"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractText_RefMsg_MediaQuote(t *testing.T) {
	msg := &WeixinMessage{
		ItemList: []MessageItem{
			{
				Type:     ItemText,
				TextItem: &TextItem{Text: "看这张图"},
				RefMsg: &RefMessage{
					Title: "张三",
					MessageItem: &MessageItem{
						Type:      ItemImage,
						ImageItem: &ImageItem{},
					},
				},
			},
		},
	}
	// Media refs should NOT be prepended as text quote
	got := ExtractText(msg)
	if got != "看这张图" {
		t.Errorf("got %q, want %q", got, "看这张图")
	}
}
