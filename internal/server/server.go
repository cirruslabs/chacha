package server

import (
	"context"
	"fmt"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/noop"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	responderpkg "github.com/cirruslabs/chacha/internal/server/responder"
	"github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/cirruslabs/chacha/internal/server/tlsinterceptor"
	"github.com/im7mortal/kmutex"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	listener   net.Listener
	httpServer *http.Server
	kmutex     *kmutex.Kmutex
	logger     *zap.SugaredLogger

	disk           cachepkg.Cache
	tlsInterceptor *tlsinterceptor.TLSInterceptor
	rules          rule.Rules
	cluster        *cluster.Cluster
}

func New(addr string, opts ...Option) (*Server, error) {
	server := &Server{
		kmutex: kmutex.New(),
	}

	// Listen on the desired port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	server.listener = listener

	// Configure HTTP server
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	// Apply defaults
	if server.disk == nil {
		server.disk = noop.New()
	}

	if server.logger == nil {
		server.logger = zap.NewNop().Sugar()
	}

	// Sanity check
	if server.cluster != nil {
		rawIP, rawPort, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("addr %q doesn't seem to be fully-qualified: %w", addr, err)
		}

		ip := net.ParseIP(rawIP)
		if ip == nil || ip.IsUnspecified() {
			return nil, fmt.Errorf("IP address in addr %q cannot be empty or unspecified when using cluster mode",
				addr)
		}

		port, err := strconv.Atoi(rawPort)
		if err != nil {
			return nil, fmt.Errorf("failed to parse port in addr %q: %w", addr, err)
		}

		if port == 0 {
			return nil, fmt.Errorf("port in addr %q cannot be zero when using cluster mode", addr)
		}
	}

	return server, nil
}

func (server *Server) Addr() string {
	return strings.ReplaceAll(server.listener.Addr().String(), "[::]", "127.0.0.1")
}

func (server *Server) Run(ctx context.Context) error {
	server.logger.Infof("listening on %s", server.Addr())

	go func() {
		<-ctx.Done()

		_ = server.httpServer.Close()
	}()

	return server.httpServer.Serve(server.listener)
}

func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	server.logger.Debugf("request: %+v", request)

	// Default responder
	var responder responderpkg.Responder

	responder = responderpkg.NewCodef(http.StatusNotFound, "not found")

	if request.Host == "" || request.Host == server.Addr() {
		switch request.Method {
		case http.MethodPut:
			responder = server.handleClusterPut(writer, request)
		case http.MethodGet:
			responder = server.handleClusterGet(writer, request)
		}
	} else {
		switch request.Method {
		case http.MethodConnect:
			responder = server.handleProxyConnect(writer, request)
		default:
			responder = server.handleProxyDefault(writer, request)
		}
	}

	responder.Respond(writer, request)

	server.logger.With(
		"remote_addr", request.RemoteAddr,
		"method", request.Method,
		"host", request.Host,
		"path", request.URL.Path,
	).Infof("%s", responder.Message())
}
