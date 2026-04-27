package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type ShutdownFunc func(ctx context.Context) error

// InitTracing initializes a global OpenTelemetry tracer provider.
//
// It is intentionally lightweight and opt-in:
// - Disabled unless FREERANGE_OTEL_ENABLED=true
// - Exports traces via OTLP gRPC to FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT
//
// Environment variables (all optional):
// - FREERANGE_OTEL_ENABLED=true|false
// - FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
// - FREERANGE_OTEL_SAMPLE_RATIO=0.1
// - FREERANGE_OTEL_ENV=production|staging|dev
func InitTracing(ctx context.Context, serviceName, serviceVersion string) (ShutdownFunc, error) {
	if strings.ToLower(strings.TrimSpace(os.Getenv("FREERANGE_OTEL_ENABLED"))) != "true" {
		return func(context.Context) error { return nil }, nil
	}

	endpoint := strings.TrimSpace(os.Getenv("FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	ratio := 0.1
	if v := strings.TrimSpace(os.Getenv("FREERANGE_OTEL_SAMPLE_RATIO")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			ratio = f
		}
	}

	env := strings.TrimSpace(os.Getenv("FREERANGE_OTEL_ENV"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("FREERANGE_APP_ENVIRONMENT"))
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("otel exporter init failed: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource init failed: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

