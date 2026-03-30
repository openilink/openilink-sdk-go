package ilink

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	uploadMaxRetries  = 3
	defaultCDNTimeout = 60 * time.Second
)

// --- AES-128-ECB encrypt / decrypt ---

// pkcs7Pad pads data to a multiple of blockSize using PKCS#7.
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

// pkcs7Unpad removes PKCS#7 padding.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ilink: pkcs7 unpad: empty data")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(data) {
		return nil, fmt.Errorf("ilink: pkcs7 unpad: invalid padding %d", pad)
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("ilink: pkcs7 unpad: inconsistent padding")
		}
	}
	return data[:len(data)-pad], nil
}

// EncryptAESECB encrypts plaintext with AES-128-ECB and PKCS#7 padding.
func EncryptAESECB(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ilink: aes cipher: %w", err)
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	return ciphertext, nil
}

// DecryptAESECB decrypts AES-128-ECB ciphertext and removes PKCS#7 padding.
func DecryptAESECB(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ilink: aes cipher: %w", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ilink: ciphertext not multiple of block size")
	}
	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(plaintext[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(plaintext)
}

// AESECBPaddedSize returns the ciphertext size for a given plaintext size.
// Equivalent to Math.ceil((plaintextSize + 1) / 16) * 16.
func AESECBPaddedSize(plaintextSize int) int {
	return ((plaintextSize + 1 + aes.BlockSize - 1) / aes.BlockSize) * aes.BlockSize
}

// --- CDN URL builders ---

// BuildCDNDownloadURL constructs a CDN download URL from an encrypted query param.
func BuildCDNDownloadURL(cdnBaseURL, encryptedQueryParam string) string {
	return cdnBaseURL + "/download?encrypted_query_param=" + url.QueryEscape(encryptedQueryParam)
}

// BuildCDNUploadURL constructs a CDN upload URL from upload param and filekey.
func BuildCDNUploadURL(cdnBaseURL, uploadParam, filekey string) string {
	return cdnBaseURL + "/upload?encrypted_query_param=" + url.QueryEscape(uploadParam) +
		"&filekey=" + url.QueryEscape(filekey)
}

// --- AES key parsing ---

// ParseAESKey decodes a base64-encoded AES key.
// Two encodings are seen in the wild:
//   - base64(raw 16 bytes) — images
//   - base64(hex string of 16 bytes) — file / voice / video
func ParseAESKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := decodeBase64Flexible(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("ilink: decode aes_key: %w", err)
	}
	if len(decoded) == 16 {
		return decoded, nil
	}
	if len(decoded) == 32 && isHex(decoded) {
		raw, err := hex.DecodeString(string(decoded))
		if err != nil {
			return nil, fmt.Errorf("ilink: decode hex aes_key: %w", err)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("ilink: aes_key must decode to 16 raw bytes or 32-char hex, got %d bytes", len(decoded))
}

func isHex(b []byte) bool {
	for _, c := range b {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// decodeBase64Flexible decodes base64 with support for both standard and
// URL-safe alphabets, with or without padding.
func decodeBase64Flexible(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64: %q", s)
}

// --- CDN upload / download ---

// UploadResult holds the result of a CDN upload.
type UploadResult struct {
	FileKey                    string
	DownloadEncryptedQueryParam string
	AESKey                     string
	FileSize                   int64
	CiphertextSize             int64
}

// UploadFile uploads a file to the Weixin CDN with AES-128-ECB encryption.
// It handles the full pipeline: MD5 hash, AES key generation, getUploadUrl API
// call, encryption, and CDN POST with retry.
func (c *Client) UploadFile(ctx context.Context, plaintext []byte, toUserID string, mediaType UploadMediaType) (*UploadResult, error) {
	rawSize := int64(len(plaintext))
	rawMD5 := fmt.Sprintf("%x", md5.Sum(plaintext))
	fileSize := int64(AESECBPaddedSize(int(rawSize)))
	filekey := randomHex(16)
	aesKey := make([]byte, 16)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("ilink: generate aes key: %w", err)
	}

	// 1. Get pre-signed upload URL
	uploadResp, err := c.GetUploadURL(ctx, &GetUploadURLReq{
		FileKey:     filekey,
		MediaType:   int(mediaType),
		ToUserID:    toUserID,
		RawSize:     rawSize,
		RawFileMD5:  rawMD5,
		FileSize:    fileSize,
		NoNeedThumb: true,
		AESKey:      hex.EncodeToString(aesKey),
	})
	if err != nil {
		return nil, fmt.Errorf("ilink: getUploadUrl: %w", err)
	}
	if uploadResp.Ret != 0 {
		return nil, &APIError{Ret: uploadResp.Ret, ErrMsg: uploadResp.ErrMsg}
	}
	// Prefer upload_full_url; fallback to building from upload_param.
	var cdnURL string
	if u := trimString(uploadResp.UploadFullURL); u != "" {
		cdnURL = u
	} else if uploadResp.UploadParam != "" {
		cdnURL = BuildCDNUploadURL(c.cdnBaseURL, uploadResp.UploadParam, filekey)
	} else {
		return nil, fmt.Errorf("ilink: getUploadUrl returned no upload URL (need upload_full_url or upload_param)")
	}

	// 2. Encrypt
	ciphertext, err := EncryptAESECB(plaintext, aesKey)
	if err != nil {
		return nil, err
	}

	// 3. Upload to CDN with retry
	downloadParam, err := c.uploadToCDN(ctx, cdnURL, ciphertext)
	if err != nil {
		return nil, err
	}

	return &UploadResult{
		FileKey:                    filekey,
		DownloadEncryptedQueryParam: downloadParam,
		AESKey:                     hex.EncodeToString(aesKey),
		FileSize:                   rawSize,
		CiphertextSize:             int64(len(ciphertext)),
	}, nil
}

func (c *Client) uploadToCDN(ctx context.Context, cdnURL string, ciphertext []byte) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= uploadMaxRetries; attempt++ {
		downloadParam, err := c.doUpload(ctx, cdnURL, ciphertext)
		if err == nil {
			return downloadParam, nil
		}
		lastErr = err
		// Abort immediately on client errors (4xx)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return "", err
		}
		if attempt < uploadMaxRetries {
			sleepCtx(ctx, retryDelay)
		}
	}
	return "", fmt.Errorf("ilink: cdn upload failed after %d attempts: %w", uploadMaxRetries, lastErr)
}

func (c *Client) doUpload(ctx context.Context, cdnURL string, body []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCDNTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdnURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ilink: cdn request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := resp.Header.Get("x-error-message")
		if errMsg == "" && len(respBody) > 0 {
			errMsg = string(respBody)
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return "", &HTTPError{StatusCode: resp.StatusCode, Body: []byte(errMsg)}
	}
	io.Copy(io.Discard, resp.Body)

	downloadParam := resp.Header.Get("x-encrypted-param")
	if downloadParam == "" {
		return "", fmt.Errorf("ilink: cdn response missing x-encrypted-param header")
	}
	return downloadParam, nil
}

// resolveCDNDownloadURL returns the download URL for a CDNMedia reference.
// It prefers FullURL when available, falling back to building from EncryptQueryParam.
func (c *Client) resolveCDNDownloadURL(media *CDNMedia) (string, error) {
	if media.FullURL != "" {
		return media.FullURL, nil
	}
	if media.EncryptQueryParam != "" {
		return BuildCDNDownloadURL(c.cdnBaseURL, media.EncryptQueryParam), nil
	}
	return "", fmt.Errorf("ilink: cdn media has no full_url or encrypt_query_param")
}

// DownloadMedia downloads and decrypts media from the CDN using a [CDNMedia] reference.
// It prefers [CDNMedia.FullURL] when available, falling back to building from EncryptQueryParam.
func (c *Client) DownloadMedia(ctx context.Context, media *CDNMedia) ([]byte, error) {
	if media == nil {
		return nil, fmt.Errorf("ilink: media is nil")
	}
	key, err := ParseAESKey(media.AESKey)
	if err != nil {
		return nil, err
	}
	dlURL, err := c.resolveCDNDownloadURL(media)
	if err != nil {
		return nil, err
	}
	apiResp, err := c.doGet(ctx, dlURL, nil, defaultCDNTimeout)
	if err != nil {
		return nil, fmt.Errorf("ilink: cdn download: %w", err)
	}
	plaintext, err := DecryptAESECB(apiResp.Body, key)
	if err != nil {
		return nil, fmt.Errorf("ilink: cdn decrypt: %w", err)
	}
	return plaintext, nil
}

// DownloadFile downloads and decrypts a file from the Weixin CDN.
// aesKeyBase64 is the aes_key field from CDNMedia (base64-encoded).
//
// Deprecated: Use [Client.DownloadMedia] which supports CDNMedia.FullURL.
func (c *Client) DownloadFile(ctx context.Context, encryptedQueryParam, aesKeyBase64 string) ([]byte, error) {
	return c.DownloadMedia(ctx, &CDNMedia{
		EncryptQueryParam: encryptedQueryParam,
		AESKey:            aesKeyBase64,
	})
}

// DownloadMediaRaw downloads raw bytes from the CDN using a [CDNMedia] reference, without decryption.
func (c *Client) DownloadMediaRaw(ctx context.Context, media *CDNMedia) ([]byte, error) {
	if media == nil {
		return nil, fmt.Errorf("ilink: media is nil")
	}
	dlURL, err := c.resolveCDNDownloadURL(media)
	if err != nil {
		return nil, err
	}
	apiResp, err := c.doGet(ctx, dlURL, nil, defaultCDNTimeout)
	if err != nil {
		return nil, fmt.Errorf("ilink: cdn download: %w", err)
	}
	return apiResp.Body, nil
}

// DownloadRaw downloads raw bytes from the CDN without decryption.
//
// Deprecated: Use [Client.DownloadMediaRaw] which supports CDNMedia.FullURL.
func (c *Client) DownloadRaw(ctx context.Context, encryptedQueryParam string) ([]byte, error) {
	return c.DownloadMediaRaw(ctx, &CDNMedia{EncryptQueryParam: encryptedQueryParam})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
