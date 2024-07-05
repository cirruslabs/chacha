package box_test

import (
	"github.com/cirruslabs/chacha/internal/server/box"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSimple(t *testing.T) {
	manager, err := box.NewManager()
	require.NoError(t, err)

	expectedBox := box.Box{
		CacheKeyPrefix: uuid.NewString(),
	}

	sealedBox, err := manager.Seal(expectedBox)
	require.NoError(t, err)

	unsealedBox, err := manager.Unseal(sealedBox)
	require.NoError(t, err)

	require.Equal(t, expectedBox, unsealedBox)
}
