package proxy

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/pucora/lura/v2/logging"
)

func TestGraphQLNilHeadersGET(t *testing.T) {
	mw := NewGraphQLMiddleware(logging.NoOp, graphqlBackend(map[string]interface{}{
		"type":   "query",
		"query":  "{ foo }",
		"method": "GET",
	}))
	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{IsComplete: true}, nil
	})
	_, err := prxy(context.Background(), &Request{
		Params:  map[string]string{},
		Headers: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGraphQLNilHeadersPOST(t *testing.T) {
	mw := NewGraphQLMiddleware(logging.NoOp, graphqlBackend(map[string]interface{}{
		"type":  "mutation",
		"query": "mutation { ok }",
	}))
	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{IsComplete: true}, nil
	})
	_, err := prxy(context.Background(), &Request{
		Body:    io.NopCloser(strings.NewReader(`{}`)),
		Params:  map[string]string{},
		Headers: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
}
