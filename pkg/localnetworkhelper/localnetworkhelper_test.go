package localnetworkhelper_test

import (
	"context"
	"github.com/cirruslabs/chacha/pkg/localnetworkhelper"
	"github.com/stretchr/testify/require"
	"net"
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "localnetworkhelper" {
		if err := localnetworkhelper.Serve(context.Background(), 3); err != nil {
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

	privilegedConn, err := localNetworkHelper.PrivilegedDialContext(ctx, "tcp", "example.com:443")
	require.NoError(t, err)
	require.IsType(t, &net.TCPConn{}, privilegedConn)

	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return privilegedConn, nil
			},
		},
	}

	resp, err := httpClient.Get("https://example.com/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
