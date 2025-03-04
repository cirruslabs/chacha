package cache

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("cache entry not found")

type Metadata struct {
	ETag string `json:"etag,omitempty"`
}

type Cache interface {
	Get(ctx context.Context, key string) (io.ReadCloser, Metadata, error)
	Put(ctx context.Context, key string, metadata Metadata, blobReader io.Reader) error
}
