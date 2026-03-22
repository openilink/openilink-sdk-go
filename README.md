# openilink-sdk-go

微信 [iLink Bot API](https://ilinkai.weixin.qq.com) 的 Go SDK。

```
go get github.com/openilink/openilink-sdk-go
```

## 特性

- 扫码登录，支持扫码/过期回调
- 长轮询消息监听，自动重试与退避
- 主动推送（自动缓存 contextToken）
- 输入状态指示器、Bot 配置、CDN 上传
- Functional Options 配置模式
- `HTTPDoer` 接口，方便自定义传输层和测试
- 结构化错误类型（`APIError`、`HTTPError`）
- 零外部依赖，仅使用标准库

## 快速开始

```go
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

	// 扫码登录
	result, err := client.LoginWithQR(context.Background(), &ilink.LoginCallbacks{
		OnQRCode:  func(url string) { fmt.Printf("请扫码: %s\n", url) },
		OnScanned: func() { fmt.Println("已扫码，请在微信上确认...") },
	})
	if err != nil || !result.Connected {
		log.Fatal("登录失败")
	}
	fmt.Printf("已连接 BotID=%s\n", result.BotID)

	// 监听消息 & 自动回复
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client.Monitor(ctx, func(msg ilink.WeixinMessage) {
		text := ilink.ExtractText(&msg)
		if text != "" {
			client.Push(ctx, msg.FromUserID, "收到: "+text)
		}
	}, &ilink.MonitorOptions{
		OnBufUpdate: func(buf string) {
			os.WriteFile("sync_buf.dat", []byte(buf), 0600)
		},
	})
}
```

## API

### 创建客户端

```go
// 默认配置
client := ilink.NewClient(token)

// 自定义配置
client := ilink.NewClient(token,
    ilink.WithBaseURL("https://custom.endpoint.com"),
    ilink.WithHTTPClient(myHTTPClient),
    ilink.WithBotType("3"),
    ilink.WithVersion("1.0.0"),
)
```

### 扫码登录

```go
result, err := client.LoginWithQR(ctx, &ilink.LoginCallbacks{
    OnQRCode:  func(url string) { /* 展示二维码 */ },
    OnScanned: func() { /* 用户已扫码 */ },
    OnExpired: func(attempt, max int) { /* 二维码过期，正在刷新 */ },
})
// result.Connected, result.BotID, result.BotToken, result.UserID
```

登录成功后，客户端的 Token 和 BaseURL 会自动更新。

### 接收消息

```go
err := client.Monitor(ctx, func(msg ilink.WeixinMessage) {
    // msg.FromUserID, msg.ContextToken, msg.ItemList
    text := ilink.ExtractText(&msg)
}, &ilink.MonitorOptions{
    InitialBuf:       savedBuf,          // 从上次位置恢复
    OnBufUpdate:      func(buf string) { /* 持久化游标 */ },
    OnError:          func(err error) { /* 记录错误 */ },
    OnSessionExpired: func() { /* 重新登录 */ },
})
```

Monitor 会自动缓存每个用户的 contextToken，供 `Push` 使用。

### 发送消息

```go
// 回复消息（需要入站消息的 contextToken）
client.SendText(ctx, userID, "你好", contextToken)

// 主动推送（使用缓存的 contextToken）
client.Push(ctx, userID, "这是一条定时通知")
```

### 其他

```go
client.SendTyping(ctx, userID, ticket, ilink.Typing)
client.GetConfig(ctx, userID, contextToken)
client.GetUploadURL(ctx, &ilink.GetUploadURLReq{...})
```

## 错误处理

```go
import "errors"

var apiErr *ilink.APIError
if errors.As(err, &apiErr) {
    if apiErr.IsSessionExpired() {
        // 需要重新登录
    }
    fmt.Println(apiErr.ErrCode, apiErr.ErrMsg)
}

var httpErr *ilink.HTTPError
if errors.As(err, &httpErr) {
    fmt.Println(httpErr.StatusCode)
}

if errors.Is(err, ilink.ErrNoContextToken) {
    // 该用户尚未发送过消息，无法主动推送
}
```

## 许可证

MIT
