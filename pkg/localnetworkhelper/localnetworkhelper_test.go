//go:build darwin

package localnetworkhelper_test

import (
	"context"
	"github.com/cirruslabs/chacha/pkg/localnetworkhelper"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "localnetworkhelper" {
		if err := localnetworkhelper.Serve(3); err != nil {
			panic(err)
		}

		os.Exit(0)
	} else {
		os.Exit(m.Run())
	}
}

func TestLocalNetworkHelper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localNetworkHelper, err := localnetworkhelper.New(ctx, "", "chacha-*")
	require.NoError(t, err)

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				privilegedConn, err := localNetworkHelper.PrivilegedDialContext(ctx, network, addr)
				require.NoError(t, err)
				require.IsType(t, &net.TCPConn{}, privilegedConn)

				return privilegedConn, nil
			},
		},
	}

	var wg sync.WaitGroup

	wg.Add(3)

	go func() {
		defer wg.Done()

		respExample, err := httpClient.Get("https://example.com/")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respExample.StatusCode)

		exampleBytes, err := io.ReadAll(respExample.Body)
		require.NoError(t, err)
		require.Contains(t, string(exampleBytes), "Example Domain")
	}()

	go func() {
		defer wg.Done()

		respCirrus, err := httpClient.Get("https://cirrus-ci.org/")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respCirrus.StatusCode)

		cirrusBytes, err := io.ReadAll(respCirrus.Body)
		require.NoError(t, err)
		require.Contains(t, string(cirrusBytes), "Cirrus CI")
	}()

	go func() {
		defer wg.Done()

		respCirrus, err := httpClient.Get("https://cirrus-runners.app/")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respCirrus.StatusCode)

		cirrusBytes, err := io.ReadAll(respCirrus.Body)
		require.NoError(t, err)
		require.Contains(t, string(cirrusBytes), "Cirrus Runners")
	}()

	wg.Wait()

	require.NoError(t, localNetworkHelper.Close())
}

func TestLocalNetworkHelperRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localNetworkHelper, err := localnetworkhelper.New(ctx, "", "chacha-*")
	require.NoError(t, err)

	boundedCtx, boundedCtxCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer boundedCtxCancel()

	startedAt := time.Now()

	// Try to connect to an address in non-routable TEST-NET-1[1]
	//
	// [1]: https://datatracker.ietf.org/doc/html/rfc5737#section-3
	_, err = localNetworkHelper.PrivilegedDialContext(boundedCtx, "tcp", "192.0.2.1:80")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.InDelta(t, 3.0, time.Since(startedAt).Seconds(), 0.5)

	require.NoError(t, localNetworkHelper.Close())
}
