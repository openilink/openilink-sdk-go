package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	ilink "github.com/openilink/openilink-sdk-go"
)

func main() {
	client := ilink.NewClient("")

	// --- QR Login ---

	fmt.Println("Fetching QR code...")
	result, err := client.LoginWithQR(context.Background(), &ilink.LoginCallbacks{
		OnQRCode: func(imgContent string) {
			fmt.Printf("\nScan QR code with WeChat:\n%s\n\n", imgContent)
		},
		OnScanned: func() {
			fmt.Println("Scanned, confirm on WeChat...")
		},
		OnExpired: func(attempt, max int) {
			fmt.Printf("QR expired, refreshing (%d/%d)...\n", attempt, max)
		},
	})
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	if !result.Connected {
		log.Fatalf("Login incomplete: %s", result.Message)
	}
	fmt.Printf("Connected! BotID=%s UserID=%s\n\n", result.BotID, result.UserID)

	// --- Typing ticket cache (per-user) ---

	typingTickets := map[string]string{} // userID → ticket

	fetchTypingTicket := func(ctx context.Context, userID, contextToken string) string {
		if t, ok := typingTickets[userID]; ok {
			return t
		}
		cfg, err := client.GetConfig(ctx, userID, contextToken)
		if err != nil {
			log.Printf("GetConfig failed: %v", err)
			return ""
		}
		typingTickets[userID] = cfg.TypingTicket
		return cfg.TypingTicket
	}

	// --- Monitor ---

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println("Listening for messages... (Ctrl+C to quit)")
	err = client.Monitor(ctx, func(msg ilink.WeixinMessage) {
		from := msg.FromUserID

		// Typing indicator
		if ticket := fetchTypingTicket(ctx, from, msg.ContextToken); ticket != "" {
			_ = client.SendTyping(ctx, from, ticket, ilink.Typing)
			defer client.SendTyping(ctx, from, ticket, ilink.CancelTyping)
		}

		// Echo images back
		for _, item := range msg.ItemList {
			if item.Type == ilink.ItemImage && item.ImageItem != nil && item.ImageItem.Media != nil {
				data, err := client.DownloadMedia(ctx, item.ImageItem.Media)
				if err != nil {
					log.Printf("Download image failed: %v", err)
					continue
				}
				if err := client.SendMediaFile(ctx, from, msg.ContextToken, data, "echo.png", ""); err != nil {
					log.Printf("Send image failed: %v", err)
				}
				return
			}
		}

		// Echo text
		text := ilink.ExtractText(&msg)
		if text == "" {
			return
		}
		fmt.Printf("[%s] %s\n", from, text)

		if _, err := client.Push(ctx, from, "echo: "+text); err != nil {
			log.Printf("Reply failed: %v", err)
		}
	}, &ilink.MonitorOptions{
		InitialBuf: loadBuf(),
		OnBufUpdate: func(buf string) {
			_ = os.WriteFile(bufPath(), []byte(buf), 0600)
		},
		OnError: func(err error) {
			log.Printf("Error: %v", err)
		},
		OnSessionExpired: func() {
			log.Println("Session expired, will retry in 1h")
		},
	})

	if err != nil && err != context.Canceled {
		log.Fatalf("Monitor error: %v", err)
	}
}

func bufPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "echo-bot-sync.dat")
}

func loadBuf() string {
	data, err := os.ReadFile(bufPath())
	if err != nil {
		return ""
	}
	return string(data)
}
