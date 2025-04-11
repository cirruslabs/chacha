package server

import (
	"context"
	"errors"
	"fmt"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/server/responder"
	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"io"
	"net/http"
	"time"
)

const (
	jwtIssuerChacha          = "chacha"
	jwtAudienceDirectConnect = "direct-connect"

	jwtValidity = 10 * time.Minute
	jwtLeeway   = 10 * time.Second
)

func (server *Server) handleDirectConnectGet(writer http.ResponseWriter, request *http.Request) responder.Responder {
	// Perform authentication
	key, authResponder := server.performDirectConnectAuth(request)
	if authResponder != nil {
		return authResponder
	}

	// Read cache entry from the local disk
	cacheEntryReader, _, err := server.disk.Get(request.Context(), key)
	if err != nil {
		if errors.Is(err, cachepkg.ErrNotFound) {
			return responder.NewCodef(http.StatusNotFound, "no cache entry found for key %s", key)
		}

		return responder.NewCodef(http.StatusInternalServerError, "failed to get cache entry for key %s: %v",
			key, err)
	}

	// Write cache entry to the requester
	copyStartAt := time.Now()

	n, err := io.Copy(writer, cacheEntryReader)
	if err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "unable to write cache entry: %v", err)
	}

	// Metrics
	//nolint:contextcheck // can's use request.Context() here because it might be canceled
	server.cacheOperationCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("type", "direct-connect"),
	))

	bytesPerSecond := float64(n) / max(time.Since(copyStartAt).Seconds(), 1)

	//nolint:contextcheck // can's use request.Context() here because it might be canceled
	server.cacheSpeedHistogram.Record(context.Background(), int64(bytesPerSecond), metric.WithAttributes(
		attribute.String("type", "direct-connect"),
	))

	if err := cacheEntryReader.Close(); err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "unable to close cache entry: %v", err)
	}

	return responder.NewEmptyf("cache entry read successfully")
}

func (server *Server) generateDirectConnectToken(key string) (string, error) {
	if server.cluster == nil {
		return "", fmt.Errorf("direct connect token requested, but cluster is not initialized")
	}

	now := time.Now()

	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    jwtIssuerChacha,
		Audience:  []string{jwtAudienceDirectConnect},
		Subject:   key,
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(jwtValidity)),
	}).SignedString([]byte(server.cluster.Secret()))
}

func (server *Server) performDirectConnectAuth(request *http.Request) (string, responder.Responder) {
	if server.cluster == nil {
		return "", responder.NewCodef(http.StatusNotFound, "direct connect request received, "+
			"but cluster mode is not configured")
	}

	token := request.URL.Query().Get("token")

	if token == "" {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is missing "+
			"or is empty")
	}

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(server.cluster.Secret()), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithLeeway(jwtLeeway))
	if err != nil {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: %v", err)
	}

	issuer, err := parsedToken.Claims.GetIssuer()
	if err != nil {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: "+
			"missing issuer")
	}
	if issuer != jwtIssuerChacha {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: "+
			"invalid issuer")
	}

	audience, err := parsedToken.Claims.GetAudience()
	if err != nil {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: "+
			"missing audience")
	}
	if !lo.Contains(audience, jwtAudienceDirectConnect) {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: "+
			"mismatched audience")
	}

	subject, err := parsedToken.Claims.GetSubject()
	if err != nil {
		return "", responder.NewCodef(http.StatusUnauthorized, "direct connect token is invalid: "+
			"missing subject")
	}

	return subject, nil
}
