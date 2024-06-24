package server_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache/disk"
	"github.com/cirruslabs/chacha/internal/cache/s3"
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/cirruslabs/chacha/internal/server"
	"github.com/cirruslabs/chacha/internal/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/stretchr/testify/require"
	actionscache "github.com/tonistiigi/go-actions-cache"
	"go.uber.org/zap"
	"net/http"
	"testing"
	"time"
)

func TestServerHTTPCacheProtocol(t *testing.T) {
	addr, token := testServerCommon(t)

	httpClient := http.Client{
		Transport: NewHeadersTransport(map[string]string{
			"Authorization": "Bearer " + token,
		}, http.DefaultTransport),
	}

	key := uuid.NewString()

	chachaServerEndpointURL := fmt.Sprintf("http://%s/%s", addr, key)

	// Load of a non-existing key yields HTTP 404
	resp, err := httpClient.Get(chachaServerEndpointURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Store of a non-existing key succeeds
	resp, err = httpClient.Post(chachaServerEndpointURL, "application/octet-stream",
		bytes.NewReader([]byte(key)))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Retrieval of an existing key succeeds
	resp, err = httpClient.Get(chachaServerEndpointURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestServerGHACacheProtocol(t *testing.T) {
	// Configure logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	zap.ReplaceGlobals(logger)

	ctx := context.Background()

	addr, token := testServerCommon(t)
	cache, err := actionscache.New(token, fmt.Sprintf("http://%s/", addr), actionscache.Opt{})
	require.NoError(t, err)

	// Load of a non-existing key yields nothing
	entry, err := cache.Load(ctx, "test")
	require.NoError(t, err)
	require.Nil(t, entry)

	// Store of a non-existing key succeeds
	require.NoError(t, cache.Save(ctx, "test", actionscache.NewBlob([]byte("Hello, World!"))))

	// Retrieval of an existing key succeeds
	entry, err = cache.Load(ctx, "test")
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, entry.WriteTo(ctx, buf))
	require.Equal(t, "Hello, World!", buf.String())
}

func testServerCommon(t *testing.T) (string, string) {
	t.Helper()

	// Start an OIDC server
	oidcServer, err := mockoidc.Run()
	require.NoError(t, err)

	// Start Chacha server
	oidcProviders := []config.OIDCProvider{
		{
			URL:           oidcServer.Issuer(),
			CacheKeyExprs: []string{`"mock-"`},
		},
	}

	localCache, err := disk.New(t.TempDir(), 50*1024*1024*1024)
	require.NoError(t, err)

	remoteCache, err := s3.NewFromConfig(context.Background(), testutil.S3(t))
	require.NoError(t, err)

	chachaServer, err := server.New(":0", nil, oidcProviders, localCache, remoteCache)
	require.NoError(t, err)

	go func() {
		if err := chachaServer.Run(context.Background()); err != nil {
			panic(err)
		}
	}()

	token, err := oidcServer.Keypair.SignJWT(jwt.MapClaims{
		"iss": oidcServer.Issuer(),
		"nbf": &jwt.NumericDate{Time: time.Now().Add(-time.Hour)},
		"exp": &jwt.NumericDate{Time: time.Now().Add(time.Hour)},
		// Needed by the github.com/tonistiigi/go-actions-cache
		"ac": "[]",
	})
	require.NoError(t, err)

	return chachaServer.Addr(), token
}

type HeadersTransport struct {
	headers   map[string]string
	transport http.RoundTripper
}

func NewHeadersTransport(headers map[string]string, transport http.RoundTripper) *HeadersTransport {
	return &HeadersTransport{
		headers:   headers,
		transport: transport,
	}
}

func (transport *HeadersTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	for key, value := range transport.headers {
		request.Header.Set(key, value)
	}

	return transport.transport.RoundTrip(request)
}
