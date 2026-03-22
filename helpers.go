package ilink

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

func ensureTrailingSlash(u string) string {
	if strings.HasSuffix(u, "/") {
		return u
	}
	return u + "/"
}

func randomWechatUIN() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	n := binary.BigEndian.Uint32(b)
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", n)))
}

func generateClientID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sdk-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// isTimeoutError reports whether err is a timeout (deadline exceeded)
// as opposed to a real network failure.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// ExtractText returns the text body from a message's item list.
//
// It handles three cases in priority order:
//  1. A text item with an optional quoted/referenced message prefix
//  2. A voice item with speech-to-text transcription
//  3. Empty string if neither is found
func ExtractText(msg *WeixinMessage) string {
	for _, item := range msg.ItemList {
		if item.Type == ItemText && item.TextItem != nil {
			text := item.TextItem.Text
			// Prepend quoted message context if present and not a media ref
			if ref := item.RefMsg; ref != nil && ref.MessageItem != nil && !IsMediaItem(ref.MessageItem) {
				refBody := ""
				if ref.MessageItem.TextItem != nil {
					refBody = ref.MessageItem.TextItem.Text
				}
				title := ref.Title
				if title != "" || refBody != "" {
					text = "[引用: " + title + " | " + refBody + "]\n" + text
				}
			}
			return text
		}
	}
	// Fallback: voice-to-text transcription
	for _, item := range msg.ItemList {
		if item.Type == ItemVoice && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}
	return ""
}
