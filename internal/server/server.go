package server

import (
	"context"
	"github.com/brpaz/echozap"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	configpkg "github.com/cirruslabs/chacha/internal/config"
	"github.com/cirruslabs/chacha/internal/server/auth"
	"github.com/cirruslabs/chacha/internal/server/box"
	"github.com/cirruslabs/chacha/internal/server/protocol/ghacache"
	"github.com/cirruslabs/chacha/internal/server/protocol/httpcache"
	providerpkg "github.com/cirruslabs/chacha/internal/server/provider"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Server struct {
	listener      net.Listener
	httpServer    *http.Server
	issToProvider map[string]*providerpkg.Provider
	localCache    cachepkg.LocalCache
	remoteCache   cachepkg.RemoteCache
	boxManager    *box.Manager
}

func New(
	addr string,
	baseURL *url.URL,
	oidcProviders []configpkg.OIDCProvider,
	localCache cachepkg.LocalCache,
	remoteCache cachepkg.RemoteCache,
) (*Server, error) {
	server := &Server{
		issToProvider: map[string]*providerpkg.Provider{},
		localCache:    localCache,
		remoteCache:   remoteCache,
	}

	boxManager, err := box.NewManager()
	if err != nil {
		return nil, err
	}
	server.boxManager = boxManager

	for _, oidcProvider := range oidcProviders {
		provider, err := oidc.NewProvider(context.Background(), oidcProvider.URL)
		if err != nil {
			return nil, err
		}

		var cacheKeyProgs []*vm.Program

		for _, cacheKeyExpr := range oidcProvider.CacheKeyExprs {
			cacheKeyProg, err := expr.Compile(cacheKeyExpr)
			if err != nil {
				return nil, err
			}

			cacheKeyProgs = append(cacheKeyProgs, cacheKeyProg)
		}

		server.issToProvider[oidcProvider.URL] = &providerpkg.Provider{
			Verifier: provider.Verifier(&oidc.Config{
				SkipClientIDCheck: true,
			}),
			CacheKeyPrograms: cacheKeyProgs,
		}
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	server.listener = listener

	if baseURL == nil {
		baseURL = &url.URL{
			Scheme: "http",
			Host:   server.Addr(),
		}
	}

	// Configure HTTP server
	e := echo.New()

	e.Use(
		echozap.ZapLogger(zap.L()),
		auth.Middleware(server.issToProvider, boxManager),
	)

	// Serve HTTP cache protocol
	httpCacheGroup := e.Group("/*")
	httpcache.New(httpCacheGroup, server.localCache, server.remoteCache)

	// Serve GHA cache protocol
	ghaCacheGroup := e.Group("/_apis/artifactcache")
	ghacache.New(ghaCacheGroup, baseURL, server.remoteCache, server.boxManager)

	server.httpServer = &http.Server{
		Addr:              ":8080",
		Handler:           e,
		ReadHeaderTimeout: 30 * time.Second,
	}

	return server, nil
}

func (server *Server) Addr() string {
	return strings.ReplaceAll(server.listener.Addr().String(), "[::]", "localhost")
}

func (server *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()

		_ = server.httpServer.Close()
	}()

	return server.httpServer.Serve(server.listener)
}
