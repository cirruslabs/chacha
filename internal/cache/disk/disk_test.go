package disk_test

import (
	"github.com/cirruslabs/chacha/internal/cache/disk"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSimple(t *testing.T) {
	cache, err := disk.New(t.TempDir(), 1*1024*1024)
	require.NoError(t, err)

	// Retrieval and deletion of a non-existent key should fail
	_, err = cache.Get("test")
	require.Error(t, err)

	require.Error(t, cache.Delete("test"))

	// Insertion of a non-existent key should succeed
	contentBytes := []byte("Hello, World!")
	pathToPut := filepath.Join(t.TempDir(), "test1.txt")
	require.NoError(t, os.WriteFile(pathToPut, contentBytes, 0600))

	require.NoError(t, cache.Put("test", pathToPut))

	// Retrieval of an existent key should succeed
	retrievalReader, err := cache.Get("test")
	require.NoError(t, err)

	retrievedContentBytes, err := io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, contentBytes, retrievedContentBytes)

	// Re-insertion of an existent key should succeed
	newContentsBytes := []byte("Bye bye!")
	pathToPut = filepath.Join(t.TempDir(), "test2.txt")
	require.NoError(t, os.WriteFile(pathToPut, newContentsBytes, 0600))

	require.NoError(t, cache.Put("test", pathToPut))

	// Retrieval of a re-inserted key should yield modified contents
	retrievalReader, err = cache.Get("test")
	require.NoError(t, err)

	retrievedContentBytes, err = io.ReadAll(retrievalReader)
	require.NoError(t, err)
	require.Equal(t, newContentsBytes, retrievedContentBytes)

	// Deletion of an existing key should succeed
	require.NoError(t, cache.Delete("test"))

	// Retrieval of a deleted key should fail
	_, err = cache.Get("test")
	require.Error(t, err)
}

func TestEvict(t *testing.T) {
	cache, err := disk.New(t.TempDir(), 5)
	require.NoError(t, err)

	// Eviction shouldn't occur if cache entries fit the budget
	tmpDir := t.TempDir()

	smallPathToPut1 := filepath.Join(tmpDir, "small1.txt")
	require.NoError(t, os.WriteFile(smallPathToPut1, []byte("ab"), 0600))
	require.NoError(t, cache.Put("small1", smallPathToPut1))

	smallPathToPut2 := filepath.Join(tmpDir, "small2.txt")
	require.NoError(t, os.WriteFile(smallPathToPut2, []byte("cde"), 0600))
	require.NoError(t, cache.Put("small2", smallPathToPut2))

	_, err = cache.Get("small1")
	require.NoError(t, err)

	_, err = cache.Get("small2")
	require.NoError(t, err)

	// Eviction should occur for oldest entry if the budget is violated
	smallPathToPut3 := filepath.Join(tmpDir, "small3.txt")
	require.NoError(t, os.WriteFile(smallPathToPut3, []byte("f"), 0600))
	require.NoError(t, cache.Put("small3", smallPathToPut3))

	_, err = cache.Get("small1")
	require.Error(t, err)

	_, err = cache.Get("small2")
	require.NoError(t, err)

	_, err = cache.Get("small3")
	require.NoError(t, err)
}

func TestSecure(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := disk.New(cacheDir, 1*1024*1024)
	require.NoError(t, err)

	// Ensure that insecure keys are percent-encoded
	pathToPut := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(pathToPut, []byte("doesn't matter"), 0600))
	require.NoError(t, cache.Put("../../../../../etc/passwd", pathToPut))

	dirEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)

	var dirEntryNames []string

	for _, entry := range dirEntries {
		dirEntryNames = append(dirEntryNames, entry.Name())
	}

	require.Equal(t, []string{"%2e%2e%2f%2e%2e%2f%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd"}, dirEntryNames)
}
