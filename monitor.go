package ilink

import (
	"context"
	"fmt"
	"time"
)

const (
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second
)

// MessageHandler is called for each inbound message from getUpdates.
type MessageHandler func(msg WeixinMessage)

// MonitorOptions configures the long-poll [Client.Monitor] loop.
type MonitorOptions struct {
	// InitialBuf is the get_updates_buf to resume from (empty for fresh start).
	InitialBuf string

	// OnBufUpdate is called whenever a new sync cursor is received.
	// Persist this value to resume polling after a restart.
	OnBufUpdate func(buf string)

	// OnError is called on non-fatal poll errors. Defaults to no-op.
	OnError func(err error)

	// OnSessionExpired is called when the server returns errcode -14.
	OnSessionExpired func()
}

// Monitor runs a long-poll loop, invoking handler for each inbound message.
// It blocks until ctx is cancelled and handles retries/backoff automatically.
//
// Context tokens are cached automatically for use with [Client.Push].
func (c *Client) Monitor(ctx context.Context, handler MessageHandler, opts *MonitorOptions) error {
	if opts == nil {
		opts = &MonitorOptions{}
	}

	onError := opts.OnError
	if onError == nil {
		onError = func(error) {}
	}

	buf := opts.InitialBuf
	failures := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := c.GetUpdates(ctx, buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failures++
			onError(fmt.Errorf("getUpdates (%d/%d): %w", failures, maxConsecutiveFailures, err))
			if failures >= maxConsecutiveFailures {
				failures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		// API-level error
		if resp.Ret != 0 || resp.ErrCode != 0 {
			apiErr := &APIError{Ret: resp.Ret, ErrCode: resp.ErrCode, ErrMsg: resp.ErrMsg}

			if apiErr.IsSessionExpired() {
				if opts.OnSessionExpired != nil {
					opts.OnSessionExpired()
				}
				onError(apiErr)
				sleepCtx(ctx, 1*time.Hour)
				continue
			}

			failures++
			onError(fmt.Errorf("getUpdates (%d/%d): %w", failures, maxConsecutiveFailures, apiErr))
			if failures >= maxConsecutiveFailures {
				failures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		failures = 0

		// Update sync cursor
		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf
			if opts.OnBufUpdate != nil {
				opts.OnBufUpdate(buf)
			}
		}

		// Dispatch messages
		for _, msg := range resp.Msgs {
			if msg.ContextToken != "" && msg.FromUserID != "" {
				c.SetContextToken(msg.FromUserID, msg.ContextToken)
			}
			handler(msg)
		}
	}
}
