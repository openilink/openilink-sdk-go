package ilink

import "net/http"

// APIResponse holds the raw HTTP response metadata from an API call.
// Access it via the LastResponse() method on any parsed response type.
type APIResponse struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Header contains the HTTP response headers.
	Header http.Header
	// Body is the raw response body bytes.
	Body []byte
}

// rawResponse is embedded in parsed response types to carry the raw HTTP response.
type rawResponse struct {
	apiResponse *APIResponse
}

// RawResponse returns the underlying HTTP response (status, headers, body)
// from the API call that produced this value.
// Returns nil for synthetic responses (e.g. long-poll timeout).
func (r *rawResponse) RawResponse() *APIResponse { return r.apiResponse }

func (r *rawResponse) setRawResponse(resp *APIResponse) { r.apiResponse = resp }

// BaseInfo is attached to every outgoing API request.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// UploadMediaType identifies the type of media being uploaded.
type UploadMediaType int

const (
	MediaImage UploadMediaType = 1
	MediaVideo UploadMediaType = 2
	MediaFile  UploadMediaType = 3
	MediaVoice UploadMediaType = 4
)

// GetUploadURLReq requests a pre-signed CDN upload URL.
type GetUploadURLReq struct {
	FileKey       string    `json:"filekey,omitempty"`
	MediaType     int       `json:"media_type,omitempty"`
	ToUserID      string    `json:"to_user_id,omitempty"`
	RawSize       int64     `json:"rawsize,omitempty"`
	RawFileMD5    string    `json:"rawfilemd5,omitempty"`
	FileSize      int64     `json:"filesize,omitempty"`
	ThumbRawSize  int64     `json:"thumb_rawsize,omitempty"`
	ThumbRawMD5   string    `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize int64     `json:"thumb_filesize,omitempty"`
	NoNeedThumb   bool      `json:"no_need_thumb,omitempty"`
	AESKey        string    `json:"aeskey,omitempty"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetUploadURLResp contains pre-signed upload parameters from the CDN.
type GetUploadURLResp struct {
	rawResponse
	Ret              int    `json:"ret,omitempty"`
	ErrMsg           string `json:"errmsg,omitempty"`
	UploadParam      string `json:"upload_param,omitempty"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
}

// MessageType distinguishes user messages from bot messages.
type MessageType int

const (
	MsgTypeNone MessageType = 0
	MsgTypeUser MessageType = 1
	MsgTypeBot  MessageType = 2
)

// MessageItemType identifies the content type of a message item.
type MessageItemType int

const (
	ItemNone  MessageItemType = 0
	ItemText  MessageItemType = 1
	ItemImage MessageItemType = 2
	ItemVoice MessageItemType = 3
	ItemFile  MessageItemType = 4
	ItemVideo MessageItemType = 5
)

// MessageState represents the generation state of a message.
type MessageState int

const (
	StateNew        MessageState = 0
	StateGenerating MessageState = 1
	StateFinish     MessageState = 2
)

// TypingStatus represents the typing indicator state.
type TypingStatus int

const (
	Typing       TypingStatus = 1
	CancelTyping TypingStatus = 2
)

// EncryptType identifies the CDN encryption algorithm.
const (
	// EncryptAES128ECB is AES-128-ECB encryption, the only type currently used.
	EncryptAES128ECB = 1
)

// CDNMedia is a reference to encrypted media on the Weixin CDN.
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

// TextItem holds the text content of a message.
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// ImageItem holds image-related fields for a message.
type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	AESKey      string    `json:"aeskey,omitempty"`
	URL         string    `json:"url,omitempty"`
	MidSize     int64     `json:"mid_size,omitempty"`
	ThumbSize   int64     `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	HDSize      int64     `json:"hd_size,omitempty"`
}

// VoiceItem holds voice message fields.
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int      `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	PlayTime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}

// FileItem holds file attachment fields.
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

// VideoItem holds video message fields.
type VideoItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	VideoSize   int64     `json:"video_size,omitempty"`
	PlayLength  int       `json:"play_length,omitempty"`
	VideoMD5    string    `json:"video_md5,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int64     `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
}

// RefMessage holds a quoted/referenced message.
type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

// MessageItem is a single content item within a message (text, image, etc.).
type MessageItem struct {
	Type         MessageItemType `json:"type,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64          `json:"update_time_ms,omitempty"`
	IsCompleted  bool           `json:"is_completed,omitempty"`
	MsgID        string         `json:"msg_id,omitempty"`
	RefMsg       *RefMessage    `json:"ref_msg,omitempty"`
	TextItem     *TextItem      `json:"text_item,omitempty"`
	ImageItem    *ImageItem     `json:"image_item,omitempty"`
	VoiceItem    *VoiceItem     `json:"voice_item,omitempty"`
	FileItem     *FileItem      `json:"file_item,omitempty"`
	VideoItem    *VideoItem     `json:"video_item,omitempty"`
}

// WeixinMessage is the unified message structure from getUpdates.
type WeixinMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64         `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64         `json:"delete_time_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  MessageType   `json:"message_type,omitempty"`
	MessageState MessageState  `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

// GetUpdatesReq is the request body for the getUpdates long-poll endpoint.
type GetUpdatesReq struct {
	GetUpdatesBuf string    `json:"get_updates_buf"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// GetUpdatesResp is the response from the getUpdates endpoint.
type GetUpdatesResp struct {
	rawResponse
	Ret                  int             `json:"ret,omitempty"`
	ErrCode              int             `json:"errcode,omitempty"`
	ErrMsg               string          `json:"errmsg,omitempty"`
	Msgs                 []WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf        string          `json:"get_updates_buf,omitempty"`
	SyncBuf              string          `json:"sync_buf,omitempty"`
	LongPollingTimeoutMs int64           `json:"longpolling_timeout_ms,omitempty"`
}

// SendMessageReq wraps a single outbound message.
type SendMessageReq struct {
	Msg      *WeixinMessage `json:"msg,omitempty"`
	BaseInfo *BaseInfo      `json:"base_info,omitempty"`
}

// SendTypingReq sends or cancels a typing indicator.
type SendTypingReq struct {
	ILinkUserID  string       `json:"ilink_user_id,omitempty"`
	TypingTicket string       `json:"typing_ticket,omitempty"`
	Status       TypingStatus `json:"status,omitempty"`
	BaseInfo     *BaseInfo    `json:"base_info,omitempty"`
}

// GetConfigReq requests bot config for a given user.
type GetConfigReq struct {
	ILinkUserID  string    `json:"ilink_user_id,omitempty"`
	ContextToken string    `json:"context_token,omitempty"`
	BaseInfo     *BaseInfo `json:"base_info,omitempty"`
}

// GetConfigResp contains bot config including typing_ticket.
type GetConfigResp struct {
	rawResponse
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// QRCodeResponse is returned when requesting a login QR code.
type QRCodeResponse struct {
	rawResponse
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

// QRStatusResponse is returned when polling QR code scan status.
type QRStatusResponse struct {
	rawResponse
	Status      string `json:"status"` // wait, scaned, confirmed, expired
	BotToken    string `json:"bot_token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
}

// LoginResult holds the outcome of a QR login flow.
type LoginResult struct {
	Connected bool
	BotToken  string
	BotID     string
	BaseURL   string
	UserID    string
	Message   string
}
