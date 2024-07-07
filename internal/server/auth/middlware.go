package auth

import (
	"context"
	"github.com/cirruslabs/chacha/internal/server/box"
	"github.com/cirruslabs/chacha/internal/server/fail"
	providerpkg "github.com/cirruslabs/chacha/internal/server/provider"
	"github.com/expr-lang/expr"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

const ContextKey = "auth"

type Auth struct {
	CacheKeyPrefixes []string
}

func Middleware(issToProvider map[string]*providerpkg.Provider, boxManager *box.Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try Authorization: Bearer <token>
			token, found := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
			if found {
				if err := authenticate(c, issToProvider, token); err != nil {
					return err
				}

				return next(c)
			}

			// Try "token" query parameter
			if sealedBox := c.QueryParam("token"); sealedBox != "" {
				unsealedBox, err := boxManager.Unseal(sealedBox)
				if err != nil {
					return fail.Fail(c, http.StatusUnauthorized, "failed to validate the provided token: %v",
						err)
				}

				c.Set(ContextKey, &Auth{
					CacheKeyPrefixes: []string{
						unsealedBox.CacheKeyPrefix,
					},
				})

				return next(c)
			}

			return fail.Fail(c, http.StatusUnauthorized, "no JWT token nor a query "+
				"parameter \"token\" was present")
		}
	}
}

func authenticate(c echo.Context, issToProvider map[string]*providerpkg.Provider, rawToken string) error {
	parsedJWT, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{
		jose.RS256,
	})
	if err != nil {
		return fail.Fail(c, http.StatusUnauthorized, "failed to parse JWT token: %v", err)
	}

	preClaims := struct {
		Iss string `json:"iss"`
	}{}

	if err := parsedJWT.UnsafeClaimsWithoutVerification(&preClaims); err != nil {
		return fail.Fail(c, http.StatusUnauthorized, "failed to get JWT token claims: %v", err)
	}

	entity, ok := issToProvider[preClaims.Iss]
	if !ok {
		return fail.Fail(c, http.StatusUnauthorized, "no OIDC provider registered "+
			"that can handle issuer %q", preClaims.Iss)
	}

	token, err := entity.Verifier.Verify(context.Background(), rawToken)
	if err != nil {
		return fail.Fail(c, http.StatusUnauthorized, "failed to verify JWT token: %v", err)
	}

	var claims map[string]any

	if err := token.Claims(&claims); err != nil {
		return fail.Fail(c, http.StatusUnauthorized, "failed to get JWT token claims: %v", err)
	}

	env := map[string]any{
		"claims": claims,
	}

	auth := &Auth{}

	for idx, cacheKeyProgram := range entity.CacheKeyPrograms {
		cacheKeyPrefix, err := expr.Run(cacheKeyProgram, env)
		if err != nil {
			return fail.Fail(c, http.StatusInternalServerError,
				"failed to calculate the cache key prefix: %v", err)
		}

		cacheKeyPrefixString, ok := cacheKeyPrefix.(string)
		if !ok {
			return fail.Fail(c, http.StatusInternalServerError, "cache key prefix expression "+
				"%d should've evaluated to string, got %T instead", idx, cacheKeyPrefix)
		}

		auth.CacheKeyPrefixes = append(auth.CacheKeyPrefixes, cacheKeyPrefixString)
	}

	c.Set(ContextKey, auth)

	return nil
}
