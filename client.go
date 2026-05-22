package ilink

import (
	"os"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the production iLink Bot API endpoint.
	DefaultBaseURL = "https://ilinkai.weixin.qq.com"
	// DefaultCDNBaseURL is the production CDN endpoint for media.
	DefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
	// DefaultBotType is the default bot_type parameter for QR login.
	DefaultBotType = "3"

	// iLinkAppID matches the ilink_appid field from the upstream openclaw-weixin package.json.
	iLinkAppID = "bot"
	// iLinkChannelVersion tracks the upstream openclaw-weixin version we are compatible with.
	iLinkChannelVersion = "2.4.3"

	defaultLongPollTimeout = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second
	defaultConfigTimeout   = 10 * time.Second
)

// HTTPDoer executes HTTP requests. *http.Client satisfies this interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Option configures a [Client].
type Option func(*Client)

// WithBaseURL overrides the API base URL.
// This also sets the QR login base URL; in production the QR login
// endpoint is always [DefaultBaseURL] and [Client.LoginWithQR] uses
// the value set at construction time, not whatever the server returns.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url; c.loginBaseURL = url }
}

// WithCDNBaseURL overrides the CDN base URL for media uploads.
func WithCDNBaseURL(url string) Option { return func(c *Client) { c.cdnBaseURL = url } }

// WithHTTPClient sets the HTTP client used for all requests.
func WithHTTPClient(doer HTTPDoer) Option { return func(c *Client) { c.httpClient = doer } }

// WithBotType sets the bot_type parameter used during QR login.
func WithBotType(t string) Option { return func(c *Client) { c.botType = t } }

// WithVersion sets the channel_version sent in base_info.
func WithVersion(v string) Option { return func(c *Client) { c.version = v } }

// WithRouteTag sets the SKRouteTag header sent with every request.
func WithRouteTag(tag string) Option { return func(c *Client) { c.routeTag = tag } }

// Client communicates with the Weixin iLink Bot API.
type Client struct {
	baseURL      string
	cdnBaseURL   string
	loginBaseURL string // fixed base URL for QR login; defaults to DefaultBaseURL
	token        string
	botType      string
	version      string
	routeTag     string
	httpClient   HTTPDoer
	silkDecoder  SILKDecoder

	contextTokens sync.Map // map[userID]contextToken
}

// NewClient creates a new iLink API client.
//
// Pass an empty token for login-only usage; the token is set automatically
// after a successful [Client.LoginWithQR].
//
//	client := ilink.NewClient("",
//	    ilink.WithBaseURL("https://custom.endpoint.com"),
//	)
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:      DefaultBaseURL,
		cdnBaseURL:   DefaultCDNBaseURL,
		loginBaseURL: DefaultBaseURL,
		token:        token,
		botType:      DefaultBotType,
		version:      iLinkChannelVersion,
		httpClient:   &http.Client{},
	}
	// Allow ILINK_CHANNEL_VERSION env var to override the default channel version.
	// This is useful for testing against a newer protocol version without
	// recompiling, or for hot-fixing compatibility when the upstream plugin advances.
	if v := os.Getenv("ILINK_CHANNEL_VERSION"); v != "" {
		c.version = v
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetToken updates the authentication token.
func (c *Client) SetToken(token string) { c.token = token }

// SetBaseURL updates the API base URL.
func (c *Client) SetBaseURL(url string) { c.baseURL = url }

// Token returns the current authentication token.
func (c *Client) Token() string { return c.token }

// BaseURL returns the current API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) buildBaseInfo() *BaseInfo {
	return &BaseInfo{ChannelVersion: c.version}
}

// commonHeaders returns headers shared by all requests (GET and POST).
func (c *Client) commonHeaders() http.Header {
	h := http.Header{}
	h.Set("iLink-App-Id", iLinkAppID)
	h.Set("iLink-App-ClientVersion", fmt.Sprintf("%d", encodeClientVersion(c.version)))
	if c.routeTag != "" {
		h.Set("SKRouteTag", c.routeTag)
	}
	return h
}

func (c *Client) buildHeaders(body []byte) http.Header {
	h := c.commonHeaders()
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	h.Set("X-WECHAT-UIN", randomWechatUIN())
	if t := strings.TrimSpace(c.token); t != "" {
		h.Set("Authorization", "Bearer "+t)
	}
	return h
}

func (c *Client) doPost(ctx context.Context, endpoint string, body any, timeout time.Duration) (*APIResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ilink: marshal request: %w", err)
	}

	base := ensureTrailingSlash(c.baseURL)
	u, err := url.JoinPath(base, endpoint)
	if err != nil {
		return nil, fmt.Errorf("ilink: build url: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ilink: new request: %w", err)
	}
	req.Header = c.buildHeaders(data)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ilink: read response: %w", err)
	}
	apiResp := &APIResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: respBody}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiResp, &HTTPError{StatusCode: resp.StatusCode, Body: respBody}
	}
	return apiResp, nil
}

