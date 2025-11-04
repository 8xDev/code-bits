package main

import (
	"context"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracer initializes the OpenTelemetry tracer provider
// It connects to an OTLP collector (e.g., otel-collector)
func InitTracer() (*sdktrace.TracerProvider, func(context.Context) error, error) {
	ctx := context.Background()

	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		slog.WarnContext(ctx, "OTEL_EXPORTER_OTLP_ENDPOINT not set, using default: localhost:4317")
		otelEndpoint = "localhost:4317"
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "go-app"
	}

	// Set up a connection to the OTLP collector
	conn, err := grpc.DialContext(ctx, otelEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, err
	}

	// Create a resource describing this service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("v1.0.0"),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Set up a batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	// Create a new tracer provider
	// We use AlwaysSample for demo purposes. In production, you'd use
	// ParentBased(TraceIDRatioBased(0.1)) or similar to sample 10% of traces.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	// Set the global propagators to trace context (W3C)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Shutdown function
	shutdown := func(ctx context.Context) error {
		// Shutdown the tracer provider
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		// Close the gRPC connection
		if err := conn.Close(); err != nil {
			return err
		}
		return nil
	}

	return tp, shutdown, nil

}
