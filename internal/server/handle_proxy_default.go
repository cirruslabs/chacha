package server

import (
	"context"
	"errors"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/kv"
	"github.com/cirruslabs/chacha/internal/server/responder"
	rulepkg "github.com/cirruslabs/chacha/internal/server/rule"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"io"
	"net/http"
	"net/url"
	"strings"
)

//nolint:cyclop,funlen // not sure if chopping this function will make the matters easier
func (server *Server) handleProxyDefault(writer http.ResponseWriter, request *http.Request) responder.Responder {
	// From http.Request's URL field documentation:
	//
	// >For most requests, fields other than Path and RawQuery will be empty.
	//
	// So, we fix this by adding fields necessary to perform the upstream request.
	if request.TLS != nil {
		request.URL.Scheme = "https"
	} else {
		request.URL.Scheme = "http"
	}

	if request.Host != "" {
		request.URL.Host = request.Host
	} else {
		return responder.NewCodef(http.StatusBadRequest, "Host header is empty")
	}

	// Determine our caching policy for this request
	rule := server.rules.Get(request.URL.String())

	// Determine the cache key
	key := server.cacheKey(request, rule)

	// Prevent multiple in-flight proxy requests to the same key,
	// otherwise we may needlessly fetch the content twice
	server.kmutex.Lock(key)
	defer server.kmutex.Unlock(key)

	// Determine the cache implementation to use
	//
	// If we're in a cluster, it could be that another node should store contents for this key
	cache := server.cache(key)

	// Acquire a handle to the cache entry, if any, because we
	// need the ETag first to provide it to the origin server
	var cacheEntryReader io.ReadCloser
	var metadata cachepkg.Metadata
	var err error

	cacheEntryReader, metadata, err = cache.Get(request.Context(), key)
	if err != nil && !errors.Is(err, cachepkg.ErrNotFound) {
		if kv, ok := cache.(*kv.KV); ok {
			return responder.NewCodef(http.StatusBadGateway, "failed to retrieve cache entry "+
				"for key %q: cluster node %s is not available: %v", key, kv.Node(), err)
		}

		return responder.NewCodef(http.StatusInternalServerError, "failed to retrieve cache entry "+
			"for key %q: %v", key, err)
	}
	if cacheEntryReader != nil {
		defer func() {
			_ = cacheEntryReader.Close()
		}()
	}

	// Always perform an upstream request in order to guarantee that
	// the requestor still has access to the upstream resource
	//
	// According to RFC 9110 "HTTP Semantics", §13.2.1 "When to Evaluate",
	// this should be safe even when making conditional requests:
	//
	// Except when excluded below, a recipient cache or origin server MUST evaluate
	// received request preconditions after it has successfully performed its normal
	// request checks and just before it would process the request content (if any)
	// or perform the action associated with the request method. A server MUST ignore
	// all received preconditions if its response to the same request without those
	// conditions, prior to processing the request content, would have been a status
	// code other than a 2xx (Successful) or 412 (Precondition Failed). In other words,
	// redirects and failures that can be detected before significant processing occurs
	// take precedence over the evaluation of preconditions.
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc9110#section-13.2.1
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, request.URL.String(),
		request.Body)
	if err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "failed to create an upstream request: %v",
			err)
	}

	// Try to make the request conditional to save bandwidth and time
	if eTag := metadata.ETag; eTag != "" {
		upstreamRequest.Header.Set("If-None-Match", eTag)
	}

	// Propagate our request headers to the upstream's request
	for key, values := range request.Header {
		for _, value := range values {
			upstreamRequest.Header.Set(key, value)
		}
	}

	// Remove end-to-end headers
	removeEndToEndHeaders(upstreamRequest.Header)

	server.logger.Debugf("upstream request: %+v", upstreamRequest)

	// Perform an upstream request
	upstreamResponse, err := server.httpClient.Do(upstreamRequest)
	if err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "failed to perform a request "+
			"to the upstream: %v", err)
	}
	defer upstreamResponse.Body.Close()

	// Propagate upstream's response headers to our response
	for key, values := range upstreamResponse.Header {
		for _, value := range values {
			writer.Header().Set(key, value)
		}
	}

	server.logger.Debugf("upstream response: %v", upstreamResponse)

	switch {
	case upstreamResponse.StatusCode == http.StatusOK && server.shouldCache(request, upstreamResponse, rule):
		// Our cache entry is outdated and caching is allowed, refresh cache entry contents
		teeReader := io.TeeReader(upstreamResponse.Body, writer)

		err = cache.Put(request.Context(), key, cachepkg.Metadata{
			ETag: upstreamResponse.Header.Get("ETag"),
		}, teeReader)
		if err != nil {
			return responder.NewCodef(http.StatusInternalServerError, "failed to create a cache entry "+
				"for key %q: %v", key, err)
		}

		// Metrics
		//nolint:contextcheck // can's use request.Context() here because it might be canceled
		server.cacheOperationCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("type", "miss"),
		))

		//nolint:contextcheck // can's use request.Context() here because it might be canceled
		server.cacheTransferCounter.Add(context.Background(), upstreamResponse.ContentLength, metric.WithAttributes(
			attribute.String("type", "miss"),
		))

		return responder.NewEmptyf("fetched from the upstream, cache entry is outdated")
	case upstreamResponse.StatusCode == http.StatusNotModified && cacheEntryReader != nil:
		// Our cached entry is up-to-date, return cache entry contents

		writer.WriteHeader(http.StatusOK)

		n, err := io.Copy(writer, cacheEntryReader)
		if err != nil {
			return responder.NewCodef(http.StatusInternalServerError, "failed to write all data "+
				"to the client: %v", err)
		}

		// Metrics
		//nolint:contextcheck // can's use request.Context() here because it might be canceled
		server.cacheOperationCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("type", "hit"),
		))

		//nolint:contextcheck // can's use request.Context() here because it might be canceled
		server.cacheTransferCounter.Add(context.Background(), n, metric.WithAttributes(
			attribute.String("type", "hit"),
		))

		return responder.NewEmptyf("retrieved from the cache")
	default:
		// Caching is not allowed
		writer.WriteHeader(upstreamResponse.StatusCode)

		if _, err := io.Copy(writer, upstreamResponse.Body); err != nil {
			return responder.NewCodef(http.StatusInternalServerError, "failed to write all data "+
				"to the client: %v", err)
		}

		// Metrics
		//nolint:contextcheck // can's use request.Context() here because it might be canceled
		server.cacheOperationCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("type", "not-allowed"),
		))

		return responder.NewEmptyf("fetched from the upstream, caching is not allowed")
	}
}

