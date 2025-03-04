package kv_test

import (
	"github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/kv"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"testing/quick"
)

func TestGetKeyMissing(t *testing.T) {
	header := http.Header{}

	_, err := kv.GetKey(header)
	require.Error(t, err)
}

func TestGetMetadataMissing(t *testing.T) {
	header := http.Header{}

	_, err := kv.GetMetadata(header)
	require.Error(t, err)
}

func TestSetGetKey(t *testing.T) {
	require.NoError(t, quick.Check(func(expectedKey string) bool {
		// We don't support empty keys
		if expectedKey == "" {
			return true
		}

		header := http.Header{}

		require.NoError(t, kv.SetKey(header, expectedKey))

		actualKey, err := kv.GetKey(header)
		require.NoError(t, err)

		return expectedKey == actualKey
	}, &quick.Config{
		MaxCount: 100_000,
	}))
}

func TestSetGetMetadata(t *testing.T) {
	require.NoError(t, quick.Check(func(expectedMetadata cache.Metadata) bool {
		header := http.Header{}

		require.NoError(t, kv.SetMetadata(header, expectedMetadata))

		actualMetadata, err := kv.GetMetadata(header)
		require.NoError(t, err)

		return expectedMetadata == actualMetadata
	}, &quick.Config{
		MaxCount: 100_000,
	}))
}
