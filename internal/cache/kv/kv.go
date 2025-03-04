package kv

import (
	"context"
	"fmt"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"io"
	"net/http"
)

type KV struct {
	node   string
	secret string
}

func New(node string, secret string) *KV {
	return &KV{
		node:   node,
		secret: secret,
	}
}

func (kv *KV) Node() string {
	return kv.node
}

func (kv *KV) Get(
	ctx context.Context,
	key string,
) (io.ReadCloser, cachepkg.Metadata, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, kv.url(), nil)
	if err != nil {
		return nil, cachepkg.Metadata{}, err
	}

	// Provide authorization
	if kv.secret != "" {
		request.SetBasicAuth("", kv.secret)
	}

	// Provide input parameters
	if err := SetKey(request.Header, key); err != nil {
		return nil, cachepkg.Metadata{}, err
	}

	// Perform request
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, cachepkg.Metadata{}, err
	}

	switch response.StatusCode {
	case http.StatusOK:
		// All good, continue
	case http.StatusNotFound:
		// Cache entry does not exist
		return nil, cachepkg.Metadata{}, cachepkg.ErrNotFound
	default:
		// Unexpected status code
		return nil, cachepkg.Metadata{}, fmt.Errorf("unexpected HTTP %d", response.StatusCode)
	}

	// Retrieve output parameters
	metadata, err := GetMetadata(response.Header)
	if err != nil {
		_ = response.Body.Close()

		return nil, cachepkg.Metadata{}, err
	}

	return response.Body, metadata, nil
}

func (kv *KV) Put(
	ctx context.Context,
	key string,
	metadata cachepkg.Metadata,
	blobReader io.Reader,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, kv.url(), blobReader)
	if err != nil {
		return err
	}

	// Provide authorization
	if kv.secret != "" {
		request.SetBasicAuth("", kv.secret)
	}

	// Provide input parameters
	if err := SetKey(request.Header, key); err != nil {
		return err
	}

	if err := SetMetadata(request.Header, metadata); err != nil {
		return err
	}

	// Perform request
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// Handle unexpected status code
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP %d", response.StatusCode)
	}

	return nil
}

func (kv *KV) url() string {
	return fmt.Sprintf("http://%s", kv.node)
}
