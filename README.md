# openilink-sdk-go

微信 [iLink Bot API](https://ilinkai.weixin.qq.com) 的 Go SDK。

```
go get github.com/openilink/openilink-sdk-go
```

## 特性

- 扫码登录，支持扫码/过期回调
- 长轮询消息监听，自动重试与退避，动态超时
- 主动推送（自动缓存 contextToken）
- 发送图片、视频、文件，MIME 自动路由
- CDN 加密上传/下载（AES-128-ECB）
- 语音消息解码（可插拔 SILK 解码器 + WAV 封装）
- 输入状态指示器、Bot 配置
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
		OnQRCode:  func(img string) { fmt.Printf("请扫码:\n%s\n", img) },
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
    ilink.WithCDNBaseURL("https://custom.cdn.com/c2c"),
    ilink.WithHTTPClient(myHTTPClient),
    ilink.WithBotType("3"),
    ilink.WithVersion("1.0.2"),
    ilink.WithRouteTag("my-route-tag"),
    ilink.WithSILKDecoder(mySILKDecoder),
)
```

### 扫码登录

```go
result, err := client.LoginWithQR(ctx, &ilink.LoginCallbacks{
    OnQRCode:  func(imgContent string) { /* 展示二维码 */ },
    OnScanned: func() { /* 用户已扫码 */ },
    OnExpired: func(attempt, max int) { /* 二维码过期，正在刷新 */ },
})
// result.Connected, result.BotID, result.BotToken, result.UserID
```

登录成功后，客户端的 Token 和 BaseURL 会自动更新。

### 接收消息

```go
err := client.Monitor(ctx, func(msg ilink.WeixinMessage) {
    text := ilink.ExtractText(&msg) // 支持引用消息和语音转文字
    // msg.FromUserID, msg.ContextToken, msg.ItemList
}, &ilink.MonitorOptions{
    InitialBuf:       savedBuf,          // 从上次位置恢复
    OnBufUpdate:      func(buf string) { /* 持久化游标 */ },
    OnError:          func(err error) { /* 记录错误 */ },
    OnSessionExpired: func() { /* 重新登录 */ },
})
```

Monitor 会自动缓存每个用户的 contextToken，供 `Push` 使用。服务端返回的 `longpolling_timeout_ms` 会被自动采纳。

### 发送文本

```go
// 回复消息（需要入站消息的 contextToken）
client.SendText(ctx, userID, "你好", contextToken)

// 主动推送（使用缓存的 contextToken）
client.Push(ctx, userID, "这是一条定时通知")
```

### 发送媒体

```go
// 高级接口：自动识别 MIME 类型 → 上传 → 发送
data, _ := os.ReadFile("photo.jpg")
client.SendMediaFile(ctx, userID, contextToken, data, "photo.jpg", "看看这张图")

// 分步操作：上传 → 发送
uploaded, _ := client.UploadFile(ctx, data, userID, ilink.MediaImage)
client.SendImage(ctx, userID, contextToken, uploaded)
client.SendVideo(ctx, userID, contextToken, uploaded)
client.SendFileAttachment(ctx, userID, contextToken, "report.pdf", uploaded)
```

### 下载媒体

```go
for _, item := range msg.ItemList {
    switch item.Type {
    case ilink.ItemImage:
        data, _ := client.DownloadMedia(ctx, item.ImageItem.Media)

    case ilink.ItemVoice:
        // 需要配置 WithSILKDecoder
        wav, _ := client.DownloadVoice(ctx, item.VoiceItem)
    }
}
```

### 语音解码

SDK 通过可插拔的 `SILKDecoder` 支持语音消息解码，保持零外部依赖。以 [youthlin/silk](https://github.com/youthlin/silk) 为例：

```go
import "github.com/youthlin/silk"

client := ilink.NewClient(token, ilink.WithSILKDecoder(
    func(data []byte, sampleRate int) ([]byte, error) {
        return silk.Decode(bytes.NewReader(data),
            silk.WithSampleRate(sampleRate))
    },
))

wav, err := client.DownloadVoice(ctx, voiceItem) // 返回 WAV 文件字节
```

也可以单独使用 WAV 封装：

```go
wav := ilink.BuildWAV(pcmData, 24000, 1, 16) // sampleRate, channels, bitsPerSample
```

### 其他

```go
// 输入状态
client.SendTyping(ctx, userID, ticket, ilink.Typing)
client.SendTyping(ctx, userID, ticket, ilink.CancelTyping)

// Bot 配置（含 typing_ticket）
config, _ := client.GetConfig(ctx, userID, contextToken)

// 工具函数
ilink.IsMediaItem(&item)                  // 是否为媒体类型
ilink.MIMEFromFilename("photo.jpg")       // "image/jpeg"
ilink.ExtensionFromMIME("video/mp4")      // ".mp4"
ilink.IsImageMIME("image/png")            // true
ilink.IsVideoMIME("video/mp4")            // true
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

## 常量

```go
// 媒体类型（用于 UploadFile）
ilink.MediaImage   // 1
ilink.MediaVideo   // 2
ilink.MediaFile    // 3
ilink.MediaVoice   // 4

// 消息类型
ilink.MsgTypeUser  // 1 - 用户消息
ilink.MsgTypeBot   // 2 - Bot 消息

// 消息项类型
ilink.ItemText     // 1
ilink.ItemImage    // 2
ilink.ItemVoice    // 3
ilink.ItemFile     // 4
ilink.ItemVideo    // 5

// 消息状态
ilink.StateNew        // 0
ilink.StateGenerating // 1
ilink.StateFinish     // 2
```

## 许可证

MIT
