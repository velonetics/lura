// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"io"
	"net/http"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/encoding"
)

// IsStreamingEndpoint reports whether the endpoint is configured for transparent
// HTTP streaming (KrakenD-style no-op proxy with a single no-op backend).
func IsStreamingEndpoint(cfg *config.EndpointConfig) bool {
	if cfg == nil || cfg.OutputEncoding != encoding.NOOP || len(cfg.Backend) != 1 {
		return false
	}
	return cfg.Backend[0].Encoding == encoding.NOOP
}

// StreamCopy copies from r to w, flushing after each write when w supports http.Flusher.
// This is required for SSE and other long-lived HTTP streams where clients must receive
// chunks as they arrive rather than when the buffer fills.
func StreamCopy(w io.Writer, r io.Reader) (int64, error) {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, er := r.Read(buf)
		if nr > 0 {
			nw, ew := w.Write(buf[:nr])
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}
