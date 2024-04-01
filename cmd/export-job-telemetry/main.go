package main

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/sethvargo/go-githubactions"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

const actionName = "export-job-telemetry"

var (
	BUILD_VERSION string
	BUILD_DATE    string
	COMMIT_ID     string
)

type InputParams struct {
	Traceparent             string
	OtelResourceAttrs       map[string]string
	OtelServiceName         string
	OtelExporterEndpoint    string
	OtelExporterOtlpHeaders map[string]string
	StartedAt               string
	CreatedAt               string
}

func parseInputParams() InputParams {
	return InputParams{
		Traceparent:             githubactions.GetInput("traceparent"),
		OtelResourceAttrs:       parseKeyValuePairs(githubactions.GetInput("otel-resource-attributes")),
		OtelServiceName:         githubactions.GetInput("otel-service-name"),
		OtelExporterEndpoint:    githubactions.GetInput("otel-exporter-otlp-endpoint"),
		OtelExporterOtlpHeaders: parseKeyValuePairs(githubactions.GetInput("otel-exporter-otlp-headers")),
		StartedAt:               githubactions.GetInput("started-at"),
		CreatedAt:               githubactions.GetInput("created-at"),
	}
}

func parseKeyValuePairs(input string) map[string]string {
	pairs := make(map[string]string)
	for _, pair := range strings.Split(input, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			pairs[kv[0]] = kv[1]
		}
	}
	return pairs
}

func initTracer(endpoint, serviceName string, attrs, headers map[string]string) func() {
	resourceAttributes := make([]attribute.KeyValue, 0, len(attrs)+1)
	for k, v := range attrs {
		resourceAttributes = append(resourceAttributes, attribute.String(k, v))
	}
	resourceAttributes = append(resourceAttributes, attribute.String(string(semconv.ServiceNameKey), serviceName))

	res := resource.NewWithAttributes(semconv.SchemaURL, resourceAttributes...)

	clientOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithHeaders(headers),
	}

	exp, err := otlptracegrpc.New(context.Background(), clientOptions...)
	if err != nil {
		githubactions.Fatalf("failed to initialize exporter: %v", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)

	return func() {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			githubactions.Errorf("failed to shut down tracer provider: %v", err)
		}
	}
}

func main() {
	githubactions.Infof("Starting %s version: %s (%s) commit: %s", actionName, BUILD_VERSION, BUILD_DATE, COMMIT_ID)

	params := parseInputParams()

	// Initialize tracing
	shutdownTracer := initTracer(params.OtelExporterEndpoint, params.OtelServiceName, params.OtelResourceAttrs, params.OtelExporterOtlpHeaders)
	defer shutdownTracer()

	// Parse the traceparent to extract the TraceID and SpanID
	parts := strings.Split(params.Traceparent, "-")
	if len(parts) != 4 {
		githubactions.Fatalf("invalid traceparent: %v", params.Traceparent)
	}

	traceID, err := hex.DecodeString(parts[1])
	if err != nil {
		githubactions.Fatalf("invalid TraceID: %v", err)
	}

	parentSpanID, err := hex.DecodeString(parts[2])
	if err != nil {
		githubactions.Fatalf("invalid SpanID: %v", err)
	}

	// Create a span context using the extracted TraceID and SpanID
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(traceID),
		SpanID:     trace.SpanID(parentSpanID),
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	// Prepare the context with the remote span context
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)

	// Extract the start time from the input parameters
	startedAtTime, err := time.Parse(time.RFC3339, params.StartedAt)
	if err != nil {
		githubactions.Fatalf("failed to parse started-at time: %v", err)
	}

	// Get the current time to use as the end time for the span
	endTime := time.Now()

	// Start the span with the specified start time
	tracer := otel.Tracer(actionName)
	_, span := tracer.Start(ctx, "Export Job Telemetry", trace.WithTimestamp(startedAtTime))

	// End the span with the specified end time
	span.End(trace.WithTimestamp(endTime))

	// Calculate the duration and set it as an attribute
	duration := endTime.Sub(startedAtTime)
	span.SetAttributes(attribute.Int64("job.duration.ms", duration.Milliseconds()))

	// Set additional attributes from the input parameters
	for k, v := range params.OtelResourceAttrs {
		span.SetAttributes(attribute.String(k, v))
	}
}
