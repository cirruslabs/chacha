package kv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache"
	"net/http"
)

const (
	HeaderKey      = "X-Chacha-Key"
	HeaderMetadata = "X-Chacha-Metadata"
)

//nolint:gochecknoglobals // yes, it's a global variable that helps to mismatched encoding/decoding options
var encoding = base64.StdEncoding

func GetKey(header http.Header) (string, error) {
	keyBytes, err := decode(header, HeaderKey)
	if err != nil {
		return "", err
	}

	return string(keyBytes), nil
}

func SetKey(header http.Header, key string) error {
	keyBytes := []byte(key)

	header.Set(HeaderKey, encoding.EncodeToString(keyBytes))

	return nil
}

func GetMetadata(header http.Header) (cache.Metadata, error) {
	metadataBytes, err := decode(header, HeaderMetadata)
	if err != nil {
		return cache.Metadata{}, err
	}

	var metadata cache.Metadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return cache.Metadata{}, fmt.Errorf("unable to unmarshal metadata: %w", err)
	}

	return metadata, nil
}

func SetMetadata(header http.Header, metadata cache.Metadata) error {
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		return err
	}

	header.Set(HeaderMetadata, encoding.EncodeToString(metadataBytes))

	return nil
}

func decode(header http.Header, key string) ([]byte, error) {
	headerValueRaw := header.Get(key)
	if headerValueRaw == "" {
		return nil, fmt.Errorf("no %s header is found or it is empty", key)
	}

	headerValueBytes, err := encoding.DecodeString(headerValueRaw)
	if err != nil {
		return nil, fmt.Errorf("unable to decode %s header contents: %w", key, err)
	}

	return headerValueBytes, nil
}
