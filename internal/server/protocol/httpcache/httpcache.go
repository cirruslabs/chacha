package httpcache

import (
	"bytes"
	"errors"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	authpkg "github.com/cirruslabs/chacha/internal/server/auth"
	"github.com/cirruslabs/chacha/internal/server/fail"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type HTTPCache struct {
	localCache  cachepkg.LocalCache
	remoteCache cachepkg.RemoteCache
}

func New(group *echo.Group, localCache cachepkg.LocalCache, remoteCache cachepkg.RemoteCache) *HTTPCache {
	cache := &HTTPCache{
		localCache:  localCache,
		remoteCache: remoteCache,
	}

	group.GET("/*", cache.get)
	group.HEAD("/*", cache.head)
	group.POST("/*", cache.put)
	group.PUT("/*", cache.put)
	group.DELETE("/*", cache.delete)

	return cache
}

func (cache *HTTPCache) get(c echo.Context) error {
	for _, key := range keysFromContext(c) {
		// Try local cache first
		cacheReadCloser, err := cache.localCache.Get(key)
		if err == nil {
			defer func() {
				_ = cacheReadCloser.Close()
			}()

			return c.Stream(http.StatusOK, "application/octet-stream", cacheReadCloser)
		}

		// Fallback to remote cache if not found in the local cache
		if err != nil && errors.Is(err, cachepkg.ErrNotFound) {
			cacheReadCloser, err = cache.remoteCache.Get(c.Request().Context(), key)
		}

		if err != nil {
			if errors.Is(err, cachepkg.ErrNotFound) {
				continue
			}

			return backendErrorToFail(c, key, "retrieve", err)
		}
		defer func() {
			_ = cacheReadCloser.Close()
		}()

		// Create a temporary file that we'll populate from the remote cache
		// and the Put() into the local cache
		tmpFile, err := os.CreateTemp("", "")
		if err != nil {
			return err
		}

		teeReader := io.TeeReader(cacheReadCloser, tmpFile)

		if err := c.Stream(http.StatusOK, "application/octet-stream", teeReader); err != nil {
			return err
		}

		if err := cache.localCache.Put(key, tmpFile.Name()); err != nil {
			return err
		}

		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())

		return nil
	}

	return c.NoContent(http.StatusNotFound)
}

func (cache *HTTPCache) head(c echo.Context) error {
	for _, key := range keysFromContext(c) {
		info, err := cache.remoteCache.Info(c.Request().Context(), key, true)
		if err != nil {
			if errors.Is(err, cachepkg.ErrNotFound) {
				continue
			}

			return backendErrorToFail(c, key, "retrieve", err)
		}

		c.Response().Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))

		return c.NoContent(http.StatusOK)
	}

	return c.NoContent(http.StatusNotFound)
}

func (cache *HTTPCache) put(c echo.Context) error {
	key := keysFromContext(c)[0]

	multipartUpload, err := cache.remoteCache.Put(c.Request().Context(), key)
	if err != nil {
		return fail.Fail(c, http.StatusInternalServerError, "failed to initiate multipart upload "+
			"of the cache entry for key %q: %v", key, err)
	}

	buf := make([]byte, 8*1024*1024)
	partNumber := int32(1)

	for {
		n, readFullErr := io.ReadFull(c.Request().Body, buf)
		if readFullErr != nil && !(errors.Is(readFullErr, io.ErrUnexpectedEOF) || errors.Is(readFullErr, io.EOF)) {
			return fail.Fail(c, http.StatusInternalServerError, "failed to read data to be uploaded "+
				"for cache key %q: %v", key, readFullErr)
		}

		err = multipartUpload.UploadPart(c.Request().Context(), partNumber, bytes.NewReader(buf[:n]), int64(n))
		if err != nil {
			return err
		}

		if errors.Is(readFullErr, io.EOF) || errors.Is(readFullErr, io.ErrUnexpectedEOF) {
			break
		}

		partNumber++
	}

	if err := multipartUpload.Commit(c.Request().Context()); err != nil {
		return err
	}

	return c.NoContent(http.StatusCreated)
}

func (cache *HTTPCache) delete(c echo.Context) error {
	key := keysFromContext(c)[0]

	if err := cache.localCache.Delete(key); err != nil {
		return backendErrorToFail(c, key, "delete", err)
	}

	if err := cache.remoteCache.Delete(c.Request().Context(), key); err != nil {
		return backendErrorToFail(c, key, "delete", err)
	}

	return c.NoContent(http.StatusOK)
}

func keysFromContext(c echo.Context) []string {
	//nolint:forcetypeassert // the existence of authentication and its type is guaranteed by the middleware
	auth := c.Get(authpkg.ContextKey).(*authpkg.Auth)

	return lo.Map(auth.CacheKeyPrefixes, func(cacheKeyPrefix string, _ int) string {
		return cacheKeyPrefix + strings.TrimPrefix(c.Request().URL.Path, "/")
	})
}

func backendErrorToFail(c echo.Context, key string, operation string, err error) error {
	if errors.Is(err, cachepkg.ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}

	return fail.Fail(c, http.StatusInternalServerError, "failed to %s cache entry for key %q: %v",
		operation, key, err)
}
