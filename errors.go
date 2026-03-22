package ilink

import (
	"errors"
	"fmt"
)

// APIError represents an error response from the iLink API.
type APIError struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ilink: api error ret=%d errcode=%d errmsg=%s", e.Ret, e.ErrCode, e.ErrMsg)
}

// IsSessionExpired reports whether the error indicates an expired session (errcode -14).
func (e *APIError) IsSessionExpired() bool {
	return e.ErrCode == -14 || e.Ret == -14
}

// HTTPError represents a non-2xx HTTP response from the server.
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("ilink: http %d: %s", e.StatusCode, string(e.Body))
}

// Sentinel errors.
var (
	// ErrNoContextToken is returned by Push when no cached context token
	// exists for the target user.
	ErrNoContextToken = errors.New("ilink: no cached context token; user must send a message first")
)
