package cache

import (
	"context"
	"errors"
	"io"
)

var ErrNotFound = errors.New("cache entry not found")

type RemoteCache interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Put(ctx context.Context, key string) (MultipartUpload, error)
	Info(ctx context.Context, key string, exact bool) (*Info, error)
	Delete(ctx context.Context, key string) error
}

type MultipartUpload interface {
	UploadPart(ctx context.Context, number int32, r io.Reader) error
	Size(ctx context.Context) (int64, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Info struct {
	Key  string
	Size int64
}

type LocalCache interface {
	Get(key string) (io.ReadCloser, error)
	Put(key string, path string) error
	Delete(key string) error
}
