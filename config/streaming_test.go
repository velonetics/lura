// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"
	"time"

	"github.com/pucora/lura/v2/encoding"
)

func baseStreamingService() *ServiceConfig {
	return &ServiceConfig{
		Version: ConfigVersion,
		Host:    []string{"http://127.0.0.1:8081"},
		Endpoints: []*EndpointConfig{
			{
				Endpoint:       "/stream",
				OutputEncoding: encoding.NOOP,
				Backend: []*Backend{{
					Encoding:    encoding.NOOP,
					Host:        []string{"http://127.0.0.1:8081"},
					URLPattern:  "/events",
					Decoder:     encoding.NoOpDecoder,
				}},
			},
		},
	}
}

func TestValidateStreamingService_writeTimeout(t *testing.T) {
	s := baseStreamingService()
	s.WriteTimeout = 30 * time.Second
	if err := s.Init(); !errors.Is(err, errStreamingServiceWriteTimeout) {
		t.Fatalf("expected write_timeout error, got %v", err)
	}
}

func TestValidateStreamingService_responseHeaderTimeout(t *testing.T) {
	s := baseStreamingService()
	s.ResponseHeaderTimeout = 5 * time.Second
	if err := s.Init(); !errors.Is(err, errStreamingServiceHeaderTimeout) {
		t.Fatalf("expected response_header_timeout error, got %v", err)
	}
}

func TestValidateStreamingService_ok(t *testing.T) {
	s := baseStreamingService()
	s.WriteTimeout = 0
	s.ResponseHeaderTimeout = 0
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStreamingEndpoint_luaPost(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].ExtraConfig = ExtraConfig{
		luaRouterNamespace: map[string]interface{}{
			"post": "local r = response.load()",
		},
	}
	if err := s.Init(); !errors.Is(err, errStreamingResponseManipulation) {
		t.Fatalf("expected lua post error, got %v", err)
	}
}

func TestValidateStreamingEndpoint_responseJSONSchema(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].ExtraConfig = ExtraConfig{
		"validation/response-json-schema": map[string]interface{}{},
	}
	if err := s.Init(); !errors.Is(err, errStreamingResponseManipulation) {
		t.Fatalf("expected response schema error, got %v", err)
	}
}

func TestValidateStreamingEndpoint_sequentialProxy(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].ExtraConfig = ExtraConfig{
		proxyNamespace: map[string]interface{}{"sequential": true},
	}
	if err := s.Init(); !errors.Is(err, errStreamingSequentialProxy) {
		t.Fatalf("expected sequential proxy error, got %v", err)
	}
}

func TestValidateStreamingEndpoint_backendHTTPCache(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].Backend[0].ExtraConfig = ExtraConfig{
		httpcacheNamespace: map[string]interface{}{"shared": true},
	}
	if err := s.Init(); !errors.Is(err, errStreamingBackendHTTPCache) {
		t.Fatalf("expected httpcache error, got %v", err)
	}
}

func TestValidateStreamingEndpoint_martianResponseScope(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].Backend[0].ExtraConfig = ExtraConfig{
		martianNamespace: map[string]interface{}{
			"header.Modifier": map[string]interface{}{
				"scope": []interface{}{"response"},
				"name":  "X-Test",
				"value": "1",
			},
		},
	}
	if err := s.Init(); !errors.Is(err, errStreamingResponseManipulation) {
		t.Fatalf("expected martian response error, got %v", err)
	}
}

func TestValidateStreamingEndpoint_martianRequestScopeAllowed(t *testing.T) {
	s := baseStreamingService()
	s.Endpoints[0].Backend[0].ExtraConfig = ExtraConfig{
		martianNamespace: map[string]interface{}{
			"header.Modifier": map[string]interface{}{
				"scope": []interface{}{"request"},
				"name":  "X-Test",
				"value": "1",
			},
		},
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
}
