package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-githubactions"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
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

	ctx := context.Background()

	shutdownTracer := initTracer(params.OtelExporterEndpoint, params.OtelServiceName, params.OtelResourceAttrs, params.OtelExporterOtlpHeaders)
	defer shutdownTracer()

	tracer := otel.Tracer(actionName)
	_, span := tracer.Start(ctx, "Export Job Telemetry")
	defer span.End()

	startedAtTime, err := time.Parse(time.RFC3339, params.StartedAt)
	if err != nil {
		githubactions.Fatalf("failed to parse started-at time: %v", err)
	}

	duration := time.Since(startedAtTime)
	span.SetAttributes(attribute.String("job.duration", duration.String()))

	parts := strings.Split(params.Traceparent, "-")
	if len(parts) < 4 {
		githubactions.Fatalf("invalid traceparent: %v", params.Traceparent)
	}

	traceID, _ := hex.DecodeString(parts[1])
	spanID, _ := hex.DecodeString(parts[2])

	fmt.Printf("TraceID: %x\nSpanID: %x\n", traceID, spanID)

	for k, v := range params.OtelResourceAttrs {
		span.SetAttributes(attribute.String(k, v))
	}
}
