package ilink

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

const (
	maxQRRefreshCount   = 3
	qrLongPollTimeout   = 35 * time.Second
	defaultLoginTimeout = 8 * time.Minute
)

// FetchQRCode requests a new login QR code from the API.
func (c *Client) FetchQRCode(ctx context.Context) (*QRCodeResponse, error) {
	base := ensureTrailingSlash(c.baseURL)
	botType := c.botType
	if botType == "" {
		botType = DefaultBotType
	}
	u, _ := url.JoinPath(base, "ilink/bot/get_bot_qrcode")
	u += "?bot_type=" + url.QueryEscape(botType)

	headers := c.routeTagHeaders()
	apiResp, err := c.doGet(ctx, u, headers, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("ilink: fetch QR code: %w", err)
	}

	var resp QRCodeResponse
	if err := json.Unmarshal(apiResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("ilink: unmarshal QR response: %w", err)
	}
	resp.setRawResponse(apiResp)
	return &resp, nil
}

// PollQRStatus polls the scan status of a QR code.
func (c *Client) PollQRStatus(ctx context.Context, qrcode string) (*QRStatusResponse, error) {
	base := ensureTrailingSlash(c.baseURL)
	u, _ := url.JoinPath(base, "ilink/bot/get_qrcode_status")
	u += "?qrcode=" + url.QueryEscape(qrcode)

	headers := c.routeTagHeaders()
	headers["iLink-App-ClientVersion"] = "1"

	apiResp, err := c.doGet(ctx, u, headers, qrLongPollTimeout)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if isTimeoutError(err) {
			return &QRStatusResponse{Status: "wait"}, nil
		}
		return nil, err
	}

	var resp QRStatusResponse
	if err := json.Unmarshal(apiResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("ilink: unmarshal QR status: %w", err)
	}
	resp.setRawResponse(apiResp)
	return &resp, nil
}

// LoginCallbacks receives events during the QR login flow.
type LoginCallbacks struct {
	// OnQRCode is called with the QR code image content for the user to scan.
	OnQRCode func(qrcodeImgContent string)
	// OnScanned is called once after the user scans the QR code.
	OnScanned func()
	// OnExpired is called when the QR code expires and a new one is fetched.
	OnExpired func(attempt, maxAttempts int)
}

// LoginWithQR performs the full QR code login flow.
//
// On success the client's token and base URL are updated automatically.
// A non-nil LoginResult with Connected=false and a nil error indicates
// a timeout or user-side issue, not a transport error.
func (c *Client) LoginWithQR(ctx context.Context, cb *LoginCallbacks) (*LoginResult, error) {
	if cb == nil {
		cb = &LoginCallbacks{}
	}

	ctx, cancel := context.WithTimeout(ctx, defaultLoginTimeout)
	defer cancel()

	qr, err := c.FetchQRCode(ctx)
	if err != nil {
		return nil, err
	}
	if cb.OnQRCode != nil {
		cb.OnQRCode(qr.QRCodeImgContent)
	}

	scannedNotified := false
	refreshCount := 1
	currentQR := qr.QRCode

	for {
		select {
		case <-ctx.Done():
			return &LoginResult{Message: "login timeout"}, nil
		default:
		}

		status, err := c.PollQRStatus(ctx, currentQR)
		if err != nil {
			return nil, fmt.Errorf("ilink: poll QR status: %w", err)
		}

		switch status.Status {
		case "wait":
			// keep polling

		case "scaned":
			if !scannedNotified {
				scannedNotified = true
				if cb.OnScanned != nil {
					cb.OnScanned()
				}
			}

		case "expired":
			refreshCount++
			if refreshCount > maxQRRefreshCount {
				return &LoginResult{Message: "QR code expired too many times"}, nil
			}
			if cb.OnExpired != nil {
				cb.OnExpired(refreshCount, maxQRRefreshCount)
			}
			newQR, err := c.FetchQRCode(ctx)
			if err != nil {
				return nil, fmt.Errorf("ilink: refresh QR code: %w", err)
			}
			currentQR = newQR.QRCode
			scannedNotified = false
			if cb.OnQRCode != nil {
				cb.OnQRCode(newQR.QRCodeImgContent)
			}

		case "confirmed":
			if status.ILinkBotID == "" {
				return &LoginResult{Message: "server did not return bot ID"}, nil
			}
			c.SetToken(status.BotToken)
			if status.BaseURL != "" {
				c.SetBaseURL(status.BaseURL)
			}
			return &LoginResult{
				Connected: true,
				BotToken:  status.BotToken,
				BotID:     status.ILinkBotID,
				BaseURL:   status.BaseURL,
				UserID:    status.ILinkUserID,
				Message:   "connected",
			}, nil
		}

		sleepCtx(ctx, 1*time.Second)
	}
}
