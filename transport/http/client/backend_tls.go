// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/logging"
	httpserver "github.com/pucora/lura/v2/transport/http/server"
)

// BackendHTTPClientNamespace is the extra_config key for per-backend HTTP client options.
const BackendHTTPClientNamespace = "backend/http/client"

// ParseBackendHTTPClientTLS reads client_tls from backend/http/client extra_config.
func ParseBackendHTTPClientTLS(cfg *config.Backend) (*config.ClientTLS, bool) {
	if cfg == nil || cfg.ExtraConfig == nil {
		return nil, false
	}
	v, ok := cfg.ExtraConfig[BackendHTTPClientNamespace].(map[string]interface{})
	if !ok {
		return nil, false
	}
	tlsRaw, ok := v["client_tls"]
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(tlsRaw)
	if err != nil {
		return nil, false
	}
	var clientTLS config.ClientTLS
	if err := json.Unmarshal(raw, &clientTLS); err != nil {
		return nil, false
	}
	return &clientTLS, true
}

// NewHTTPClientWithBackendTLS returns an HTTP client factory that applies per-backend TLS
// settings from backend/http/client.client_tls when configured.
func NewHTTPClientWithBackendTLS(cfg *config.Backend, next HTTPClientFactory, logger logging.Logger) HTTPClientFactory {
	clientTLS, ok := ParseBackendHTTPClientTLS(cfg)
	if !ok {
		return next
	}
	tlsConfig := httpserver.ParseClientTLSConfigWithLogger(clientTLS, logger)
	if tlsConfig == nil {
		return next
	}
	return func(ctx context.Context) *http.Client {
		base := next(ctx)
		transport := cloneTransport(base.Transport)
		transport.TLSClientConfig = tlsConfig
		return &http.Client{
			Transport:     transport,
			CheckRedirect: base.CheckRedirect,
			Jar:           base.Jar,
			Timeout:       base.Timeout,
		}
	}
}

func cloneTransport(rt http.RoundTripper) *http.Transport {
	if rt == nil {
		if dt, ok := http.DefaultTransport.(*http.Transport); ok {
			return dt.Clone()
		}
		return &http.Transport{}
	}
	if t, ok := rt.(*http.Transport); ok {
		return t.Clone()
	}
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		return dt.Clone()
	}
	return &http.Transport{}
}
