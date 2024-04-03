package main

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/sethvargo/go-githubactions"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	JobStatus               string
	JobName                 string
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
		JobStatus:               githubactions.GetInput("job-status"),
		JobName:                 githubactions.GetInput("job-name"),
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

	shutdownTracer := initTracer(params.OtelExporterEndpoint, params.OtelServiceName, params.OtelResourceAttrs, params.OtelExporterOtlpHeaders)
	defer shutdownTracer()

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

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(traceID),
		SpanID:     trace.SpanID(parentSpanID),
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)

	startedAtTime, err := time.Parse(time.RFC3339, params.StartedAt)
	if err != nil {
		githubactions.Fatalf("failed to parse started-at time: %v", err)
	}

	tracer := otel.Tracer(actionName)
	_, span := tracer.Start(ctx, "Job telemetry", trace.WithTimestamp(startedAtTime))

	attributes := []attribute.KeyValue{
		attribute.String("ci.github.workflow.job.conclusion", params.JobStatus),
	}

	var spanStatus codes.Code
	var spanMessage string
	switch params.JobStatus {
	case "success":
		spanStatus = codes.Ok
		spanMessage = "Job completed successfully"
	case "failure":
		spanStatus = codes.Error
		spanMessage = "Job failed"
	default:
		spanStatus = codes.Unset
		spanMessage = "Job status unknown"
	}
	span.SetStatus(spanStatus, spanMessage)

	if params.CreatedAt != "" {
		createdAtTime, err := time.Parse(time.RFC3339, params.CreatedAt)
		if err != nil {
			githubactions.Fatalf("failed to parse created-at time: %v", err)
		}

		latency := startedAtTime.Sub(createdAtTime)
		attributes = append(attributes, attribute.Int64("ci.github.workflow.job.start_latency_ms", latency.Milliseconds()))
	}

	if params.JobName != "" {
		if err != nil {
			githubactions.Fatalf("failed to parse job-name: %v", err)
		}
		attributes = append(attributes, attribute.String("ci.github.workflow.job.name", params.JobName))
	}

	endTime := time.Now()
	duration := endTime.Sub(startedAtTime)
	attributes = append(attributes, attribute.Int64("ci.github.workflow.job.duration_ms", duration.Milliseconds()))

	for k, v := range params.OtelResourceAttrs {
		attributes = append(attributes, attribute.String(k, v))
	}

	span.SetAttributes(attributes...)

	span.End(trace.WithTimestamp(endTime))
}
