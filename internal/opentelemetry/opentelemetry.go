package opentelemetry

import (
	"context"
	"github.com/cirruslabs/chacha/internal/version"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap/zapcore"
	"os"
)

//nolint:gochecknoglobals // creating individual otel.{Meter,Tracer} instances makes little sense, so we have this
var (
	DefaultMeter  = otel.Meter("")
	DefaultTracer = otel.Tracer("")
)

func Init(ctx context.Context) (zapcore.Core, func(), error) {
	// Avoid logging errors when local OpenTelemetry Collector is not available, for example:
	// "failed to upload metrics: [...]: dial tcp 127.0.0.1:4318: connect: connection refused"
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(_ error) {
		// do nothing
	}))

	// Work around https://github.com/open-telemetry/opentelemetry-go/issues/4834
	if _, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); !ok {
		if err := os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"); err != nil {
			return nil, nil, err
		}
	}

	// Provide Chacha-specific default resource attributes
	resource := sdkresource.Default()

	resource, err := sdkresource.Merge(resource, sdkresource.NewSchemaless(
		semconv.ServiceName("chacha"),
		semconv.ServiceVersion(version.Version),
	))
	if err != nil {
		return nil, nil, err
	}

	resource, err = sdkresource.Merge(resource, sdkresource.Environment())
	if err != nil {
		return nil, nil, err
	}

	// Metrics
	var finalizers []func()

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	finalizers = append(finalizers, func() {
		_ = meterProvider.Shutdown(ctx)
	})
	otel.SetMeterProvider(meterProvider)

	// Traces
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource),
		sdktrace.WithBatcher(traceExporter),
	)
	finalizers = append(finalizers, func() {
		_ = traceProvider.Shutdown(ctx)
	})
	otel.SetTracerProvider(traceProvider)

	// Enable context propagation via W3C Trace Context automatically
	// when using helpers like otelhttp.NewHandler() and manually
	// when calling otel.GetTextMapPropagator()'s methods
	// like Extract() and Inject()
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Logs
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(resource),
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(logExporter),
		),
	)
	finalizers = append(finalizers, func() {
		_ = logProvider.Shutdown(ctx)
	})
	global.SetLoggerProvider(logProvider)

	// zap → OpenTelemetry bridge
	opentelemetryCore := otelzap.NewCore("github.com/cirruslabs/chacha/internal/opentelemetry",
		otelzap.WithLoggerProvider(logProvider))

	return opentelemetryCore, func() {
		for _, finalizer := range finalizers {
			finalizer()
		}
	}, nil
}
