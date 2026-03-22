package ilink

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// ExtractText returns the first text body from a message's item list.
// Returns an empty string if no text item is found.
func ExtractText(msg *WeixinMessage) string {
	for _, item := range msg.ItemList {
		if item.Type == ItemText && item.TextItem != nil {
			return item.TextItem.Text
		}
	}
	return ""
}
