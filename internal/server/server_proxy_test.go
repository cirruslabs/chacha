package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/cirruslabs/chacha/internal/server"
	"github.com/cirruslabs/chacha/internal/server/tlsinterceptor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestHTTPProxying(t *testing.T) {
	logger := zap.Must(zap.NewDevelopment()).Sugar()

	addr := chachaServer(t, server.WithLogger(logger))

	chachaServerEndpointURL, err := url.Parse(fmt.Sprintf("http://%s", addr))
	require.NoError(t, err)

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(chachaServerEndpointURL),
		},
	}

	resp, err := httpClient.Get("http://example.com")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestHTTPSProxying(t *testing.T) {
	// Generate a self-signed certificate for the TLS interceptor
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Cirrus Labs, Inc."},
			CommonName:   "Chacha Proxy Server",
		},
		Issuer: pkix.Name{
			Organization: []string{"Cirrus Labs, Inc."},
			CommonName:   "Chacha Proxy Server",
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().Add(10 * time.Minute),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(certificateDER)
	require.NoError(t, err)

	// Run Chacha with the TLS interceptor
	logger := zap.Must(zap.NewDevelopment()).Sugar()

	tlsInterceptor, err := tlsinterceptor.New(certificate, key)
	require.NoError(t, err)

	addr := chachaServer(t, server.WithTLSInterceptor(tlsInterceptor), server.WithLogger(logger))

	chachaServerEndpointURL, err := url.Parse(fmt.Sprintf("http://%s", addr))
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(certificate)

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(chachaServerEndpointURL),
			//nolint:gosec // gosec yields a false-positive here (G402), telling us that "TLS MinVersion too low"
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	resp, err := httpClient.Get("https://example.com")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Contains(t, string(bodyBytes), "Example Domain")
}

func chachaServer(t *testing.T, opts ...server.Option) string {
	t.Helper()

	return chachaServerWithAddr(t, ":0", opts...)
}

func chachaServerWithAddr(t *testing.T, addr string, opts ...server.Option) string {
	t.Helper()

	// Start Chacha server
	chachaServer, err := server.New(addr, opts...)
	require.NoError(t, err)

	go func() {
		if err := chachaServer.Run(context.Background()); err != nil {
			panic(err)
		}
	}()

	return chachaServer.Addr()
}
