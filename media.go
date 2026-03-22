package ilink

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
)

// IsMediaItem reports whether a MessageItem is a media type
// (image, video, file, or voice).
func IsMediaItem(item *MessageItem) bool {
	switch item.Type {
	case ItemImage, ItemVideo, ItemFile, ItemVoice:
		return true
	}
	return false
}

// mediaAESKey converts a hex-encoded AES key to the base64(hex) format
// used in CDNMedia.aes_key fields.
func mediaAESKey(hexKey string) string {
	return base64.StdEncoding.EncodeToString([]byte(hexKey))
}

// SendImage sends an image message to a user.
// Use [Client.UploadFile] with [MediaImage] to obtain the [UploadResult] first.
func (c *Client) SendImage(ctx context.Context, to, contextToken string, uploaded *UploadResult) (string, error) {
	clientID := generateClientID()
	msg := &SendMessageReq{
		Msg: &WeixinMessage{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MsgTypeBot,
			MessageState: StateFinish,
			ContextToken: contextToken,
			ItemList: []MessageItem{
				{
					Type: ItemImage,
					ImageItem: &ImageItem{
						Media: &CDNMedia{
							EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
							AESKey:            mediaAESKey(uploaded.AESKey),
							EncryptType:       1,
						},
						MidSize: uploaded.CiphertextSize,
					},
				},
			},
		},
	}
	if err := c.SendMessage(ctx, msg); err != nil {
		return "", err
	}
	return clientID, nil
}

// SendVideo sends a video message to a user.
// Use [Client.UploadFile] with [MediaVideo] to obtain the [UploadResult] first.
func (c *Client) SendVideo(ctx context.Context, to, contextToken string, uploaded *UploadResult) (string, error) {
	clientID := generateClientID()
	msg := &SendMessageReq{
		Msg: &WeixinMessage{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MsgTypeBot,
			MessageState: StateFinish,
			ContextToken: contextToken,
			ItemList: []MessageItem{
				{
					Type: ItemVideo,
					VideoItem: &VideoItem{
						Media: &CDNMedia{
							EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
							AESKey:            mediaAESKey(uploaded.AESKey),
							EncryptType:       1,
						},
						VideoSize: uploaded.CiphertextSize,
					},
				},
			},
		},
	}
	if err := c.SendMessage(ctx, msg); err != nil {
		return "", err
	}
	return clientID, nil
}

// SendFileAttachment sends a file attachment message to a user.
// Use [Client.UploadFile] with [MediaFile] to obtain the [UploadResult] first.
func (c *Client) SendFileAttachment(ctx context.Context, to, contextToken, fileName string, uploaded *UploadResult) (string, error) {
	clientID := generateClientID()
	msg := &SendMessageReq{
		Msg: &WeixinMessage{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MsgTypeBot,
			MessageState: StateFinish,
			ContextToken: contextToken,
			ItemList: []MessageItem{
				{
					Type: ItemFile,
					FileItem: &FileItem{
						Media: &CDNMedia{
							EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
							AESKey:            mediaAESKey(uploaded.AESKey),
							EncryptType:       1,
						},
						FileName: fileName,
						Len:      fmt.Sprintf("%d", uploaded.FileSize),
					},
				},
			},
		},
	}
	if err := c.SendMessage(ctx, msg); err != nil {
		return "", err
	}
	return clientID, nil
}

// SendMediaFile is a high-level helper that uploads a file to CDN and sends it
// as the appropriate media type (image, video, or file attachment) based on the
// filename's MIME type. An optional caption text is sent as a separate text
// message before the media (Weixin requires one item per message).
func (c *Client) SendMediaFile(ctx context.Context, to, contextToken string, data []byte, fileName string, caption string) error {
	mime := MIMEFromFilename(fileName)

	var mediaType UploadMediaType
	switch {
	case IsVideoMIME(mime):
		mediaType = MediaVideo
	case IsImageMIME(mime):
		mediaType = MediaImage
	default:
		mediaType = MediaFile
	}

	uploaded, err := c.UploadFile(ctx, data, to, mediaType)
	if err != nil {
		return fmt.Errorf("ilink: upload media: %w", err)
	}

	// Send caption as a separate text message first (if provided).
	if caption != "" {
		if _, err := c.SendText(ctx, to, caption, contextToken); err != nil {
			return fmt.Errorf("ilink: send caption: %w", err)
		}
	}

	// Send the media message.
	switch {
	case IsVideoMIME(mime):
		_, err = c.SendVideo(ctx, to, contextToken, uploaded)
	case IsImageMIME(mime):
		_, err = c.SendImage(ctx, to, contextToken, uploaded)
	default:
		_, err = c.SendFileAttachment(ctx, to, contextToken, filepath.Base(fileName), uploaded)
	}
	if err != nil {
		return fmt.Errorf("ilink: send media: %w", err)
	}
	return nil
}
