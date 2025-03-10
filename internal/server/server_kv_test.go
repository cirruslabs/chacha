package server_test

import (
	"bytes"
	"context"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	diskpkg "github.com/cirruslabs/chacha/internal/cache/disk"
	kvpkg "github.com/cirruslabs/chacha/internal/cache/kv"
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/cirruslabs/chacha/internal/server"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestKV(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	secret := uuid.NewString()
	key := uuid.NewString()

	disk, err := diskpkg.New(dir, 1*humanize.GByte)
	require.NoError(t, err)

	opts := []server.Option{
		server.WithDisk(disk),
		server.WithCluster(
			cluster.New(
				secret,
				"127.0.0.1:8080",
				[]config.Node{
					{
						Addr: "127.0.0.1:8080",
					},
				},
			),
		),
	}

	addr := chachaServerWithAddr(t, "127.0.0.1:8080", opts...)

	kv := kvpkg.New(addr, secret)

	// Ensure that a request for a non-existent key returns an error
	_, _, err = kv.Get(ctx, key)
	require.ErrorIs(t, err, cachepkg.ErrNotFound)

	// Populate the key
	firstETag := uuid.NewString()
	firstMetadata := cachepkg.Metadata{ETag: firstETag}
	firstBlob := []byte("Hello, World!\n")

	err = kv.Put(ctx, key, firstMetadata, bytes.NewReader(firstBlob))
	require.NoError(t, err)

	// Ensure that a request for an existent key succeeds
	cacheEntryReader, actualMetadata, err := kv.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, firstMetadata, actualMetadata)

	actualBlob, err := io.ReadAll(cacheEntryReader)
	require.NoError(t, err)
	require.Equal(t, firstMetadata, actualMetadata)
	require.Equal(t, firstBlob, actualBlob)

	// Overwrite the key
	secondETag := uuid.NewString()
	secondMetadata := cachepkg.Metadata{ETag: secondETag}
	secondBlob := []byte("Goodbye, Cruel World!\n")

	err = kv.Put(ctx, key, secondMetadata, bytes.NewReader(secondBlob))
	require.NoError(t, err)

	// Ensure that the key got overwritten
	cacheEntryReader, actualMetadata, err = kv.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, secondMetadata, actualMetadata)

	actualBlob, err = io.ReadAll(cacheEntryReader)
	require.NoError(t, err)
	require.Equal(t, secondMetadata, actualMetadata)
	require.Equal(t, secondBlob, actualBlob)
}
