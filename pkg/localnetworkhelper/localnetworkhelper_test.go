package localnetworkhelper_test

import (
	"context"
	"github.com/cirruslabs/chacha/pkg/localnetworkhelper"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
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

	localNetworkHelper, err := localnetworkhelper.New(ctx)
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

func TestLocalNetworkHelperReadsAcrossSocketBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	helper, err := localnetworkhelper.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = helper.Close() })

	// Force the request across the old 4 KiB read boundary. An unknown network
	// returns a short dial error, so only the request spans multiple reads.
	_, err = helper.PrivilegedDialContext(ctx, "unknown", strings.Repeat("x", 8192))
	require.ErrorContains(t, err, "unknown network unknown")

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	conn, err := helper.PrivilegedDialContext(ctx, "tcp4", listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestLocalNetworkHelperRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localNetworkHelper, err := localnetworkhelper.New(ctx)
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
