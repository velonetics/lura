// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pucora/lura/v2/encoding"
)

const (
	luaProxyNamespace  = "github.com/pucora/velonetics-lua/proxy"
	luaRouterNamespace = "github.com/pucora/velonetics-lua/router"
	martianNamespace   = "github.com/pucora/velonetics-martian"
	httpcacheNamespace = "github.com/pucora/velonetics-httpcache"
	proxyNamespace     = "github.com/pucora/pucora/proxy"
)

var (
	errStreamingResponseManipulation = fmt.Errorf("HTTP streaming endpoints cannot use response manipulation middleware")
	errStreamingSequentialProxy      = fmt.Errorf("HTTP streaming endpoints cannot use sequential proxy")
	errStreamingBackendHTTPCache     = fmt.Errorf("HTTP streaming backends cannot use HTTP cache")
	errStreamingServiceWriteTimeout  = fmt.Errorf("service write_timeout must be 0 when HTTP streaming endpoints are present")
	errStreamingServiceHeaderTimeout = fmt.Errorf("service response_header_timeout is too low for HTTP streaming endpoints")

	// Exported errors for callers and tests.
	ErrStreamingResponseManipulation = errStreamingResponseManipulation
	ErrStreamingSequentialProxy      = errStreamingSequentialProxy
	ErrStreamingBackendHTTPCache     = errStreamingBackendHTTPCache
	ErrStreamingServiceWriteTimeout  = errStreamingServiceWriteTimeout
	ErrStreamingServiceHeaderTimeout = errStreamingServiceHeaderTimeout
)

// minStreamingResponseHeaderTimeout is the minimum recommended backend first-byte wait
// when response_header_timeout is explicitly set on a config with streaming endpoints.
const minStreamingResponseHeaderTimeout = 30 * time.Second

func isStreamingEndpoint(e *EndpointConfig) bool {
	if e == nil || e.OutputEncoding != encoding.NOOP || len(e.Backend) != 1 {
		return false
	}
	return e.Backend[0].Encoding == encoding.NOOP
}

func serviceHasStreamingEndpoint(s *ServiceConfig) bool {
	for _, e := range s.Endpoints {
		if isStreamingEndpoint(e) {
			return true
		}
	}
	return false
}

func validateStreamingService(s *ServiceConfig) error {
	if !serviceHasStreamingEndpoint(s) {
		return nil
	}
	if s.WriteTimeout > 0 {
		return fmt.Errorf("%w: set \"write_timeout\": \"0s\" at the service level", errStreamingServiceWriteTimeout)
	}
	if s.ResponseHeaderTimeout > 0 && s.ResponseHeaderTimeout < minStreamingResponseHeaderTimeout {
		return fmt.Errorf(
			"%w: increase \"response_header_timeout\" to at least %s or set it to \"0s\" to disable",
			errStreamingServiceHeaderTimeout,
			minStreamingResponseHeaderTimeout,
		)
	}
	return nil
}

func validateStreamingEndpoint(e *EndpointConfig) error {
	if !isStreamingEndpoint(e) {
		return nil
	}
	if extraConfigHasResponseManipulation(e.ExtraConfig) {
		return fmt.Errorf("%w on endpoint %s", errStreamingResponseManipulation, e.Endpoint)
	}
	if proxySequentialEnabled(e.ExtraConfig) {
		return fmt.Errorf("%w on endpoint %s", errStreamingSequentialProxy, e.Endpoint)
	}
	for _, b := range e.Backend {
		if _, ok := b.ExtraConfig[httpcacheNamespace]; ok {
			return fmt.Errorf("%w on endpoint %s backend %s", errStreamingBackendHTTPCache, e.Endpoint, b.URLPattern)
		}
		if martianModifiesResponse(b.ExtraConfig) {
			return fmt.Errorf("%w on endpoint %s backend %s (martian response scope)", errStreamingResponseManipulation, e.Endpoint, b.URLPattern)
		}
	}
	return nil
}

func extraConfigHasResponseManipulation(ec ExtraConfig) bool {
	if hasLuaPost(ec) {
		return true
	}
	if _, ok := ec["validation/response-json-schema"]; ok {
		return true
	}
	if m, ok := ec["modifier/response-body"].(map[string]interface{}); ok {
		if mods, ok := m["modifiers"].([]interface{}); ok && len(mods) > 0 {
			return true
		}
	}
	if m, ok := ec["modifier/response-headers"].(map[string]interface{}); ok && len(m) > 0 {
		return true
	}
	if martianModifiesResponse(ec) {
		return true
	}
	return false
}

func hasLuaPost(ec ExtraConfig) bool {
	for _, ns := range []string{luaProxyNamespace, luaRouterNamespace} {
		cfg, ok := ec[ns].(map[string]interface{})
		if !ok {
			continue
		}
		if post, ok := cfg["post"].(string); ok && strings.TrimSpace(post) != "" {
			return true
		}
	}
	return false
}

func proxySequentialEnabled(ec ExtraConfig) bool {
	cfg, ok := ec[proxyNamespace].(map[string]interface{})
	if !ok {
		return false
	}
	sequential, ok := cfg["sequential"].(bool)
	return ok && sequential
}

func martianModifiesResponse(ec ExtraConfig) bool {
	raw, ok := ec[martianNamespace]
	if !ok {
		return false
	}
	return jsonScopeContainsResponse(raw)
}

func jsonScopeContainsResponse(v interface{}) bool {
	switch t := v.(type) {
	case map[string]interface{}:
		if scopeReferencesResponse(t["scope"]) {
			return true
		}
		for _, child := range t {
			if jsonScopeContainsResponse(child) {
				return true
			}
		}
	case []interface{}:
		for _, item := range t {
			if jsonScopeContainsResponse(item) {
				return true
			}
		}
	case string:
		// Martian configs are JSON objects; ignore bare strings.
		return false
	default:
		if b, err := json.Marshal(v); err == nil {
			return strings.Contains(string(b), `"response"`) && strings.Contains(string(b), `"scope"`)
		}
	}
	return false
}

func scopeReferencesResponse(scope interface{}) bool {
	switch s := scope.(type) {
	case string:
		return s == "response"
	case []interface{}:
		for _, item := range s {
			if str, ok := item.(string); ok && str == "response" {
				return true
			}
		}
	}
	return false
}
