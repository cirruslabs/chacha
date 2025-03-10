package noop_test

import (
	"bytes"
	"context"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGet(t *testing.T) {
	ctx := context.Background()
	key := uuid.NewString()

	noop := noop.New()

	// Retrieval from no-op cache should return ErrNotFound
	_, _, err := noop.Get(ctx, key)
	require.ErrorIs(t, err, cachepkg.ErrNotFound)

	// ...even after a Put()
	err = noop.Put(ctx, key, cachepkg.Metadata{ETag: uuid.NewString()}, bytes.NewReader([]byte("Hello, World!")))
	require.NoError(t, err)

	_, _, err = noop.Get(ctx, key)
	require.ErrorIs(t, err, cachepkg.ErrNotFound)
}

func TestPut(t *testing.T) {
	ctx := context.Background()
	key := uuid.NewString()

	noop := noop.New()

	// Put() to a no-nop cache should read everything from the io.Reader that we pass to it
	buf := bytes.NewBufferString("Hello, World!")
	require.NotEmpty(t, buf.String())

	err := noop.Put(ctx, key, cachepkg.Metadata{ETag: uuid.NewString()}, buf)
	require.NoError(t, err)

	require.Empty(t, buf.String())
}
