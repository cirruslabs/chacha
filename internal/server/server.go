package server

import (
	"context"
	"fmt"
	"github.com/alecthomas/units"
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/noop"
	"github.com/cirruslabs/chacha/internal/opentelemetry"
	"github.com/cirruslabs/chacha/internal/server/capturingresponsewriter"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	responderpkg "github.com/cirruslabs/chacha/internal/server/responder"
	"github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/cirruslabs/chacha/internal/server/tlsinterceptor"
	"github.com/cirruslabs/chacha/pkg/localnetworkhelper"
	"github.com/im7mortal/kmutex"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	listener           net.Listener
	httpServer         *http.Server
	internalHTTPClient *http.Client
	externalHTTPClient *http.Client
	kmutex             *kmutex.Kmutex
	logger             *zap.SugaredLogger

	disk               cachepkg.Cache
	tlsInterceptor     *tlsinterceptor.TLSInterceptor
	rules              rule.Rules
	cluster            *cluster.Cluster
	localNetworkHelper *localnetworkhelper.LocalNetworkHelper

	// Metrics
	requestsCounter       metric.Int64Counter
	cacheOperationCounter metric.Int64Counter
	cacheSpeedHistogram   metric.Int64Histogram
}

func New(addr string, opts ...Option) (*Server, error) {
	server := &Server{
		internalHTTPClient: http.DefaultClient,
		externalHTTPClient: &http.Client{
			Transport: &http.Transport{
				DisableCompression: true,
			},
		},
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
		Handler:           otelhttp.NewHandler(server, "http.request"),
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

	// Use a customized internal HTTP client when "Local Network" permission helper is enabled
	if server.localNetworkHelper != nil {
		server.internalHTTPClient = &http.Client{
			Transport: &http.Transport{
				DialContext: server.localNetworkHelper.PrivilegedDialContext,
			},
		}

		// We need this when using direct connect functionality
		// with direct connect header disabled
		server.externalHTTPClient = &http.Client{
			Transport: &http.Transport{
				DialContext: server.localNetworkHelper.PrivilegedDialContext,

				DisableCompression: true,
			},
		}
	}

	// Metrics
	server.requestsCounter, err = opentelemetry.DefaultMeter.Int64Counter("org.cirruslabs.chacha.requests.total")
	if err != nil {
		return nil, err
	}

	server.cacheOperationCounter, err = opentelemetry.DefaultMeter.Int64Counter(
		"org.cirruslabs.chacha.cache.operation_count",
	)
	if err != nil {
		return nil, err
	}

	server.cacheSpeedHistogram, err = opentelemetry.DefaultMeter.Int64Histogram(
		"org.cirruslabs.chacha.cache.speed_bytes_per_second",
		metric.WithExplicitBucketBoundaries(
			100*float64(units.Mega),
			500*float64(units.Mega),
			1*float64(units.Giga),
			2.5*float64(units.Giga),
			5*float64(units.Giga),
			7.5*float64(units.Giga),
			10*float64(units.Giga),
			15*float64(units.Giga),
			20*float64(units.Giga),
			25*float64(units.Giga),
			30*float64(units.Giga),
			35*float64(units.Giga),
			40*float64(units.Giga),
		),
	)
	if err != nil {
		return nil, err
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

	// Capture response writer's status code
	capturingResponseWriter := capturingresponsewriter.Wrap(writer)

	// Default responder
	var responder responderpkg.Responder

	responder = responderpkg.NewCodef(http.StatusNotFound, "not found")

	operation := "unknown"

	if request.Host == "" || request.Host == server.Addr() {
		switch request.Method {
		case http.MethodPut:
			responder = server.handleClusterPut(capturingResponseWriter, request)
			operation = "cluster-put"
		case http.MethodGet:
			switch request.URL.Path {
			case "/health":
				responder = responderpkg.NewCodef(http.StatusOK, "healthy")
				operation = "health-check"
			case "/direct-connect":
				responder = server.handleDirectConnectGet(capturingResponseWriter, request)
				operation = "direct-connect-get"
			default:
				responder = server.handleClusterGet(capturingResponseWriter, request)
				operation = "cluster-get"
			}
		}
	} else {
		switch request.Method {
		case http.MethodConnect:
			responder = server.handleProxyConnect(capturingResponseWriter, request)
			operation = "proxy-connect"
		default:
			responder = server.handleProxyDefault(capturingResponseWriter, request)
			operation = "proxy-default"
		}
	}

	responder.Respond(capturingResponseWriter, request)

	server.logger.With(
		"remote_addr", request.RemoteAddr,
		"method", request.Method,
		"status_code", capturingResponseWriter.StatusCode(),
		"operation", operation,
		"host", request.Host,
		"path", request.URL.Path,
	).Infof("%s", responder.Message())

	// Metrics
	//nolint:contextcheck // can's use request.Context() here because it might be canceled
	server.requestsCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", request.Method),
		attribute.Int("status_code", capturingResponseWriter.StatusCode()),
		attribute.String("operation", operation),
	))
}
