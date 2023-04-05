#  Virtual Kubelet Metrics Update

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Design Details](#design-details)
  - [API](#api)
  - [Data](#data)
  - [Changes to the Provider](#changes-to-the-provider)
  - [Test Plan](#test-plan)
<!-- /toc -->

## Summary

Add the new /metrics/resource endpoint in the virtual-kubelet to support the metrics server update for new Kubernetes versions `>=1.24`


## Motivation

The Kubernetes metrics server now tries to get metrics from the kubelet using the new metrics endpoint [/metrics/resource](https://github.com/kubernetes-sigs/metrics-server/commit/a2d732e5cdbfd93a6ebce221e8df0e8b463eecc6#diff-6e5b914d1403a14af1cc43582a2c9af727113037a3c6a77d8729aaefba084fb5R88),
while Virtual Kubelet is still exposing the earlier metrics endpoint [/stats/summary](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/node/api/server.go#L90). 
This causes metrics to break when using virtual kubelet with newer Kubernetes versions (>=1.24). 
To support the new metrics server, this document proposes adding a new handler to handle the updated metrics endpoint. 
This will be an additive update, and the old 
[/stats/summary](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/node/api/server.go#L90) endpoint will still be available to maintain backward compatibility with 
the older metrics server version.


### Goals

- Support metrics for kubernetes version `>=1.24` through adding /metrics/resource endpoint handler.

### Non-Goals

- Ensure pod autoscaling works as expected with the newer kubernetes versions `>=1.24` as expected

## Proposal

Add a new handler for `/metrics/resource` endpoint that calls a new `GetMetricsResource` method in the provider,
which in-turn returns metrics using the prometheus `model.Samples` data structure as expected by the new metrics server.
The provider will need to implement the `GetMetricsResource` method in order to add support for the new `/metrics/resource` endpoint with Kubernetes version >=1.24


## Design Details
Currently the virtual kubelet code uses the `PodStatsSummaryHandler` method to set up a http handler for serving pod metrics via the `/stats/summary` endpoint. 
To support the updated metrics server, we need to add another handler `PodMetricsResourceHandler`  which can serve metrics via the `/metrics/resource` endpoint. 
The `PodMetricsResourceHandler` calls the new `GetMetricsResource` method of the provider to get the metrics from the specific provider. 

### API
Add `GetMetricsResource` to `PodHandlerConfig`
```go
type PodHandlerConfig struct { //nolint:golint
	RunInContainer   ContainerExecHandlerFunc
	GetContainerLogs ContainerLogsHandlerFunc
	// GetPods is meant to enumerate the pods that the provider knows about
	GetPods PodListerFunc
	// GetPodsFromKubernetes is meant to enumerate the pods that the node is meant to be running
	GetPodsFromKubernetes PodListerFunc
	GetStatsSummary       PodStatsSummaryHandlerFunc
	GetMetricsResource    PodMetricsResourceHandlerFunc
	StreamIdleTimeout     time.Duration
	StreamCreationTimeout time.Duration
}
```
Add endpoint to `PodHandler` method 
```go
const MetricsResourceRouteSuffix = "/metrics/resource"

func PodHandler(p PodHandlerConfig, debug bool) http.Handler {
	r := mux.NewRouter()

	// This matches the behaviour in the reference kubelet
	r.StrictSlash(true)
	if debug {
		r.HandleFunc("/runningpods/", HandleRunningPods(p.GetPods)).Methods("GET")
	}

	r.HandleFunc("/pods", HandleRunningPods(p.GetPodsFromKubernetes)).Methods("GET")
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", HandleContainerLogs(p.GetContainerLogs)).Methods("GET")
	r.HandleFunc(
		"/exec/{namespace}/{pod}/{container}",
		HandleContainerExec(
			p.RunInContainer,
			WithExecStreamCreationTimeout(p.StreamCreationTimeout),
			WithExecStreamIdleTimeout(p.StreamIdleTimeout),
		),
	).Methods("POST", "GET")

	if p.GetStatsSummary != nil {
		f := HandlePodStatsSummary(p.GetStatsSummary)
		r.HandleFunc("/stats/summary", f).Methods("GET")
		r.HandleFunc("/stats/summary/", f).Methods("GET")
	}

	if p.GetMetricsResource != nil {
		f := HandlePodMetricsResource(p.GetMetricsResource)
		r.HandleFunc(MetricsResourceRouteSuffix, f).Methods("GET")
		r.HandleFunc(MetricsResourceRouteSuffix+"/", f).Methods("GET")
	}
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}
```
  
New `PodMetricsResourceHandler` method, that uses the new `PodMetricsResourceHandlerFunc` definition.
```go
// PodMetricsResourceHandler creates an http handler for serving pod metrics.
//
// If the passed in handler func is nil this will create handlers which only
// serves http.StatusNotImplemented
func PodMetricsResourceHandler(f PodMetricsResourceHandlerFunc) http.Handler {
	if f == nil {
		return http.HandlerFunc(NotImplemented)
	}

	r := mux.NewRouter()

	h := HandlePodMetricsResource(f)

	r.Handle(MetricsResourceRouteSuffix, ochttp.WithRouteTag(h, "PodMetricsResourceHandler")).Methods("GET")
	r.Handle(MetricsResourceRouteSuffix+"/", ochttp.WithRouteTag(h, "PodMetricsResourceHandler")).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}

```
 
 
`HandlePodMetricsResource` method returns a HandlerFunc which serves the metrics encoded in prometheus' text format encoding as expected by the  metrics-server
```go
// HandlePodMetricsResource makes an HTTP handler for implementing the kubelet /metrics/resource endpoint
func HandlePodMetricsResource(h PodMetricsResourceHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		metrics, err := h(req.Context())
		if err != nil {
			if isCancelled(err) {
				return err
			}
			return errors.Wrap(err, "error getting status from provider")
		}

		b, err := json.Marshal(metrics)
		if err != nil {
			return errors.Wrap(err, "error marshalling metrics")
		}

		if _, err := w.Write(b); err != nil {
			return errors.Wrap(err, "could not write to client")
		}
		return nil
	})
}
```
 
The `PodMetricsResourceHandlerFunc` returns the metrics data using Prometheus' `MetricFamily` data structure. More details are provided in the Data subsection
```go
// PodMetricsResourceHandlerFunc defines the handler for getting pod metrics
type PodMetricsResourceHandlerFunc func(context.Context) ([]*dto.MetricFamily, error)
```
 
### Data

The updated metrics server does not add any new fields to the metrics data but uses the Prometheus textparse series parser to parse and reconstruct the [MetricsBatch](https://github.com/kubernetes-sigs/metrics-server/blob/83b2e01f9825849ae5f562e47aa1a4178b5d06e5/pkg/storage/types.go#L31) data structure.  
Currently virtual-kubelet is sending data to the server using the [summary](https://github.com/virtual-kubelet/virtual-kubelet/blob/be0a062aec9a5eeea3ad6fbe5aec557a235558f6/node/api/statsv1alpha1/types.go#L24) data structure. The Prometheus text parser expects a series of bytes as in the Prometheus [model.Samples](https://github.com/kubernetes/kubernetes/blob/a93eda9db305611cacd8b6ee930ab3149a08f9b0/vendor/github.com/prometheus/common/model/value.go#L184) data structure, similar to the test [here](https://github.com/prometheus/prometheus/blob/c70d85baed260f6013afd18d6cd0ffcac4339861/model/textparse/promparse_test.go#L31).
 
Examples of how the new metrics are defined may be seen in the Kubernetes e2e test that calls the /metrics/resource endpoint [here](https://github.com/kubernetes/kubernetes/blob/a93eda9db305611cacd8b6ee930ab3149a08f9b0/test/e2e_node/resource_metrics_test.go#L76), and the kubelet metrics defined in the Kubernetes/kubelet code [here](https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/metrics/collectors/resource_metrics.go) .
 
```go
var (
	nodeCPUUsageDesc = metrics.NewDesc("node_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the node in core-seconds",
		nil,
		nil,
		metrics.ALPHA,
		"")

	nodeMemoryUsageDesc = metrics.NewDesc("node_memory_working_set_bytes",
		"Current working set of the node in bytes",
		nil,
		nil,
		metrics.ALPHA,
		"")

	containerCPUUsageDesc = metrics.NewDesc("container_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the container in core-seconds",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	containerMemoryUsageDesc = metrics.NewDesc("container_memory_working_set_bytes",
		"Current working set of the container in bytes",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	podCPUUsageDesc = metrics.NewDesc("pod_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the pod in core-seconds",
		[]string{"pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	podMemoryUsageDesc = metrics.NewDesc("pod_memory_working_set_bytes",
		"Current working set of the pod in bytes",
		[]string{"pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	resourceScrapeResultDesc = metrics.NewDesc("scrape_error",
		"1 if there was an error while getting container metrics, 0 otherwise",
		nil,
		nil,
		metrics.ALPHA,
		"")

	containerStartTimeDesc = metrics.NewDesc("container_start_time_seconds",
		"Start time of the container since unix epoch in seconds",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")
)
```
 
The kubernetes/kubelet code implements Prometheus' [collector](https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/metrics/collectors/resource_metrics.go#L88) interface which is used along with the k8s.io/component-base implementation of the [registry](https://github.com/kubernetes/component-base/blob/40d14bdbd62f9e2ea697f97d81d4abc72839901e/metrics/registry.go#L114) interface in order to collect and return the metrics data using the Prometheus' [MetricFamily](https://github.com/prometheus/client_model/blob/master/go/metrics.pb.go#L773) data structure.
 
The Gather method in the registry calls the kubelet collector's Collect method, and returns the data using the MetricFamily data structure. The metrics server expects metrics to be encoded in prometheus'
text format, and the kubelet uses the http handler from prometheus' promhttp module which returns the metrics data encoded in prometheus' text format encoding.
```go
type KubeRegistry interface {
	// Deprecated
	RawMustRegister(...prometheus.Collector)
	// CustomRegister is our internal variant of Prometheus registry.Register
	CustomRegister(c StableCollector) error
	// CustomMustRegister is our internal variant of Prometheus registry.MustRegister
	CustomMustRegister(cs ...StableCollector)
	// Register conforms to Prometheus registry.Register
	Register(Registerable) error
	// MustRegister conforms to Prometheus registry.MustRegister
	MustRegister(...Registerable)
	// Unregister conforms to Prometheus registry.Unregister
	Unregister(collector Collector) bool
	// Gather conforms to Prometheus gatherer.Gather
	Gather() ([]*dto.MetricFamily, error)
	// Reset invokes the Reset() function on all items in the registry
	// which are added as resettables.
	Reset()
}
```

Prometheus’ MetricsFamily data structure:  
```go
type MetricFamily struct {
	Name                 *string     `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Help                 *string     `protobuf:"bytes,2,opt,name=help" json:"help,omitempty"`
	Type                 *MetricType `protobuf:"varint,3,opt,name=type,enum=io.prometheus.client.MetricType" json:"type,omitempty"`
	Metric               []*Metric   `protobuf:"bytes,4,rep,name=metric" json:"metric,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}
```

Therefore the provider's GetMetricsResource method should use the same return type as the Gather method in the registry interface.
 
### Changes to the Provider.

In order to support the new metrics endpoint the Provider must implement the GetMetricsResource method with definition

```golang

import (
  dto "github.com/prometheus/client_model/go"
  "context"
)

func GetMetricsResource(context.Context) ([]*dto.MetricsFamily, error) {
...
}
```

### Test Plan

- Write a provider implementation for GetMetricsResource method in ACI Provider and deploy pods get metrics using kubectl 
- Run end-to-end tests with the provider implementation

