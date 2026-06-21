// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/logging"
)

func TestParseBackendHTTPClientTLS(t *testing.T) {
	cfg := &config.Backend{
		ExtraConfig: config.ExtraConfig{
			BackendHTTPClientNamespace: map[string]interface{}{
				"client_tls": map[string]interface{}{
					"allow_insecure_connections": true,
					"client_certs": []map[string]interface{}{
						{
							"certificate": "cert.pem",
							"private_key": "key.pem",
						},
					},
				},
			},
		},
	}
	clientTLS, ok := ParseBackendHTTPClientTLS(cfg)
	if !ok {
		t.Fatal("expected client_tls to be parsed")
	}
	if !clientTLS.AllowInsecureConnections {
		t.Fatal("expected allow_insecure_connections")
	}
	if len(clientTLS.ClientCerts) != 1 {
		t.Fatalf("expected one client cert, got %d", len(clientTLS.ClientCerts))
	}
}

func TestParseBackendHTTPClientTLS_missing(t *testing.T) {
	if _, ok := ParseBackendHTTPClientTLS(&config.Backend{}); ok {
		t.Fatal("expected missing config")
	}
}

func TestNewHTTPClientWithBackendTLS_appliesConfig(t *testing.T) {
	caCert, err := os.ReadFile("../../server/ca.pem")
	if err != nil {
		t.Skip("test certificates not available:", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		t.Fatal("cannot load ca.pem")
	}

	clientCert, err := tls.LoadX509KeyPair("../../server/cert.pem", "../../server/key.pem")
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "no client cert", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	server.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,
	}
	server.StartTLS()
	defer server.Close()

	backendCfg := &config.Backend{
		ExtraConfig: config.ExtraConfig{
			BackendHTTPClientNamespace: mustJSONMap(t, map[string]interface{}{
				"client_tls": map[string]interface{}{
					"ca_certs":             []string{"../../server/ca.pem"},
					"disable_system_ca_pool": true,
					"client_certs": []map[string]interface{}{
						{
							"certificate": "../../server/cert.pem",
							"private_key": "../../server/key.pem",
						},
					},
				},
			}),
		},
	}

	factory := NewHTTPClientWithBackendTLS(backendCfg, NewHTTPClient, logging.NoOp)
	httpClient := factory(context.Background())
	transport := httpClient.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLS config on transport")
	}
	if len(transport.TLSClientConfig.Certificates) == 0 {
		transport.TLSClientConfig.Certificates = []tls.Certificate{clientCert}
	}

	resp, err := httpClient.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func mustJSONMap(t *testing.T, v map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
