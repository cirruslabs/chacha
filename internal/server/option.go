package server

import (
	cachepkg "github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	"github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/cirruslabs/chacha/internal/server/tlsinterceptor"
	"go.uber.org/zap"
)

type Option func(server *Server)

func WithDisk(disk cachepkg.Cache) Option {
	return func(server *Server) {
		server.disk = disk
	}
}

func WithTLSInterceptor(tlsInterceptor *tlsinterceptor.TLSInterceptor) Option {
	return func(server *Server) {
		server.tlsInterceptor = tlsInterceptor
	}
}

func WithRules(rules rule.Rules) Option {
	return func(server *Server) {
		server.rules = rules
	}
}

func WithCluster(cluster *cluster.Cluster) Option {
	return func(server *Server) {
		server.cluster = cluster
	}
}

func WithLogger(logger *zap.SugaredLogger) Option {
	return func(server *Server) {
		server.logger = logger
	}
}
