package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	ilink "github.com/openilink/openilink-sdk-go"
)

func main() {
	client := ilink.NewClient("")

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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println("Listening for messages... (Ctrl+C to quit)")
	err = client.Monitor(ctx, func(msg ilink.WeixinMessage) {
		text := ilink.ExtractText(&msg)
		if text == "" {
			return
		}
		fmt.Printf("[%s] %s\n", msg.FromUserID, text)

		if _, err := client.Push(ctx, msg.FromUserID, "echo: "+text); err != nil {
			log.Printf("Reply failed: %v", err)
		}
	}, &ilink.MonitorOptions{
		InitialBuf: loadBuf(),
		OnBufUpdate: func(buf string) {
			_ = os.WriteFile("sync_buf.dat", []byte(buf), 0600)
		},
		OnError: func(err error) {
			log.Printf("Error: %v", err)
		},
	})

	if err != nil && err != context.Canceled {
		log.Fatalf("Monitor error: %v", err)
	}
}

func loadBuf() string {
	data, err := os.ReadFile("sync_buf.dat")
	if err != nil {
		return ""
	}
	return string(data)
}
