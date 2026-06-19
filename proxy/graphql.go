// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/pucora/lura/v2/config"
	"github.com/pucora/lura/v2/logging"
	"github.com/pucora/lura/v2/transport/http/client/graphql"
)

// NewGraphQLMiddleware returns a middleware with or without the GraphQL
// proxy wrapping the next element (depending on the configuration).
// It supports both queries and mutations.
// For queries, it completes the variables object using the request params.
// For mutations, it overides the defined variables with the request body.
// The resulting request will have a proper graphql body with the query and the
// variables
func NewGraphQLMiddleware(logger logging.Logger, remote *config.Backend) Middleware {
	opt, err := graphql.GetOptions(remote.ExtraConfig)
	if err != nil {
		if err != graphql.ErrNoConfigFound {
			logger.Warning(
				fmt.Sprintf("[BACKEND: %s %s -> %s][GraphQL] %s", remote.ParentEndpoint, remote.ParentEndpoint, remote.URLPattern, err.Error()))
		}
		return emptyMiddlewareFallback(logger)
	}

	method := resolveGraphQLHTTPMethod(opt, remote)

	extractor := graphql.New(*opt)
	var generateBodyFn func(*Request) ([]byte, error)
	var generateQueryFn func(*Request) (url.Values, error)

	switch opt.Type {
	case graphql.OperationMutation:
		generateBodyFn = func(req *Request) ([]byte, error) {
			if req.Body == nil {
				return extractor.BodyFromBody(strings.NewReader(""))
			}
			defer req.Body.Close()
			return extractor.BodyFromBody(req.Body)
		}
		generateQueryFn = func(req *Request) (url.Values, error) {
			if req.Body == nil {
				return extractor.QueryFromBody(strings.NewReader(""))
			}
			defer req.Body.Close()
			return extractor.QueryFromBody(req.Body)
		}

	case graphql.OperationQuery:
		generateBodyFn = func(req *Request) ([]byte, error) {
			return extractor.BodyFromParams(req.Params)
		}
		generateQueryFn = func(req *Request) (url.Values, error) {
			return extractor.QueryFromParams(req.Params)
		}

	default:
		return emptyMiddlewareFallback(logger)
	}

	return func(next ...Proxy) Proxy {
		if len(next) > 1 {
			logger.Fatal("too many proxies for this %s %s -> %s proxy middleware: NewGraphQLMiddleware only accepts 1 proxy, got %d",
				remote.ParentEndpointMethod, remote.ParentEndpoint, remote.URLPattern, len(next))
			return nil
		}

		logger.Debug(
			fmt.Sprintf(
				"[BACKEND: %s %s -> %s][GraphQL] Operation: %s, Method: %s",
				remote.ParentEndpointMethod,
				remote.ParentEndpoint,
				remote.URLPattern,
				opt.Type,
				method,
			),
		)

		if method == graphql.MethodGet {
			return func(ctx context.Context, req *Request) (*Response, error) {
				q, err := generateQueryFn(req)
				if err != nil {
					return nil, err
				}

				if req.Headers == nil {
					req.Headers = make(map[string][]string)
				}

				req.Body = io.NopCloser(bytes.NewReader([]byte{}))
				req.Method = string(method)
				req.Headers["Content-Length"] = []string{"0"}
				// even when there is no content, we just set the content-type
				// header to be safe if the server side checks it:
				req.Headers["Content-Type"] = []string{"application/json"}
				if req.Query == nil {
					req.Query = url.Values{}
				}
				for k, vs := range q {
					for _, v := range vs {
						req.Query.Set(k, v)
					}
				}
				if req.URL != nil {
					req.URL.RawQuery = req.Query.Encode()
				}

				return next[0](ctx, req)
			}
		}

		return func(ctx context.Context, req *Request) (*Response, error) {
			b, err := generateBodyFn(req)
			if err != nil {
				return nil, err
			}

			if req.Headers == nil {
				req.Headers = make(map[string][]string)
			}

			req.Body = io.NopCloser(bytes.NewReader(b))
			req.Method = string(method)
			req.Headers["Content-Length"] = []string{strconv.Itoa(len(b))}
			req.Headers["Content-Type"] = []string{"application/json"}

			return next[0](ctx, req)
		}
	}
}

// resolveGraphQLHTTPMethod picks the HTTP verb for the upstream GraphQL call.
// Priority: extra_config.method → backend.method → endpoint.method → POST.
func resolveGraphQLHTTPMethod(opt *graphql.Options, remote *config.Backend) graphql.OperationMethod {
	if tmp, ok := remote.ExtraConfig[graphql.Namespace]; ok {
		if extra, ok := tmp.(map[string]interface{}); ok {
			if m, ok := extra["method"].(string); ok && m != "" {
				return graphql.OperationMethod(strings.ToUpper(m))
			}
		}
	}
	if remote.Method != "" {
		return graphql.OperationMethod(strings.ToUpper(remote.Method))
	}
	if remote.ParentEndpointMethod != "" {
		return graphql.OperationMethod(strings.ToUpper(remote.ParentEndpointMethod))
	}
	if opt.Method == graphql.MethodGet {
		return graphql.MethodGet
	}
	return graphql.MethodPost
}
