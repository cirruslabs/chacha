package disk_test

import (
	"bytes"
	"context"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/disk"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func TestSimple(t *testing.T) {
	ctx := context.Background()

	cache, err := disk.New(t.TempDir(), 1*1024*1024)
	require.NoError(t, err)

	// Retrieval and deletion of a non-existent key should fail
	_, _, err = cache.Get(ctx, "test")
	require.ErrorIs(t, err, cachepkg.ErrNotFound)

	err = cache.Delete("test")
	require.ErrorIs(t, err, cachepkg.ErrNotFound)

	// Insertion of a non-existent key should succeed
	contentBytes := []byte("Hello, World!")
	eTag := uuid.NewString()

	err = cache.Put(ctx, "test", cachepkg.Metadata{
		ETag: eTag,
	}, bytes.NewReader(contentBytes))
	require.NoError(t, err)

	// Retrieval of an existent key should succeed
	retrievalReader, _, err := cache.Get(ctx, "test")
	require.NoError(t, err)

	retrievedContentBytes, err := io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, contentBytes, retrievedContentBytes)

	// Re-insertion of an existent key should succeed
	newContentsBytes := []byte("Bye bye!")

	err = cache.Put(ctx, "test", cachepkg.Metadata{}, bytes.NewReader(newContentsBytes))
	require.NoError(t, err)

	// Retrieval of a re-inserted key should yield modified contents
	retrievalReader, _, err = cache.Get(ctx, "test")
	require.NoError(t, err)

	retrievedContentBytes, err = io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, newContentsBytes, retrievedContentBytes)

	// Deletion of an existing key should succeed
	require.NoError(t, cache.Delete("test"))

	// Retrieval of a deleted key should fail
	_, _, err = cache.Get(ctx, "test")
	require.ErrorIs(t, err, cachepkg.ErrNotFound)
}

func TestEvict(t *testing.T) {
	ctx := context.Background()

	cache, err := disk.New(t.TempDir(), 768)
	require.NoError(t, err)

	// Eviction shouldn't occur if cache entries fit the budget
	err = cache.Put(ctx, "small1", cachepkg.Metadata{}, bytes.NewReader([]byte("ab")))
	require.NoError(t, err)

	err = cache.Put(ctx, "small2", cachepkg.Metadata{}, bytes.NewReader([]byte("cde")))
	require.NoError(t, err)

	_, _, err = cache.Get(ctx, "small1")
	require.NoError(t, err)

	_, _, err = cache.Get(ctx, "small2")
	require.NoError(t, err)

	// Eviction should occur for oldest entry if the budget is violated
	err = cache.Put(ctx, "small3", cachepkg.Metadata{}, bytes.NewReader([]byte("f")))
	require.NoError(t, err)

	_, _, err = cache.Get(ctx, "small1")
	require.ErrorIs(t, err, cachepkg.ErrNotFound)

	_, _, err = cache.Get(ctx, "small2")
	require.NoError(t, err)

	_, _, err = cache.Get(ctx, "small3")
	require.NoError(t, err)
}

func TestSecure(t *testing.T) {
	ctx := context.Background()

	cacheDir := t.TempDir()
	cache, err := disk.New(cacheDir, 1*1024*1024)
	require.NoError(t, err)

	// Ensure that insecure keys are percent-encoded
	err = cache.Put(ctx, "../../../../../etc/passwd", cachepkg.Metadata{},
		bytes.NewReader([]byte("doesn't matter")))
	require.NoError(t, err)

	dirEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)

	var dirEntryNames []string

	for _, entry := range dirEntries {
		dirEntryNames = append(dirEntryNames, entry.Name())
	}

	require.Regexp(t, "[A-Za-z0-9]+", dirEntryNames)
}
