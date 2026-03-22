// Package ilink provides a Go client for the Weixin iLink Bot API.
//
// The SDK covers the full lifecycle: QR-code login, long-poll message
// monitoring, text/media sending, typing indicators, and proactive push.
//
// Basic usage:
//
//	client := ilink.NewClient("")
//	result, _ := client.LoginWithQR(ctx, nil)
//	client.Monitor(ctx, func(msg ilink.WeixinMessage) {
//	    text := ilink.ExtractText(&msg)
//	    client.Push(ctx, msg.FromUserID, "echo: "+text)
//	}, nil)
package ilink
