// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/logging"
	"github.com/pucora/lura/v2/transport/http/client/graphql"
)

func graphqlBackend(extra map[string]interface{}, opts ...func(*config.Backend)) *config.Backend {
	b := &config.Backend{
		ExtraConfig: config.ExtraConfig{
			graphql.Namespace: extra,
		},
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

func TestNewGraphQLMiddleware_mutationPOST(t *testing.T) {
	query := "mutation addAuthor($author: [AddAuthorInput!]!) {\n  addAuthor(input: $author) {\n    author {\n      id\n      name\n    }\n  }\n}\n"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "mutation",
			"query": query,
			"variables": map[string]interface{}{
				"author": map[string]interface{}{
					"name":  "A.N. Author",
					"dob":   "2000-01-01",
					"posts": []interface{}{},
				},
			},
		}, func(b *config.Backend) {
			b.ParentEndpointMethod = "POST"
		}),
	)

	expectedResponse := &Response{Data: map[string]interface{}{"foo": "bar"}}
	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.Method != "POST" {
			t.Errorf("unexpected method: %s", req.Method)
		}
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		var request graphql.GraphQLRequest
		if err := json.Unmarshal(b, &request); err != nil {
			return nil, err
		}
		if request.Query != query {
			t.Errorf("unexpected query: %s", request.Query)
		}
		return expectedResponse, nil
	})

	resp, err := prxy(context.Background(), &Request{
		Body: io.NopCloser(strings.NewReader(`{
			"name": "foo",
			"dob": "bar"
		}`)),
		Params:  map[string]string{},
		Headers: map[string][]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, expectedResponse) {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestNewGraphQLMiddleware_queryGET(t *testing.T) {
	query := "{ q(func: uid(1)) { uid } }"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "query",
			"query": query,
			"variables": map[string]interface{}{
				"name":  "{foo}",
				"dob":   "{bar}",
				"posts": []interface{}{},
			},
		}, func(b *config.Backend) {
			b.Method = "GET"
		}),
	)

	expectedResponse := &Response{Data: map[string]interface{}{"foo": "bar"}}
	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.Method != "GET" {
			t.Errorf("unexpected method: %s", req.Method)
		}
		request := graphql.GraphQLRequest{
			Query:     req.Query.Get("query"),
			Variables: map[string]interface{}{},
		}
		_ = json.Unmarshal([]byte(req.Query.Get("variables")), &request.Variables)

		if request.Query != query {
			t.Errorf("unexpected query: %s", request.Query)
		}
		if v, ok := request.Variables["name"].(string); !ok || v != "foo" {
			t.Errorf("unexpected var name: %v", request.Variables["name"])
		}
		if v, ok := request.Variables["dob"].(string); !ok || v != "bar" {
			t.Errorf("unexpected var dob: %v", request.Variables["dob"])
		}
		return expectedResponse, nil
	})

	resp, err := prxy(context.Background(), &Request{
		Params: map[string]string{
			"Foo": "foo",
			"Bar": "bar",
		},
		Body:    io.NopCloser(strings.NewReader("ignored")),
		Headers: map[string][]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, expectedResponse) {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestNewGraphQLMiddleware_queryPOST(t *testing.T) {
	query := "query Hero($episode: Episode!) { hero(episode: $episode) { name } }"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "query",
			"query": query,
			"variables": map[string]interface{}{
				"episode": "{Episode}",
			},
			"operationName": "Hero",
		}, func(b *config.Backend) {
			b.ParentEndpointMethod = "POST"
		}),
	)

	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.Method != "POST" {
			t.Errorf("unexpected method: %s", req.Method)
		}
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		var request graphql.GraphQLRequest
		if err := json.Unmarshal(b, &request); err != nil {
			return nil, err
		}
		if request.Query != query {
			t.Errorf("unexpected query: %s", request.Query)
		}
		if request.OperationName != "Hero" {
			t.Errorf("unexpected operationName: %s", request.OperationName)
		}
		if v, ok := request.Variables["episode"].(string); !ok || v != "JEDI" {
			t.Errorf("unexpected episode: %v", request.Variables["episode"])
		}
		return &Response{Data: map[string]interface{}{"ok": true}}, nil
	})

	_, err := prxy(context.Background(), &Request{
		Params: map[string]string{"Episode": "JEDI"},
		Body:   io.NopCloser(strings.NewReader(`{"ignored": true}`)),
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewGraphQLMiddleware_mutationGET(t *testing.T) {
	query := "mutation CreateReview($review: ReviewInput!) { createReview(review: $review) { stars } }"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "mutation",
			"query": query,
			"variables": map[string]interface{}{
				"review": map[string]interface{}{
					"stars": 3,
				},
			},
		}, func(b *config.Backend) {
			b.Method = "GET"
		}),
	)

	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.Method != "GET" {
			t.Errorf("unexpected method: %s", req.Method)
		}
		q, err := url.ParseQuery(req.Query.Encode())
		if err != nil {
			return nil, err
		}
		if q.Get("query") != query {
			t.Errorf("unexpected query: %s", q.Get("query"))
		}
		var vars map[string]interface{}
		if err := json.Unmarshal([]byte(q.Get("variables")), &vars); err != nil {
			return nil, err
		}
		if stars, ok := vars["stars"].(float64); !ok || stars != 5 {
			t.Errorf("unexpected stars from body: %v", vars["stars"])
		}
		return &Response{Data: map[string]interface{}{"ok": true}}, nil
	})

	_, err := prxy(context.Background(), &Request{
		Body: io.NopCloser(strings.NewReader(`{"stars": 5}`)),
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewGraphQLMiddleware_queryGET_mergesURL(t *testing.T) {
	query := "{ ping }"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "query",
			"query": query,
		}, func(b *config.Backend) {
			b.Method = "GET"
		}),
	)

	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.URL == nil || !strings.Contains(req.URL.RawQuery, "query=") {
			t.Fatalf("graphql query not merged into URL: %v", req.URL)
		}
		return &Response{Data: map[string]interface{}{"ok": true}}, nil
	})

	_, err := prxy(context.Background(), &Request{
		URL:     &url.URL{Scheme: "http", Host: "127.0.0.1:8081", Path: "/graphql", RawQuery: "dump_body=1"},
		Headers: map[string][]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewGraphQLMiddleware_queryGET_noDuplicateQuery(t *testing.T) {
	query := "{ ping }"
	mw := NewGraphQLMiddleware(
		logging.NoOp,
		graphqlBackend(map[string]interface{}{
			"type":  "query",
			"query": query,
		}, func(b *config.Backend) {
			b.Method = "GET"
		}),
	)

	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		if req.Query == nil {
			t.Fatal("expected query values")
		}
		if got := req.Query["query"]; len(got) != 1 || got[0] != query {
			t.Fatalf("unexpected query values: %v", got)
		}
		return &Response{Data: map[string]interface{}{"ok": true}}, nil
	})

	_, err := prxy(context.Background(), &Request{
		Query: url.Values{"query": []string{"stale"}},
		URL:   &url.URL{Scheme: "http", Host: "127.0.0.1:8081", Path: "/graphql", RawQuery: "query=stale"},
		Headers: map[string][]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewGraphQLMiddleware_noConfigPassthrough(t *testing.T) {
	mw := NewGraphQLMiddleware(logging.NoOp, &config.Backend{})
	prxy := mw(func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Data: map[string]interface{}{"passthrough": true}}, nil
	})
	resp, err := prxy(context.Background(), &Request{
		Method:  "POST",
		Headers: map[string][]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data["passthrough"] != true {
		t.Errorf("expected passthrough, got %v", resp.Data)
	}
}

func TestResolveGraphQLHTTPMethod_priority(t *testing.T) {
	opt := &graphql.Options{Method: graphql.MethodPost}
	remote := &config.Backend{
		Method:               "GET",
		ParentEndpointMethod: "POST",
		ExtraConfig: config.ExtraConfig{
			graphql.Namespace: map[string]interface{}{
				"method": "get",
			},
		},
	}
	if got := resolveGraphQLHTTPMethod(opt, remote); got != graphql.MethodGet {
		t.Errorf("extra_config.method: got %s want GET", got)
	}

	remote.ExtraConfig = config.ExtraConfig{}
	if got := resolveGraphQLHTTPMethod(opt, remote); got != graphql.MethodGet {
		t.Errorf("backend.method: got %s want GET", got)
	}

	remote.Method = ""
	if got := resolveGraphQLHTTPMethod(opt, remote); got != graphql.MethodPost {
		t.Errorf("endpoint.method: got %s want POST", got)
	}
}

func TestExtraConfigAlias_graphqlNamespace(t *testing.T) {
	extra := config.ExtraConfig{
		"backend/graphql": map[string]interface{}{
			"type":  "query",
			"query": "{ ping }",
		},
	}
	config.ExtraConfigAlias["backend/graphql"] = graphql.Namespace
	extra.Normalize()
	if _, ok := extra[graphql.Namespace]; !ok {
		t.Fatal("expected normalized namespace key")
	}
	delete(config.ExtraConfigAlias, "backend/graphql")
}
