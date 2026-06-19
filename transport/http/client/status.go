// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pucora/lura/v2/config"
)

// Namespace to be used in extra config
const Namespace = "github.com/pucora/pucora/http"

// ErrInvalidStatusCode is the error returned by the http proxy when the received status code
// is not a 200 nor a 201
var ErrInvalidStatusCode = errors.New("invalid status code")

type ErrInvalidStatus struct {
	statusCode int
	errPrefix  string
	path       string
}

func (e *ErrInvalidStatus) Error() string {
	return fmt.Sprintf("invalid status code %d %s %s", e.statusCode, e.errPrefix, e.path)
}

func NewErrInvalidStatusCode(resp *http.Response, errPrefix string) *ErrInvalidStatus {
	var p string
	if resp.Request != nil && resp.Request.URL != nil {
		p = resp.Request.URL.String()
	}

	return &ErrInvalidStatus{
		statusCode: resp.StatusCode,
		errPrefix:  errPrefix,
		path:       p,
	}
}

// HTTPStatusHandler defines how we tread the http response code
type HTTPStatusHandler func(context.Context, *http.Response) (*http.Response, error)

// GetHTTPStatusHandler returns a status handler. If the 'return_error_details' key is defined
// at the extra config, it returns a DetailedHTTPStatusHandler. Otherwise, it returns a
// DefaultHTTPStatusHandler
func GetHTTPStatusHandler(remote *config.Backend) HTTPStatusHandler {
	errPrefix := fmt.Sprintf("[%s %s]:", remote.Method, remote.URLPattern)
	// #region agent log
	if remote.URLPattern == "/unknown" {
		keys := make([]string, 0, len(remote.ExtraConfig))
		for k := range remote.ExtraConfig {
			keys = append(keys, k)
		}
		agentStatusDebugLog("status.go:GetHTTPStatusHandler", "backend http status handler lookup", "E", map[string]interface{}{
			"urlPattern":      remote.URLPattern,
			"namespace":       Namespace,
			"hasNamespaceCfg": remote.ExtraConfig[Namespace] != nil,
			"extraConfigKeys": keys,
		})
	}
	// #endregion
	if e, ok := remote.ExtraConfig[Namespace]; ok {
		if m, ok := e.(map[string]interface{}); ok {
			if v, ok := m["return_error_details"]; ok {
				if b, ok := v.(string); ok && b != "" {
					return DetailedHTTPStatusHandlerWithErrPrefix(b, errPrefix)
				}
			} else if v, ok := m["return_error_code"].(bool); ok && v {
				return ErrorHTTPStatusHandlerWithErrPrefix(errPrefix)
			}
		}
	}
	return DefaultHTTPStatusHandlerWithErrPrefix(errPrefix)
}

// DefaultHTTPStatusHandler is the default implementation of HTTPStatusHandler
func DefaultHTTPStatusHandler(_ context.Context, resp *http.Response) (*http.Response, error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, ErrInvalidStatusCode
	}

	return resp, nil
}

// DefaultHTTPStatusHandlerWithErrPrefix is the default implementation of HTTPStatusHandler
// with information about the failing status code, and the failed request
func DefaultHTTPStatusHandlerWithErrPrefix(errPrefix string) HTTPStatusHandler {
	return func(_ context.Context, resp *http.Response) (*http.Response, error) {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return nil, NewErrInvalidStatusCode(resp, errPrefix)
		}
		return resp, nil
	}
}

// ErrorHTTPStatusHandler is a HTTPStatusHandler that returns the status code as part of the error details
func ErrorHTTPStatusHandler(ctx context.Context, resp *http.Response) (*http.Response, error) {
	if _, err := DefaultHTTPStatusHandler(ctx, resp); err == nil {
		return resp, nil
	}
	return resp, newHTTPResponseError(resp)
}

// ErrorHTTPStatusHandlerWithErrPrefix is a HTTPStatusHandler that returns the status code as part of the error details
func ErrorHTTPStatusHandlerWithErrPrefix(errPrefix string) HTTPStatusHandler {
	defaultH := DefaultHTTPStatusHandlerWithErrPrefix(errPrefix)
	return func(ctx context.Context, resp *http.Response) (*http.Response, error) {
		if _, err := defaultH(ctx, resp); err == nil {
			return resp, nil
		}
		return resp, newHTTPResponseError(resp)
	}
}

// NoOpHTTPStatusHandler is a NO-OP implementation of HTTPStatusHandler
func NoOpHTTPStatusHandler(_ context.Context, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

// DetailedHTTPStatusHandler is a HTTPStatusHandler implementation
func DetailedHTTPStatusHandler(name string) HTTPStatusHandler {
	return func(ctx context.Context, resp *http.Response) (*http.Response, error) {
		if _, err := DefaultHTTPStatusHandler(ctx, resp); err == nil {
			return resp, nil
		}

		return resp, NamedHTTPResponseError{
			HTTPResponseError: newHTTPResponseError(resp),
			name:              name,
		}
	}
}

// DetailedHTTPStatusHandlerWithErrPrefix is a HTTPStatusHandlers that
// can receive an error prefix to be added when an error happens to help
// identify the endpoint using this handler.
func DetailedHTTPStatusHandlerWithErrPrefix(name, errPrefix string) HTTPStatusHandler {
	defaultH := DefaultHTTPStatusHandlerWithErrPrefix(errPrefix)
	return func(ctx context.Context, resp *http.Response) (*http.Response, error) {
		if _, err := defaultH(ctx, resp); err == nil {
			return resp, nil
		}

		return resp, NamedHTTPResponseError{
			HTTPResponseError: newHTTPResponseError(resp),
			name:              name,
		}
	}
}

func newHTTPResponseError(resp *http.Response) HTTPResponseError {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		body = []byte{}
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	return HTTPResponseError{
		Code: resp.StatusCode,
		Msg:  string(body),
		Enc:  resp.Header.Get("Content-Type"),
	}
}

// HTTPResponseError is the error to be returned by the ErrorHTTPStatusHandler
type HTTPResponseError struct {
	Code int    `json:"http_status_code"`
	Msg  string `json:"http_body,omitempty"`
	Enc  string `json:"http_body_encoding,omitempty"`
}

// Error returns the error message
func (r HTTPResponseError) Error() string {
	return r.Msg
}

// StatusCode returns the status code returned by the backend
func (r HTTPResponseError) StatusCode() int {
	return r.Code
}

// Encoding returns the content type returned by the backend
func (r HTTPResponseError) Encoding() string {
	return r.Enc
}

// NamedHTTPResponseError is the error to be returned by the DetailedHTTPStatusHandler
type NamedHTTPResponseError struct {
	HTTPResponseError
	name string
}

// Name returns the name of the backend where the error happened
func (r NamedHTTPResponseError) Name() string {
	return r.name
}

// #region agent log
func agentStatusDebugLog(location, message, hypothesisID string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"sessionId":    "d837bb",
		"timestamp":    time.Now().UnixMilli(),
		"location":     location,
		"message":      message,
		"hypothesisId": hypothesisID,
		"data":         data,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	f, err := os.OpenFile("/Users/niteesh.chaudhary/Documents/GitHub/velonetics/.cursor/debug-d837bb.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}

// #endregion
