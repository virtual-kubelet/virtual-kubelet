Debugging virtual-kubelet
=========================

## Metrics

Not implemented.

## Tracing

virtual-kubelet uses [OpenCensus](https://www.opencensus.io) to record traces. These traces include requests on the HTTP API as well as the reconciliation loop which reconciles virtual-kubelet pods with what's in the Kubernetes API server.

The granularity of traces may depend on the service provider (e.g. `azure`, `aws`, etc) being used.

### Tracing Exporters

Traces are collected and then exported to any configured exporter. Built-in exporters currently include:

- `jaeger` - [Jaeger Tracing](https://www.jaegertracing.io), supports configuration through environment variables.
    - `JAEGER_ENDPOINT` - Jaeger HTTP Thrift endpoint, e.g. `http://localhost:14268`
    - `JAGER_AGENT_ENDPOINT` - Jaeger agent address, e.g. `localhost:6831`
    - `JAEGER_USER`
    - `JAEGER_PASSWORD`
- `zpages` - [OpenCensus Zpages](https://opencensus.io/core-concepts/z-pages/). Currently supports configuration through environment variables, but this interface is **not** considered stable.
    - ZPAGES_PORT - e.g. `localhost:8080` sets the address to setup the HTTP server to serve zpages on. Will be available at `http://<address>:<port>/debug/tracez`

If consuming virtual-kubelet as a library you can configure your own tracing exporter.

Traces propagated from other services must be propagated using Zipkin's B3 format. Other formats may be supported in the future.

### Tracing Configuration

- `--trace-exporter` - Sets the exporter to use. Multiple exporters can be specified. If this is unset, traces are not exported.
- `--trace-service-name` - Sets the name of the service, defaults to `virtual-kubelet` but can be anything. This value is passed to the exporter purely for display purposes.
- `--trace-tag` - Adds tags in a `<key>=<value>` form which is included with collected traces. Think of this like log tags but for traces.
- `--trace-sample-rate` - Sets the probability for traces to be recorded. Traces are considered an expensive operation so you may want to set this to a lower value. Range is a value of 0 to 100 where 0 is never trace and 100 is always trace.