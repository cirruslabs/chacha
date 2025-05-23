package main

import (
	"context"
	"fmt"
	"github.com/cirruslabs/chacha/internal/command"
	"github.com/cirruslabs/chacha/internal/logginglevel"
	"github.com/cirruslabs/chacha/internal/opentelemetry"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if !mainImpl() {
		os.Exit(1)
	}
}

func mainImpl() bool {
	// Set up a signal-interruptible context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize OpenTelemetry
	opentelemetryCore, opentelemetryDeinit, err := opentelemetry.Init(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize OpenTelemetry: %v", err)

		return false
	}
	defer opentelemetryDeinit()

	// Initialize logger
	cfg := zap.NewProductionConfig()
	cfg.Level = logginglevel.Level
	logger, err := cfg.Build(zap.WrapCore(func(originalCore zapcore.Core) zapcore.Core {
		return zapcore.NewTee(originalCore, opentelemetryCore)
	}))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		return false
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Replace zap.L() and zap.S() to avoid
	// propagating the *zap.Logger by hand
	zap.ReplaceGlobals(logger)

	if err := command.NewRootCommand().ExecuteContext(ctx); err != nil {
		logger.Sugar().Error(err)

		return false
	}

	return true
}
