package s3_test

import (
	"bytes"
	"context"
	"github.com/cirruslabs/chacha/internal/cache/s3"
	"github.com/cirruslabs/chacha/internal/testutil"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestSimple(t *testing.T) {
	ctx := context.Background()

	cache, err := s3.NewFromConfig(ctx, testutil.S3(t))
	require.NoError(t, err)

	// Retrieval and deletion of a non-existent key should fail
	_, err = cache.Get(ctx, "test")
	require.Error(t, err)

	require.NoError(t, cache.Delete(ctx, "test"))

	// Insertion of a non-existent key should succeed
	contentBytes := []byte("Hello, World!")

	multipartUpload, err := cache.Put(ctx, "test")
	require.NoError(t, err)

	require.NoError(t, multipartUpload.UploadPart(ctx, 1, bytes.NewReader(contentBytes)))

	size, err := multipartUpload.Size(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(len(contentBytes)), size)

	require.NoError(t, multipartUpload.Commit(ctx))

	// Retrieval of an existent key should succeed
	retrievalReader, err := cache.Get(ctx, "test")
	require.NoError(t, err)

	retrievedContentBytes, err := io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, contentBytes, retrievedContentBytes)

	// Re-insertion of an existent key should succeed
	newContentsBytes := []byte("Bye bye!")
	multipartUpload, err = cache.Put(ctx, "test")
	require.NoError(t, err)
	require.NoError(t, multipartUpload.UploadPart(ctx, 1, bytes.NewReader(newContentsBytes)))
	require.NoError(t, multipartUpload.Commit(ctx))

	// Retrieval of a re-inserted key should yield modified contents
	retrievalReader, err = cache.Get(ctx, "test")
	require.NoError(t, err)

	retrievedContentBytes, err = io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, newContentsBytes, retrievedContentBytes)

	// Deletion of an existing key should succeed
	require.NoError(t, cache.Delete(ctx, "test"))

	// Retrieval of a deleted key should fail
	_, err = cache.Get(ctx, "test")
	require.Error(t, err)
}
