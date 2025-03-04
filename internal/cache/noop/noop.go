package noop

import (
	"context"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"io"
)

type NoOp struct{}

func New() *NoOp {
	return &NoOp{}
}

func (noop *NoOp) Get(_ context.Context, _ string) (io.ReadCloser, cachepkg.Metadata, error) {
	return nil, cachepkg.Metadata{}, cachepkg.ErrNotFound
}

func (noop *NoOp) Put(_ context.Context, _ string, _ cachepkg.Metadata, blobReader io.Reader) error {
	_, err := io.Copy(io.Discard, blobReader)

	return err
}
