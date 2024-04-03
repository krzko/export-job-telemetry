# Export Job Telemetry

This GitHub Action is designed to export telemetry data for a GitHub Actions job, including resource attributes and timing information, using OpenTelemetry. To minimise API calls to the GitHub API and to ensure deterministic trace and span IDs, instrument your workflow with [https://github.com/krzko/set-up-telemetry](https://github.com/krzko/set-up-telemetry).

This action is intended to be used in conjunction with the [OpenTelemetry Collector GitHub Actions Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27460). This receiver processes GitHub Actions webhook events to observe workflows and jobs, converting them into trace telemetry for detailed observability.

## Features

- Export trace data in OpenTelemetry format.
- Capture and report the start and end times of the GitHub Actions job.
- Include custom resource attributes for enhanced observability.
- Utilises deterministic Trace and Span IDs to align with the OpenTelemetry Collector GitHub Actions Receiver.

## GitHub Actions Receiver

The GitHub Actions Receiver processes GitHub Actions webhook events to observe workflows and jobs. It handles `workflow_job` and `workflow_run` event payloads, transforming them into trace telemetry. This allows the observation of workflow execution times, success, and failure rates. If a secret is configured (recommended), it validates the payload ensuring data integrity before processing.

For more details on the receiver, see the GitHub issue: [OpenTelemetry Collector Contrib #27460](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27460).

## Usage

To use this action in your GitHub Actions workflow, add a step that references this action in your `.github/workflows/` YAML file.

Here is a basic example of how to use this action:

```yaml
name: Test and Build

on:
  push:

env:
  otel-exporter-otlp-endpoint: otelcol.foo.corp:443
  otel-service-name: o11y.workflows
  otel-resource-attributes: deployment.environent=dev,service.version=0.1.0

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Set up telemetry
        id: set-up-telemetry
        uses: krzko/set-up-telemetry@v0.2.0

      - name: Checkout
        uses: actions/checkout@v4

      - run: # do_some_work

      - name: Export job telemetry
        if: always()
        uses: krzko/export-job-telemetry@v0.2.0
        with:
          job-status: ${{ job.status }}
          otel-exporter-otlp-endpoint: ${{ env.otel-exporter-otlp-endpoint }}
          otel-resource-attributes: "foo.new_attribute=123,${{ env.otel-resource-attributes }}"
          otel-service-name: ${{ env.otel-service-name }}
          started-at: ${{ steps.set-up-telemetry.outputs.started-at }}
          traceparent: ${{ steps.set-up-telemetry.outputs.traceparent }}

  build:
    runs-on: ubuntu-latest
    steps:
      - name: Set up telemetry
        id: set-up-telemetry
        uses: krzko/set-up-telemetry@v0.2.0

      - name: Checkout
        uses: actions/checkout@v4

      - run: # do_some_work

      - name: Export job telemetry
        if: always()
        uses: krzko/export-job-telemetry@v0.1.0
        with:
          job-status: ${{ job.status }}
          otel-exporter-otlp-endpoint: ${{ env.otel-exporter-otlp-endpoint }}
          otel-resource-attributes: "foo.new_attribute=456,${{ env.otel-resource-attributes }}"
          otel-service-name: ${{ env.otel-service-name }}
          started-at: ${{ steps.set-up-telemetry.outputs.started-at }}
          traceparent: ${{ steps.set-up-telemetry.outputs.traceparent }}
```

## Inputs

| Name | Description | Required |
|------|-------------|:--------:|
| `traceparent` | The traceparent value for the OpenTelemetry trace, used to continue a trace. | Yes |
| `otel-resource-attributes` | Key-value pairs to be used as resource attributes. Set via comma-separated values; `key1=value1,key2=value2`. | No |
| `otel-service-name` | Logical name of the service. Sets the value of the `service.name` resource attribute. | Yes |
| `otel-exporter-otlp-endpoint` | The endpoint for the OTLP gRPC exporter. | Yes |
| `started-at` | The start time of the GitHub Actions job, used to calculate the job's metrics. Format should be in ISO 8601. | Yes |
| `created-at` | The creation time of the GitHub Actions job, used to calculate the job's metrics. Format should be in ISO 8601. | No |
| `job-status` | The status of the GitHub Actions job. | Yes |

## Outputs

This action does not set any outputs directly, but it sends telemetry data to the specified OpenTelemetry collector endpoint.

## Contributing

Contributions to this project are welcome! Please follow the standard GitHub pull request workflow.
