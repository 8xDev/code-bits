
package tracing

import (
	"context"
	"os"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
)

// NewTracerProvider creates and configures a new OpenTelemetry TracerProvider.
func NewTracerProvider(serviceName string) (*sdktrace.TracerProvider, error) {
	// The OTLP endpoint is the address of the OpenTelemetry Collector.
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		// Default to localhost if not set, for local development outside of Docker.
		otelEndpoint = "localhost:4317"
	}
	
	// Create a new gRPC exporter to send traces to the collector.
	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithInsecure(), // Use insecure connection for local demo.
		otlptracegrpc.WithEndpoint(otelEndpoint),
		otlptracegrpc.WithDialOption(grpc.WithBlock()), // Use blocking connection to ensure it's up.
	)
	if err != nil {
		return nil, err
	}
	
	// Create a resource to describe this service.
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)
	
	// Create a new TracerProvider with the exporter and resource.
	// We use a BatchSpanProcessor for better performance in production.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	
	// Set the global TracerProvider.
	otel.SetTracerProvider(tp)
	
	return tp, nil
}
