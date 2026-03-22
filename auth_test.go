package ilink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchQRCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "get_bot_qrcode") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("bot_type") != "3" {
			t.Errorf("bot_type = %q", r.URL.Query().Get("bot_type"))
		}
		json.NewEncoder(w).Encode(QRCodeResponse{
			QRCode:           "qr-code-string",
			QRCodeImgContent: "image-data",
		})
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL))
	resp, err := c.FetchQRCode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.QRCode != "qr-code-string" {
		t.Errorf("qrcode = %q", resp.QRCode)
	}
	if resp.QRCodeImgContent != "image-data" {
		t.Errorf("img content = %q", resp.QRCodeImgContent)
	}
}

func TestFetchQRCode_WithRouteTag(t *testing.T) {
	var gotRouteTag string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRouteTag = r.Header.Get("SKRouteTag")
		json.NewEncoder(w).Encode(QRCodeResponse{QRCode: "qr"})
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL), WithRouteTag("my-tag"))
	c.FetchQRCode(context.Background())

	if gotRouteTag != "my-tag" {
		t.Errorf("SKRouteTag = %q, want my-tag", gotRouteTag)
	}
}

func TestPollQRStatus_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("iLink-App-ClientVersion") != "1" {
			t.Errorf("missing iLink-App-ClientVersion header")
		}
		if !strings.Contains(r.URL.RawQuery, "qrcode=test-qr") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(QRStatusResponse{Status: "confirmed", BotToken: "tok123", ILinkBotID: "bot1"})
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL))
	resp, err := c.PollQRStatus(context.Background(), "test-qr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "confirmed" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.BotToken != "tok123" {
		t.Errorf("token = %q", resp.BotToken)
	}
}

func TestPollQRStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL))
	_, err := c.PollQRStatus(context.Background(), "qr")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestLoginWithQR_FullFlow(t *testing.T) {
	var pollCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "get_bot_qrcode") {
			json.NewEncoder(w).Encode(QRCodeResponse{QRCode: "qr1", QRCodeImgContent: "img1"})
			return
		}
		if strings.Contains(r.URL.Path, "get_qrcode_status") {
			pollCount++
			switch pollCount {
			case 1:
				json.NewEncoder(w).Encode(QRStatusResponse{Status: "wait"})
			case 2:
				json.NewEncoder(w).Encode(QRStatusResponse{Status: "scaned"})
			default:
				json.NewEncoder(w).Encode(QRStatusResponse{
					Status:     "confirmed",
					BotToken:   "bot-token",
					ILinkBotID: "bot-id",
					BaseURL:    "https://new-base.com",
					ILinkUserID: "user-id",
				})
			}
			return
		}
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL))

	var gotQRImg string
	var scanned bool

	result, err := c.LoginWithQR(context.Background(), &LoginCallbacks{
		OnQRCode:  func(img string) { gotQRImg = img },
		OnScanned: func() { scanned = true },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Connected {
		t.Fatal("expected connected")
	}
	if result.BotToken != "bot-token" {
		t.Errorf("token = %q", result.BotToken)
	}
	if result.BotID != "bot-id" {
		t.Errorf("botID = %q", result.BotID)
	}
	if result.UserID != "user-id" {
		t.Errorf("userID = %q", result.UserID)
	}
	if gotQRImg != "img1" {
		t.Errorf("QR img = %q", gotQRImg)
	}
	if !scanned {
		t.Error("OnScanned not called")
	}
	// Token and base URL should be updated on client
	if c.Token() != "bot-token" {
		t.Errorf("client token = %q", c.Token())
	}
	if c.BaseURL() != "https://new-base.com" {
		t.Errorf("client baseURL = %q", c.BaseURL())
	}
}

func TestLoginWithQR_NilCallbacks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "get_bot_qrcode") {
			json.NewEncoder(w).Encode(QRCodeResponse{QRCode: "qr1"})
			return
		}
		json.NewEncoder(w).Encode(QRStatusResponse{
			Status: "confirmed", BotToken: "t", ILinkBotID: "b",
		})
	}))
	defer srv.Close()

	c := NewClient("", WithBaseURL(srv.URL))
	result, err := c.LoginWithQR(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Connected {
		t.Fatal("expected connected")
	}
}
