package server

import (
	"crypto/subtle"
	"errors"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/kv"
	"github.com/cirruslabs/chacha/internal/server/responder"
	"io"
	"net/http"
)

func (server *Server) handleClusterGet(writer http.ResponseWriter, request *http.Request) responder.Responder {
	// Perform authentication
	if responder := server.performClusterAuth(request); responder != nil {
		return responder
	}

	// Retrieve input parameters
	key, err := kv.GetKey(request.Header)
	if err != nil {
		return responder.NewCodef(http.StatusBadRequest, "failed to determine the key: %v", err)
	}

	// Read cache entry from the local disk
	cacheEntryReader, metadata, err := server.disk.Get(request.Context(), key)
	if err != nil {
		if errors.Is(err, cachepkg.ErrNotFound) {
			return responder.NewCodef(http.StatusNotFound, "no cache entry found for key %s", key)
		}

		return responder.NewCodef(http.StatusInternalServerError, "failed to get cache entry for key %s: %v",
			key, err)
	}

	// Expose cache entry metadata to the requester via HTTP headers
	if err := kv.SetMetadata(writer.Header(), metadata); err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "failed to provide the metadata "+
			"to the requester: %v", err)
	}

	// Write cache entry to the requester
	if _, err := io.Copy(writer, cacheEntryReader); err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "unable to write cache entry: %v", err)
	}

	if err := cacheEntryReader.Close(); err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "unable to close cache entry: %v", err)
	}

	return responder.NewEmptyf("cache entry read successfully")
}

func (server *Server) handleClusterPut(_ http.ResponseWriter, request *http.Request) responder.Responder {
	// Perform authentication
	if responder := server.performClusterAuth(request); responder != nil {
		return responder
	}

	// Retrieve input parameters
	key, err := kv.GetKey(request.Header)
	if err != nil {
		return responder.NewCodef(http.StatusBadRequest, "failed to determine the key: %v", err)
	}

	metadata, err := kv.GetMetadata(request.Header)
	if err != nil {
		return responder.NewCodef(http.StatusBadRequest, "failed to determine the metadata: %v", err)
	}

	// Write cache entry to the local disk
	if err := server.disk.Put(request.Context(), key, metadata, request.Body); err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "unable to put cache entry: %v", err)
	}

	return responder.NewCodef(http.StatusOK, "cache entry written successfully")
}

func (server *Server) performClusterAuth(request *http.Request) responder.Responder {
	if server.cluster == nil {
		return responder.NewCodef(http.StatusNotFound, "KV request received, "+
			"but cluster mode is not configured")
	}

	_, providedToken, ok := request.BasicAuth()
	if !ok {
		return responder.NewCodef(http.StatusUnauthorized, "failed to get basic auth")
	}

	if subtle.ConstantTimeCompare([]byte(server.cluster.Secret()), []byte(providedToken)) != 1 {
		return responder.NewCodef(http.StatusUnauthorized, "invalid secret")
	}

	return nil
}
