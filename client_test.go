package ilink

import (
	"os"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("mytoken")

	if c.Token() != "mytoken" {
		t.Errorf("token = %q, want %q", c.Token(), "mytoken")
	}
	if c.BaseURL() != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.BaseURL(), DefaultBaseURL)
	}
	if c.cdnBaseURL != DefaultCDNBaseURL {
		t.Errorf("cdnBaseURL = %q, want %q", c.cdnBaseURL, DefaultCDNBaseURL)
	}
	if c.botType != DefaultBotType {
		t.Errorf("botType = %q, want %q", c.botType, DefaultBotType)
	}
	if c.version != iLinkChannelVersion {
		t.Errorf("version = %q, want %q", c.version, iLinkChannelVersion)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	c := NewClient("tok",
		WithBaseURL("https://custom.com"),
		WithCDNBaseURL("https://cdn.custom.com"),
		WithBotType("5"),
		WithVersion("2.0.0"),
		WithRouteTag("route-1"),
	)

	if c.BaseURL() != "https://custom.com" {
		t.Errorf("baseURL = %q", c.BaseURL())
	}
	if c.cdnBaseURL != "https://cdn.custom.com" {
		t.Errorf("cdnBaseURL = %q", c.cdnBaseURL)
	}
	if c.botType != "5" {
		t.Errorf("botType = %q", c.botType)
	}
	if c.version != "2.0.0" {
		t.Errorf("version = %q", c.version)
	}
	if c.routeTag != "route-1" {
		t.Errorf("routeTag = %q", c.routeTag)
	}
}

func TestClient_SetToken(t *testing.T) {
	c := NewClient("")
	c.SetToken("new-token")
	if c.Token() != "new-token" {
		t.Errorf("token = %q, want %q", c.Token(), "new-token")
	}
}

func TestClient_ContextTokenCache(t *testing.T) {
	c := NewClient("")

	_, ok := c.GetContextToken("user1")
	if ok {
		t.Error("should not have token before set")
	}

	c.SetContextToken("user1", "tok1")
	tok, ok := c.GetContextToken("user1")
	if !ok || tok != "tok1" {
		t.Errorf("got (%q, %v), want (%q, true)", tok, ok, "tok1")
	}

	// Overwrite
	c.SetContextToken("user1", "tok2")
	tok, ok = c.GetContextToken("user1")
	if !ok || tok != "tok2" {
		t.Errorf("got (%q, %v), want (%q, true)", tok, ok, "tok2")
	}
}

func TestClient_Push_NoToken(t *testing.T) {
	c := NewClient("")
	_, err := c.Push(context.Background(), "user1", "hello")
	if !errors.Is(err, ErrNoContextToken) {
		t.Errorf("got %v, want ErrNoContextToken", err)
	}
}

func TestBuildHeaders(t *testing.T) {
	c := NewClient("  my-token  ", WithRouteTag("route-x"))
	h := c.buildHeaders([]byte(`{"test":true}`))

	if h.Get("Content-Type") != "application/json" {
		t.Error("missing Content-Type")
	}
	if h.Get("AuthorizationType") != "ilink_bot_token" {
		t.Error("missing AuthorizationType")
	}
	if h.Get("Authorization") != "Bearer my-token" {
		t.Errorf("Authorization = %q, want trimmed token", h.Get("Authorization"))
	}
	if h.Get("SKRouteTag") != "route-x" {
		t.Errorf("SKRouteTag = %q", h.Get("SKRouteTag"))
	}
	if h.Get("X-WECHAT-UIN") == "" {
		t.Error("missing X-WECHAT-UIN")
	}
	if h.Get("Content-Length") != "13" {
		t.Errorf("Content-Length = %q, want 13", h.Get("Content-Length"))
	}
}

func TestBuildHeaders_EmptyToken(t *testing.T) {
	c := NewClient("")
	h := c.buildHeaders([]byte(`{}`))
	if h.Get("Authorization") != "" {
		t.Errorf("should not set Authorization for empty token, got %q", h.Get("Authorization"))
	}
}

func TestBuildHeaders_NoRouteTag(t *testing.T) {
	c := NewClient("tok")
	h := c.buildHeaders([]byte(`{}`))
	if h.Get("SKRouteTag") != "" {
		t.Errorf("should not set SKRouteTag when empty, got %q", h.Get("SKRouteTag"))
	}
}

func TestBuildBaseInfo(t *testing.T) {
	c := NewClient("", WithVersion("3.0.0"))
	bi := c.buildBaseInfo()
	if bi.ChannelVersion != "3.0.0" {
		t.Errorf("channel_version = %q, want 3.0.0", bi.ChannelVersion)
	}
}

