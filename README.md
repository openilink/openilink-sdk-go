# go-sdk

Go SDK for the [Weixin iLink Bot API](https://ilinkai.weixin.qq.com).

```
go get github.com/openilink/openilink-go-sdk
```

## Features

- QR code login with scan/expire callbacks
- Long-poll message monitoring with auto-retry and backoff
- Proactive push via cached context tokens
- Typing indicators, bot config, CDN upload URL
- Functional options pattern for client configuration
- `HTTPDoer` interface for custom transports and testing
- Structured error types (`APIError`, `HTTPError`)
- Zero external dependencies — stdlib only

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	ilink "github.com/openilink/openilink-go-sdk"
)

func main() {
	client := ilink.NewClient("")

	// Login
	result, err := client.LoginWithQR(context.Background(), &ilink.LoginCallbacks{
		OnQRCode:  func(url string) { fmt.Printf("Scan: %s\n", url) },
		OnScanned: func() { fmt.Println("Scanned, confirm on WeChat...") },
	})
	if err != nil || !result.Connected {
		log.Fatal("login failed")
	}
	fmt.Printf("Connected as %s\n", result.BotID)

	// Monitor & echo
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client.Monitor(ctx, func(msg ilink.WeixinMessage) {
		text := ilink.ExtractText(&msg)
		if text != "" {
			client.Push(ctx, msg.FromUserID, "echo: "+text)
		}
	}, &ilink.MonitorOptions{
		OnBufUpdate: func(buf string) {
			os.WriteFile("sync_buf.dat", []byte(buf), 0600)
		},
	})
}
```

## API

### Client

```go
// Create with default settings
client := ilink.NewClient(token)

// Or with options
client := ilink.NewClient(token,
    ilink.WithBaseURL("https://custom.endpoint.com"),
    ilink.WithHTTPClient(myHTTPClient),
    ilink.WithBotType("3"),
    ilink.WithVersion("1.0.0"),
)
```

### Login

```go
result, err := client.LoginWithQR(ctx, &ilink.LoginCallbacks{
    OnQRCode:  func(url string) { /* display QR code */ },
    OnScanned: func() { /* user scanned */ },
    OnExpired: func(attempt, max int) { /* QR expired, refreshing */ },
})
// result.Connected, result.BotID, result.BotToken, result.UserID
```

The client's token and base URL are updated automatically on successful login.

### Receive Messages

```go
err := client.Monitor(ctx, func(msg ilink.WeixinMessage) {
    // msg.FromUserID, msg.ContextToken, msg.ItemList
    text := ilink.ExtractText(&msg)
}, &ilink.MonitorOptions{
    InitialBuf:       savedBuf,          // resume from last position
    OnBufUpdate:      func(buf string) { /* persist buf */ },
    OnError:          func(err error) { /* log */ },
    OnSessionExpired: func() { /* re-login */ },
})
```

Monitor caches context tokens automatically for use with `Push`.

### Send Messages

```go
// Reply (requires contextToken from inbound message)
client.SendText(ctx, userID, "hello", contextToken)

// Proactive push (uses cached contextToken)
client.Push(ctx, userID, "scheduled notification")
```

### Other

```go
client.SendTyping(ctx, userID, ticket, ilink.Typing)
client.GetConfig(ctx, userID, contextToken)
client.GetUploadURL(ctx, &ilink.GetUploadURLReq{...})
```

## Error Handling

```go
import "errors"

var apiErr *ilink.APIError
if errors.As(err, &apiErr) {
    if apiErr.IsSessionExpired() {
        // re-login
    }
    fmt.Println(apiErr.ErrCode, apiErr.ErrMsg)
}

var httpErr *ilink.HTTPError
if errors.As(err, &httpErr) {
    fmt.Println(httpErr.StatusCode)
}

if errors.Is(err, ilink.ErrNoContextToken) {
    // user hasn't sent a message yet
}
```

## License

MIT