func (server *Server) cacheKey(request *http.Request, rule *rulepkg.Rule) string {
	scheme := "http"

	if request.TLS != nil {
		scheme = "https"
	}

	query := request.URL.Query()

	if rule != nil {
		for _, ignoredParameter := range rule.IgnoredParameters() {
			query.Del(ignoredParameter)
		}
	}

	cacheURL := url.URL{
		Scheme:   scheme,
		Host:     request.Host,
		Path:     request.URL.Path,
		RawQuery: query.Encode(),
	}

	return cacheURL.String()
}

func (server *Server) cache(key string) cachepkg.Cache {
	if cluster := server.cluster; cluster != nil {
		if targetNode := cluster.TargetNode(key); targetNode != cluster.LocalNode() {
			var kvOpts []kv.Option

			if server.localNetworkHelper != nil {
				httpClient := &http.Client{
					Transport: &http.Transport{
						DialContext: server.localNetworkHelper.PrivilegedDialContext,
					},
				}

				kvOpts = append(kvOpts, kv.WithHTTPClient(httpClient))
			}

			return kv.New(targetNode, cluster.Secret(), kvOpts...)
		}
	}

	return server.disk
}

func (server *Server) shouldCache(request *http.Request, response *http.Response, rule *rulepkg.Rule) bool {
	if rule == nil {
		return false
	}

	if request.Method != http.MethodGet {
		return false
	}

	if !isCacheableCacheControl(request.Header.Values("Cache-Control")) {
		return false
	}

	if strings.Contains(request.Header.Get("Pragma"), "no-cache") {
		return false
	}

	if !isCacheableCacheControl(response.Header.Values("Cache-Control")) {
		return false
	}

	if request.Header.Get("Authorization") != "" &&
		!cacheControlExplicitlyAllows(response.Header.Values("Cache-Control")) &&
		!rule.IgnoreAuthorizationHeader() {
		return false
	}

	// A Vary header field-value of "*" always fails to match.[1]
	//
	// However, since the Vary header is almost never used in our use-case,
	// we simplify things and always fail to match when this header is set.
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc7234#section-4.1
	if len(response.Header.Values("Vary")) != 0 {
		return false
	}

	if response.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func isCacheableCacheControl(cacheControls []string) bool {
	if headersContainDirective(cacheControls, "no-store") {
		return false
	}

	if headersContainDirective(cacheControls, "private") {
		return false
	}

	return true
}

func cacheControlExplicitlyAllows(cacheControlValues []string) bool {
	if headersContainDirective(cacheControlValues, "must-revalidate") {
		return true
	}

	if headersContainDirective(cacheControlValues, "public") {
		return true
	}

	if headersContainDirective(cacheControlValues, "s-maxage") {
		return true
	}

	return false
}

func headersContainDirective(headers []string, directive string) bool {
	for _, header := range headers {
		headerDirectives := strings.Split(header, ",")

		for _, headerDirective := range headerDirectives {
			directiveKey := strings.Split(headerDirective, "=")[0]

			if strings.EqualFold(strings.TrimSpace(directiveKey), directive) {
				return true
			}
		}
	}

	return false
}

func removeEndToEndHeaders(header http.Header) {
	// Remove Connection header as per RFC 9110, §7.6.1 "Connection"[1]
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc9110#section-7.6.1
	header.Del("Connection")

	// Remove hop-by-hop headers as per RFC 9110, §7.6.1 "Connection"[1]
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc9110#section-7.6.1
	header.Del("Proxy-Connection")
	header.Del("Keep-Alive")
	header.Del("TE")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	// Remove hop-by-hop headers as per RFC 9110, §11.7.1 "Proxy-Authenticate"[1]
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc9110#section-11.7.1
	header.Del("Proxy-Authenticate")

	// Remove hop-by-hop headers as per RFC 9110, §11.7.2 "Proxy-Authorization"[1]
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc9110#section-11.7.2
	header.Del("Proxy-Authorization")
}
