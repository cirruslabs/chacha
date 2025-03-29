package localnetworkhelper_test

import (
	"context"
	"github.com/cirruslabs/chacha/pkg/localnetworkhelper"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
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

	respExample, err := httpClient.Get("https://example.com/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respExample.StatusCode)

	respCirrus, err := httpClient.Get("https://cirrus-ci.org/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respCirrus.StatusCode)

	exampleBytes, err := io.ReadAll(respExample.Body)
	require.NoError(t, err)
	require.Contains(t, string(exampleBytes), "Example Domain")

	cirrusBytes, err := io.ReadAll(respCirrus.Body)
	require.NoError(t, err)
	require.Contains(t, string(cirrusBytes), "Cirrus CI")
}
