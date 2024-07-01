package ghacache

import (
	"errors"
	"fmt"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	authpkg "github.com/cirruslabs/chacha/internal/server/auth"
	"github.com/cirruslabs/chacha/internal/server/fail"
	"github.com/cirruslabs/chacha/internal/server/httprange"
	"github.com/cirruslabs/chacha/internal/server/rangetopart"
	"github.com/go-chi/render"
	"github.com/labstack/echo/v4"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/samber/lo"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type GHACache struct {
	baseURL     *url.URL
	remoteCache cachepkg.RemoteCache
	uploadables *xsync.MapOf[int64, *Uploadable]
}

type Uploadable struct {
	MultipartUpload cachepkg.MultipartUpload
	RangeToPart     *rangetopart.RangeToPart
}

type getResponse struct {
	Key string `json:"cacheKey"`
	URL string `json:"archiveLocation"`
}

type reserveUploadableRequest struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

type reserveUploadableResponse struct {
	CacheID int64 `json:"cacheId"`
}

func New(group *echo.Group, baseURL *url.URL, remoteCache cachepkg.RemoteCache) *GHACache {
	gha := &GHACache{
		baseURL:     baseURL,
		remoteCache: remoteCache,
		uploadables: xsync.NewMapOf[int64, *Uploadable](),
	}

	group.GET("/cache", gha.get)
	group.POST("/caches", gha.reserveUploadable)
	group.PATCH("/caches/:id", gha.updateUploadable)
	group.POST("/caches/:id", gha.commitUploadable)

	return &GHACache{}
}

func (cache *GHACache) get(c echo.Context) error {
	keys := strings.Split(c.Request().URL.Query().Get("keys"), ",")
	version := c.Request().URL.Query().Get("version")

	for _, cacheKeyPrefix := range authFromContext(c).CacheKeyPrefixes {
		for i, key := range keys {
			if key == "" {
				continue
			}

			keyWithoutPrefix := fmt.Sprintf("%s-%s", key, version)
			keyWithPrefix := cacheKeyPrefix + keyWithoutPrefix

			info, err := cache.remoteCache.Info(c.Request().Context(), keyWithPrefix, i == 0)
			if err != nil {
				if errors.Is(err, cachepkg.ErrNotFound) {
					continue
				}

				return fail.Fail(c, http.StatusInternalServerError, "%v", err)
			}

			downloadURL := cache.baseURL.JoinPath(keyWithoutPrefix)
			downloadURL.User = url.UserPassword("", authFromContext(c).Token)

			return c.JSON(http.StatusOK, &getResponse{
				Key: info.Key,
				URL: downloadURL.String(),
			})
		}
	}

	return c.NoContent(http.StatusNoContent)
}

func (cache *GHACache) reserveUploadable(c echo.Context) error {
	var request reserveUploadableRequest

	if err := render.DecodeJSON(c.Request().Body, &request); err != nil {
		return fail.Fail(c, http.StatusBadRequest, "GHA cache failed to read/decode the "+
			"JSON passed to the reserve uploadable endpoint: %v", err)
	}

	response := &reserveUploadableResponse{
		CacheID: generateID(),
	}

	entry, err := cache.remoteCache.Put(c.Request().Context(),
		keysFromContext(c, request.Key, request.Version)[0])
	if err != nil {
		return fail.Fail(c, http.StatusInternalServerError, "%v", err)
	}

	_, loaded := cache.uploadables.LoadOrStore(response.CacheID, &Uploadable{
		MultipartUpload: entry,
		RangeToPart:     rangetopart.New(),
	})
	if loaded {
		return fmt.Errorf("")
	}

	return c.JSON(http.StatusOK, &response)
}

func (cache *GHACache) updateUploadable(c echo.Context) error {
	id, ok := getID(c)
	if !ok {
		return fail.Fail(c, http.StatusBadRequest, "GHA cache failed to get/decode the "+
			"ID passed to the update uploadable endpoint")
	}

	uploadable, ok := cache.uploadables.Load(id)
	if !ok {
		return fail.Fail(c, http.StatusNotFound, "GHA cache failed to find an uploadable "+
			"with ID %d", id)
	}

	httpRanges, err := httprange.ParseRange(c.Request().Header.Get("Content-Range"), math.MaxInt64)
	if err != nil {
		return fail.Fail(c, http.StatusBadRequest, "failed to parse Content-Range header: %v", err)
	}

	if len(httpRanges) != 1 {
		return fail.Fail(c, http.StatusBadRequest, "expected exactly one Content-Range value, got %d",
			len(httpRanges))
	}

	partNumber, err := uploadable.RangeToPart.Tell(c.Request().Context(), httpRanges[0].Start, httpRanges[0].Length)
	if err != nil {
		return fail.Fail(c, http.StatusBadRequest, "%v", err)
	}

	return uploadable.MultipartUpload.UploadPart(c.Request().Context(), partNumber, c.Request().Body,
		httpRanges[0].Length)
}

func (cache *GHACache) commitUploadable(c echo.Context) error {
	id, ok := getID(c)
	if !ok {
		return fail.Fail(c, http.StatusBadRequest, "GHA cache failed to get/decode the "+
			"ID passed to the commit uploadable endpoint")
	}

	uploadable, ok := cache.uploadables.Load(id)
	if !ok {
		return fail.Fail(c, http.StatusNotFound, "GHA cache failed to find an uploadable "+
			"with ID %d", id)
	}
	defer cache.uploadables.Delete(id)

	var jsonReq struct {
		Size int64 `json:"size"`
	}

	if err := render.DecodeJSON(c.Request().Body, &jsonReq); err != nil {
		return fail.Fail(c, http.StatusBadRequest, "GHA cache failed to read/decode "+
			"the JSON passed to the commit uploadable endpoint: %v", err)
	}

	multipartUploadSize, err := uploadable.MultipartUpload.Size(c.Request().Context())
	if err != nil {
		return fail.Fail(c, http.StatusBadRequest, "%v", err)
	}

	if jsonReq.Size != multipartUploadSize {
		return fail.Fail(c, http.StatusBadRequest, "GHA cache detected a cache entry "+
			"size mismatch for uploadable %d: expected %d bytes, got %d bytes",
			id, multipartUploadSize, jsonReq.Size)
	}

	if err := uploadable.MultipartUpload.Commit(c.Request().Context()); err != nil {
		return fail.Fail(c, http.StatusInternalServerError, "GHA cache failed to "+
			"upload the uploadable with ID %d: %v", id, err)
	}

	return c.NoContent(http.StatusCreated)
}

func getID(c echo.Context) (int64, bool) {
	idRaw := c.Param("id")

	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

func authFromContext(c echo.Context) *authpkg.Auth {
	//nolint:forcetypeassert // the existence of authentication and its type is guaranteed by the middleware
	auth := c.Get(authpkg.ContextKey).(*authpkg.Auth)

	return auth
}

func keysFromContext(c echo.Context, key string, version string) []string {
	return lo.Map(authFromContext(c).CacheKeyPrefixes, func(cacheKeyPrefix string, _ int) string {
		return cacheKeyPrefix + fmt.Sprintf("%s-%s", key, version)
	})
}

func generateID() int64 {
	// JavaScript's Number is limited to 2^53-1[1], so clamp it <>
	//
	// [1]: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number/MAX_SAFE_INTEGER
	const jsNumberMaxSafeInteger = 9007199254740991

	//nolint:gosec // it's not that important to use a secure entropy source rather than to avoid collisions
	return rand.Int63n(jsNumberMaxSafeInteger)
}