// --- HTTP integration tests with httptest ---

func TestGetUpdates_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/ilink/bot/getupdates") {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(GetUpdatesResp{
			Ret:                  0,
			Msgs:                 []WeixinMessage{{MessageID: 1}},
			GetUpdatesBuf:        "cursor-1",
			LongPollingTimeoutMs: 40000,
		})
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	resp, err := c.GetUpdates(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Ret != 0 {
		t.Errorf("ret = %d", resp.Ret)
	}
	if len(resp.Msgs) != 1 {
		t.Errorf("msgs count = %d, want 1", len(resp.Msgs))
	}
	if resp.GetUpdatesBuf != "cursor-1" {
		t.Errorf("buf = %q", resp.GetUpdatesBuf)
	}
	if resp.LongPollingTimeoutMs != 40000 {
		t.Errorf("timeout = %d", resp.LongPollingTimeoutMs)
	}
}

func TestGetUpdates_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	_, err := c.GetUpdates(context.Background(), "buf")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != 500 {
		t.Errorf("status = %d, want 500", httpErr.StatusCode)
	}
}

func TestSendText_Success(t *testing.T) {
	var received SendMessageReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	clientID, err := c.SendText(context.Background(), "user-1", "hello world", "ctx-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clientID == "" {
		t.Error("empty client ID")
	}
	if received.Msg.ToUserID != "user-1" {
		t.Errorf("to = %q", received.Msg.ToUserID)
	}
	if received.Msg.ContextToken != "ctx-tok" {
		t.Errorf("contextToken = %q", received.Msg.ContextToken)
	}
	if received.Msg.MessageType != MsgTypeBot {
		t.Errorf("messageType = %d", received.Msg.MessageType)
	}
	if received.Msg.MessageState != StateFinish {
		t.Errorf("messageState = %d", received.Msg.MessageState)
	}
	if len(received.Msg.ItemList) != 1 || received.Msg.ItemList[0].TextItem.Text != "hello world" {
		t.Error("text item mismatch")
	}
	if received.BaseInfo == nil || received.BaseInfo.ChannelVersion != iLinkChannelVersion {
		t.Error("base_info missing or wrong version")
	}
}

func TestGetConfig_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(GetConfigResp{
			Ret:          0,
			TypingTicket: "ticket-abc",
		})
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	resp, err := c.GetConfig(context.Background(), "user-1", "ctx-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TypingTicket != "ticket-abc" {
		t.Errorf("ticket = %q", resp.TypingTicket)
	}
}

func TestDoPost_Non2xxError(t *testing.T) {
	// Test that 3xx responses are treated as errors (not just >=400)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(301)
		w.Write([]byte("moved"))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL), WithHTTPClient(&http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}))
	_, err := c.GetUpdates(context.Background(), "")
	if err == nil {
		t.Fatal("expected error on 301")
	}
}

func TestSendText_FromUserIDAlwaysSent(t *testing.T) {
	var raw map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&raw)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	c.SendText(context.Background(), "user-1", "hi", "ctx")

	// Parse msg and verify from_user_id is present
	var msg map[string]json.RawMessage
	json.Unmarshal(raw["msg"], &msg)
	if _, ok := msg["from_user_id"]; !ok {
		t.Error("from_user_id should always be present in JSON")
	}
}

func TestNewClient_ILINK_CHANNEL_VERSION_EnvOverride(t *testing.T) {
	// Temporarily set the env var
	orig := os.Getenv("ILINK_CHANNEL_VERSION")
	os.Setenv("ILINK_CHANNEL_VERSION", "99.99.99")
	defer func() {
		if orig == "" {
			os.Unsetenv("ILINK_CHANNEL_VERSION")
		} else {
			os.Setenv("ILINK_CHANNEL_VERSION", orig)
		}
	}()

	c := NewClient("tok")
	if c.version != "99.99.99" {
		t.Errorf("version = %q, want env override value %q", c.version, "99.99.99")
	}
}

func TestNewClient_ILINK_CHANNEL_VERSION_EnvEmpty(t *testing.T) {
	orig := os.Getenv("ILINK_CHANNEL_VERSION")
	os.Unsetenv("ILINK_CHANNEL_VERSION")
	defer func() {
		if orig != "" {
			os.Setenv("ILINK_CHANNEL_VERSION", orig)
		}
	}()

	c := NewClient("tok")
	if c.version != iLinkChannelVersion {
		t.Errorf("version = %q, want default %q", c.version, iLinkChannelVersion)
	}
}

