// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/encoding"
)

type flushRecorder struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	flushes int
}

func (f *flushRecorder) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *flushRecorder) Flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes++
}

func (f *flushRecorder) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

func (f *flushRecorder) FlushCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flushes
}

func TestIsStreamingEndpoint(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.EndpointConfig
		want bool
	}{
		{
			name: "valid streaming",
			cfg: &config.EndpointConfig{
				OutputEncoding: encoding.NOOP,
				Backend:        []*config.Backend{{Encoding: encoding.NOOP}},
			},
			want: true,
		},
		{
			name: "multiple backends",
			cfg: &config.EndpointConfig{
				OutputEncoding: encoding.NOOP,
				Backend: []*config.Backend{
					{Encoding: encoding.NOOP},
					{Encoding: encoding.NOOP},
				},
			},
			want: false,
		},
		{
			name: "json output",
			cfg: &config.EndpointConfig{
				OutputEncoding: encoding.JSON,
				Backend:        []*config.Backend{{Encoding: encoding.NOOP}},
			},
			want: false,
		},
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStreamingEndpoint(tc.cfg); got != tc.want {
				t.Fatalf("IsStreamingEndpoint() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStreamCopy_flushesPerChunk(t *testing.T) {
	rec := &flushRecorder{}
	src := strings.NewReader("chunk-one-chunk-two")
	if _, err := StreamCopy(rec, src); err != nil {
		t.Fatal(err)
	}
	if rec.String() != "chunk-one-chunk-two" {
		t.Fatalf("unexpected body: %q", rec.String())
	}
	if rec.FlushCount() < 1 {
		t.Fatalf("expected at least one flush, got %d", rec.FlushCount())
	}
}

func TestStreamCopy_incrementalDelivery(t *testing.T) {
	pr, pw := io.Pipe()
	done := make(chan struct{})
	rec := &flushRecorder{}

	go func() {
		defer close(done)
		_, _ = StreamCopy(rec, pr)
	}()

	for i := 0; i < 3; i++ {
		if _, err := pw.Write([]byte("evt")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = pw.Close()
	<-done

	if !strings.Contains(rec.String(), "evtevtevt") {
		t.Fatalf("unexpected streamed body: %q", rec.String())
	}
}

func TestStreamCopy_noFlusher(t *testing.T) {
	var buf bytes.Buffer
	src := strings.NewReader("plain")
	if _, err := StreamCopy(&buf, src); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "plain" {
		t.Fatalf("unexpected body: %q", buf.String())
	}
}

var _ http.Flusher = (*flushRecorder)(nil)
