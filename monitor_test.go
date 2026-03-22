package ilink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMonitor_ReceivesMessages(t *testing.T) {
	var callCount atomic.Int32
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			json.NewEncoder(w).Encode(GetUpdatesResp{
				Ret: 0,
				Msgs: []WeixinMessage{
					{MessageID: 1, FromUserID: "u1", ContextToken: "ct1",
						ItemList: []MessageItem{{Type: ItemText, TextItem: &TextItem{Text: "hi"}}}},
					{MessageID: 2, FromUserID: "u2", ContextToken: "ct2"},
				},
				GetUpdatesBuf: "buf-1",
			})
		} else {
			// Return empty response after short delay
			select {
			case <-srvCtx.Done():
			case <-time.After(100 * time.Millisecond):
			}
			json.NewEncoder(w).Encode(GetUpdatesResp{Ret: 0})
		}
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	var received []WeixinMessage
	var savedBuf string

	done := make(chan error, 1)
	go func() {
		done <- c.Monitor(ctx, func(msg WeixinMessage) {
			mu.Lock()
			received = append(received, msg)
			mu.Unlock()
		}, &MonitorOptions{
			OnBufUpdate: func(buf string) { savedBuf = buf },
		})
	}()

	// Wait for messages to be processed
	time.Sleep(300 * time.Millisecond)
	cancel()
	srvCancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("received %d messages, want 2", len(received))
	}
	if received[0].MessageID != 1 {
		t.Errorf("first msg ID = %d", received[0].MessageID)
	}
	if savedBuf != "buf-1" {
		t.Errorf("saved buf = %q, want buf-1", savedBuf)
	}

	// Context tokens should be cached
	tok, ok := c.GetContextToken("u1")
	if !ok || tok != "ct1" {
		t.Errorf("context token for u1: (%q, %v)", tok, ok)
	}
	tok, ok = c.GetContextToken("u2")
	if !ok || tok != "ct2" {
		t.Errorf("context token for u2: (%q, %v)", tok, ok)
	}
}

func TestMonitor_CancelledContext(t *testing.T) {
	c := NewClient("tok")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.Monitor(ctx, func(msg WeixinMessage) {}, nil)
	if err != context.Canceled {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestMonitor_DynamicTimeout(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		if n == 1 {
			json.NewEncoder(w).Encode(GetUpdatesResp{
				Ret:                  0,
				GetUpdatesBuf:        "b1",
				LongPollingTimeoutMs: 50000,
			})
		} else {
			json.NewEncoder(w).Encode(GetUpdatesResp{Ret: 0, GetUpdatesBuf: "b2"})
		}
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- c.Monitor(ctx, func(msg WeixinMessage) {}, nil) }()

	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	if reqCount.Load() < 2 {
		t.Errorf("expected at least 2 requests, got %d", reqCount.Load())
	}
}

func TestMonitor_SessionExpiredResetsFailures(t *testing.T) {
	var reqCount atomic.Int32
	var errCount atomic.Int32
	var sessionExpiredCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		if n == 1 {
			// Session expired on first request
			json.NewEncoder(w).Encode(GetUpdatesResp{Ret: -14, ErrCode: -14, ErrMsg: "expired"})
		} else {
			json.NewEncoder(w).Encode(GetUpdatesResp{Ret: 0})
		}
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- c.Monitor(ctx, func(msg WeixinMessage) {}, &MonitorOptions{
			OnError:          func(err error) { errCount.Add(1) },
			OnSessionExpired: func() { sessionExpiredCalled.Store(true) },
		})
	}()

	// Wait for session expired to fire (it then sleeps 1hr, cancel interrupts)
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	if errCount.Load() < 1 {
		t.Errorf("expected at least 1 error, got %d", errCount.Load())
	}
	if !sessionExpiredCalled.Load() {
		t.Error("OnSessionExpired was not called")
	}
}
