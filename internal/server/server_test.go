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
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	// Start an OIDC server
	oidcServer, err := mockoidc.Run()
	require.NoError(t, err)

	// Start Chacha server
	oidcProviders := []config.OIDCProvider{
		{
			URL:           oidcServer.Issuer(),
			CacheKeyExprs: []string{`claims.iss + "-"`},
		},
	}

	localCache, err := disk.New(t.TempDir(), 50*1024*1024*1024)
	require.NoError(t, err)

	remoteCache, err := s3.NewFromConfig(context.Background(), testutil.S3(t))
	require.NoError(t, err)

	chachaServer, err := server.New(":0", oidcProviders, localCache, remoteCache)
	require.NoError(t, err)

	go func() {
		err := chachaServer.Run(context.Background())
		fmt.Println(err)
	}()

	token, err := oidcServer.Keypair.SignJWT(jwt.RegisteredClaims{
		Issuer:    oidcServer.Issuer(),
		IssuedAt:  &jwt.NumericDate{Time: time.Now().Add(-time.Hour)},
		ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour)},
	})
	require.NoError(t, err)

	httpClient := http.Client{
		Transport: NewHeadersTransport(map[string]string{
			"Authorization": "Bearer " + token,
		}, http.DefaultTransport),
	}

	key := uuid.NewString()

	chachaServerEndpointURL := fmt.Sprintf("http://%s/%s", chachaServer.Addr(), key)

	resp, err := httpClient.Get(chachaServerEndpointURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = httpClient.Post(chachaServerEndpointURL, "application/octet-stream",
		bytes.NewReader([]byte(key)))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = httpClient.Get(chachaServerEndpointURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
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
