package server_test

import (
	"cmp"
	"encoding/json"
	"fmt"
	diskpkg "github.com/cirruslabs/chacha/internal/cache/disk"
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/cirruslabs/chacha/internal/server"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"slices"
	"testing"
)

func TestCluster(t *testing.T) {
	secret := uuid.NewString()
	logger := zap.Must(zap.NewDevelopment())

	// Configure first Chacha node, in cluster, without a disk
	firstOpts := []server.Option{
		server.WithCluster(
			cluster.New(
				secret,
				"127.0.0.1:8081",
				[]config.Node{
					{
						Addr: "127.0.0.1:8082",
					},
				},
			),
		),
		server.WithLogger(logger.Sugar()),
	}

	firstAddr := chachaServerWithAddr(t, "127.0.0.1:8081", firstOpts...)

	firstURL, err := url.Parse(fmt.Sprintf("http://%s", firstAddr))
	require.NoError(t, err)

	// Configure second Chacha node, in cluster and with a disk
	secondDir := t.TempDir()

	secondDisk, err := diskpkg.New(secondDir, 1*humanize.GByte)
	require.NoError(t, err)

	secondOpts := []server.Option{
		server.WithDisk(secondDisk),
		server.WithCluster(
			cluster.New(
				secret,
				"127.0.0.1:8082",
				[]config.Node{
					{
						Addr: "127.0.0.1:8082",
					},
				},
			),
		),
		server.WithLogger(logger.Sugar()),
	}

	_ = chachaServerWithAddr(t, "127.0.0.1:8082", secondOpts...)

	// Perform the first request
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(firstURL),
		},
	}

	req, err := http.NewRequest(http.MethodGet, "http://httpbingo.org/cache?key=1", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Perform a second request
	req, err = http.NewRequest(http.MethodGet, "http://httpbingo.org/cache?key=2", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "identity")
	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Ensure that the second node cached both of the requested pages
	dirEntries, err := os.ReadDir(secondDir)
	require.NoError(t, err)
	require.Len(t, dirEntries, 2)

	type HTTPBingoResponseArgs struct {
		Key []string `json:"key"`
	}

	type HTTPBingoResponse struct {
		Args HTTPBingoResponseArgs `json:"args"`
	}

	var httpBingoResponses []HTTPBingoResponse

	err = secondDisk.Walk(func(cacheEntryReader fs.File, _ diskpkg.Info, err error) error {
		if err != nil {
			return err
		}

		var httpBingoResponse HTTPBingoResponse

		require.NoError(t, json.NewDecoder(cacheEntryReader).Decode(&httpBingoResponse))

		httpBingoResponses = append(httpBingoResponses, httpBingoResponse)

		require.NoError(t, cacheEntryReader.Close())

		return nil
	})
	require.NoError(t, err)

	slices.SortFunc(httpBingoResponses, func(a, b HTTPBingoResponse) int {
		return cmp.Compare(a.Args.Key[0], b.Args.Key[0])
	})

	require.Equal(t, []HTTPBingoResponse{
		{
			Args: HTTPBingoResponseArgs{
				Key: []string{"1"},
			},
		},
		{
			Args: HTTPBingoResponseArgs{
				Key: []string{"2"},
			},
		},
	}, httpBingoResponses)
}

func TestClusterUnavailableTimeout(t *testing.T) {
	secret := uuid.NewString()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// Configure only the first Chacha node, in cluster, without a disk
	firstOpts := []server.Option{
		server.WithCluster(
			cluster.New(
				secret,
				"127.0.0.1:8083",
				[]config.Node{
					{
						Addr: "127.0.0.1:8084",
					},
				},
			),
		),
		server.WithLogger(logger.Sugar()),
	}

	firstAddr := chachaServerWithAddr(t, "127.0.0.1:8083", firstOpts...)

	firstURL, err := url.Parse(fmt.Sprintf("http://%s", firstAddr))
	require.NoError(t, err)

	// Ensure that the request fails because we haven't configured the second Chacha node
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(firstURL),
		},
	}

	resp, err := httpClient.Get("http://example.com")
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}