func (c *Client) doGet(ctx context.Context, rawURL string, extraHeaders map[string]string, timeout time.Duration) (*APIResponse, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ilink: new request: %w", err)
	}
	req.Header = c.commonHeaders()
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ilink: read response: %w", err)
	}
	apiResp := &APIResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiResp, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}
	return apiResp, nil
}

// --- API methods ---

// GetUpdates performs a long-poll request to receive new messages.
// Returns an empty response (not an error) on client-side timeout.
// Use timeoutMs to override the poll timeout; zero uses the default (35 s).
func (c *Client) GetUpdates(ctx context.Context, getUpdatesBuf string, timeoutMs ...int64) (*GetUpdatesResp, error) {
	reqBody := GetUpdatesReq{
		GetUpdatesBuf: getUpdatesBuf,
		BaseInfo:      c.buildBaseInfo(),
	}
	timeout := defaultLongPollTimeout
	if len(timeoutMs) > 0 && timeoutMs[0] > 0 {
		timeout = time.Duration(timeoutMs[0]) * time.Millisecond
	}

	apiResp, err := c.doPost(ctx, "ilink/bot/getupdates", reqBody, timeout)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if isTimeoutError(err) {
			return &GetUpdatesResp{Ret: 0, Msgs: []WeixinMessage{}, GetUpdatesBuf: getUpdatesBuf}, nil
		}
		return nil, err
	}

	var resp GetUpdatesResp
	if err := json.Unmarshal(apiResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("ilink: unmarshal getUpdates: %w", err)
	}
	resp.setRawResponse(apiResp)
	return &resp, nil
}

// SendMessage sends a raw message request.
func (c *Client) SendMessage(ctx context.Context, msg *SendMessageReq) error {
	msg.BaseInfo = c.buildBaseInfo()
	_, err := c.doPost(ctx, "ilink/bot/sendmessage", msg, defaultAPITimeout)
	return err
}

// SendText sends a plain text message to a user.
// contextToken must come from the inbound message's ContextToken field.
func (c *Client) SendText(ctx context.Context, to, text, contextToken string) (string, error) {
	clientID := generateClientID()
	msg := &SendMessageReq{
		Msg: &WeixinMessage{
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MsgTypeBot,
			MessageState: StateFinish,
			ContextToken: contextToken,
			ItemList: []MessageItem{
				{Type: ItemText, TextItem: &TextItem{Text: text}},
			},
		},
	}
	if err := c.SendMessage(ctx, msg); err != nil {
		return "", err
	}
	return clientID, nil
}

// GetConfig fetches bot config (includes typing_ticket) for a given user.
func (c *Client) GetConfig(ctx context.Context, userID, contextToken string) (*GetConfigResp, error) {
	reqBody := GetConfigReq{
		ILinkUserID:  userID,
		ContextToken: contextToken,
		BaseInfo:     c.buildBaseInfo(),
	}
	apiResp, err := c.doPost(ctx, "ilink/bot/getconfig", reqBody, defaultConfigTimeout)
	if err != nil {
		return nil, err
	}
	var resp GetConfigResp
	if err := json.Unmarshal(apiResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("ilink: unmarshal getConfig: %w", err)
	}
	resp.setRawResponse(apiResp)
	return &resp, nil
}

// SendTyping sends or cancels a typing indicator.
func (c *Client) SendTyping(ctx context.Context, userID, typingTicket string, status TypingStatus) error {
	reqBody := SendTypingReq{
		ILinkUserID:  userID,
		TypingTicket: typingTicket,
		Status:       status,
		BaseInfo:     c.buildBaseInfo(),
	}
	_, err := c.doPost(ctx, "ilink/bot/sendtyping", reqBody, defaultConfigTimeout)
	return err
}

// GetUploadURL requests a pre-signed CDN upload URL.
func (c *Client) GetUploadURL(ctx context.Context, req *GetUploadURLReq) (*GetUploadURLResp, error) {
	req.BaseInfo = c.buildBaseInfo()
	apiResp, err := c.doPost(ctx, "ilink/bot/getuploadurl", req, defaultAPITimeout)
	if err != nil {
		return nil, err
	}
	var resp GetUploadURLResp
	if err := json.Unmarshal(apiResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("ilink: unmarshal getUploadUrl: %w", err)
	}
	resp.setRawResponse(apiResp)
	return &resp, nil
}

// --- Context token cache ---

// SetContextToken caches a context token for a user.
// Called automatically by [Client.Monitor]; can also be called manually.
func (c *Client) SetContextToken(userID, token string) {
	c.contextTokens.Store(userID, token)
}

// GetContextToken returns the cached context token for a user, if any.
func (c *Client) GetContextToken(userID string) (string, bool) {
	v, ok := c.contextTokens.Load(userID)
	if !ok {
		return "", false
	}
	return v.(string), true
}

// Push sends a proactive text message using a cached context token.
// The target user must have previously sent a message so that a context
// token is available. Returns [ErrNoContextToken] otherwise.
func (c *Client) Push(ctx context.Context, to, text string) (string, error) {
	token, ok := c.GetContextToken(to)
	if !ok {
		return "", ErrNoContextToken
	}
	return c.SendText(ctx, to, text, token)
}
