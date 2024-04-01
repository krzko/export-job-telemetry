# Export Job Telemetry

This GitHub Action exports telemetry data for a GitHub Actions job, including resource attributes and timing information, using OpenTelemetry.

## Features

- Export trace data in OpenTelemetry format.
- Capture and report the start and end times of the GitHub Actions job.
- Include custom resource attributes for enhanced observability.

## Usage

To use this action in your GitHub Actions workflow, add a step that references this action in your `.github/workflows/` YAML file.

Here is a basic example of how to use this action:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - run: # do_some_work

      - name: Export job telemetry
        if: always()
        uses: your-username/export-job-telemetry@main
        with:
          otel-exporter-otlp-endpoint: ${{ env.OTEL_EXPORTER_OTLP_ENDPOINT }}
          otel-resource-attributes: ${{ env.OTEL_RESOURCE_ATTRIBUTES }}
          otel-service-name: ${{ env.OTEL_SERVICE_NAME }}
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

## Outputs

This action does not set any outputs directly, but it sends telemetry data to the specified OpenTelemetry collector endpoint.

## Contributing

Contributions to this project are welcome! Please follow the standard GitHub pull request workflow.
