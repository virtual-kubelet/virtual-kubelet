# Virtual Kubelet Dynamic Resource Allocation (DRA) Support

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
  - [DRA in Kubernetes](#dra-in-kubernetes)
  - [Current Virtual Kubelet Resource Model](#current-virtual-kubelet-resource-model)
- [Proposal](#proposal)
- [Design Details](#design-details)
  - [New Optional Provider Interfaces](#new-optional-provider-interfaces)
  - [Core Integration Points](#core-integration-points)
  - [ResourceSlice Lifecycle](#resourceslice-lifecycle)
  - [Pod Lifecycle Integration](#pod-lifecycle-integration)
  - [RBAC Changes](#rbac-changes)
  - [Changes to the Provider](#changes-to-the-provider)
  - [Test Plan](#test-plan)
- [Alternatives Considered](#alternatives-considered)
<!-- /toc -->

## Summary

Add optional Dynamic Resource Allocation (DRA) support to the virtual-kubelet core, allowing providers
to advertise specialized hardware resources via `ResourceSlice` objects and participate in the
Kubernetes DRA scheduling and allocation flow. This follows the existing optional interface pattern
(like `PodNotifier`) so providers that do not need DRA are unaffected.

## Motivation

Dynamic Resource Allocation (DRA) became GA in Kubernetes 1.32 and is the standard mechanism for
managing specialized hardware resources (GPUs, FPGAs, accelerators, etc.) in Kubernetes. It replaces
the limitations of the device plugin framework with a richer, more flexible model based on
`ResourceSlice`, `ResourceClaim`, and `DeviceClass` objects.

Without core support, each provider that wants DRA would need to independently implement:
- ResourceSlice publishing and lifecycle management
- ResourceClaim watching and status updates
- Prepare/unprepare coordination around pod create/delete
- Appropriate RBAC and informer setup

This duplicates Kubernetes API plumbing across providers and risks inconsistent behavior.

### Goals

- Define optional provider interfaces for DRA resource advertising and claim handling
- Manage `ResourceSlice` object lifecycle (create, update, delete) in the core on behalf of providers
- Integrate resource claim preparation into the pod create/delete lifecycle in `PodController`
- Require zero changes to existing providers that do not implement DRA interfaces

## Background

### DRA in Kubernetes

In the standard Kubernetes DRA flow:

1. A **DRA driver** runs on each node and publishes `ResourceSlice` objects advertising the devices
   available on that node (e.g., 4x NVIDIA A100 GPUs).
2. A user creates a `ResourceClaim` (or uses a `ResourceClaimTemplate` in a pod spec) to request
   specific resources.
3. The **scheduler** matches claims against available `ResourceSlice` capacity and allocates the
   claim to a specific node, writing the allocation result back to the `ResourceClaim` status.
4. When the **kubelet** starts a pod with allocated claims, it calls `NodePrepareResources` on the
   DRA driver to make the devices available to the pod's containers (e.g., mounting device files,
   setting environment variables).
5. When the pod terminates, the kubelet calls `NodeUnprepareResources` to release the devices.

Key API types (in `k8s.io/api/resource/v1beta1`, GA in `resource/v1` as of 1.32):
- `ResourceSlice` -- advertises available devices on a node
- `ResourceClaim` -- a request for resources, with allocation results
- `DeviceClass` -- defines a class of devices with default configuration

### Current Virtual Kubelet Resource Model

Virtual kubelet currently uses a static resource model:

- Providers report node capacity and allocatable resources via `ConfigureNode()`, setting
  `node.Status.Capacity` and `node.Status.Allocatable` (CPU, memory, pods, and custom extended
  resources).
- The Kubernetes scheduler uses these static values for bin-packing decisions.
- There is no resource tracking, allocation, or per-pod resource coordination in the core.
- The `PodLifecycleHandler` interface (`CreatePod`, `UpdatePod`, `DeletePod`) has no awareness of
  resource claims.

The existing optional interface pattern is `PodNotifier` (defined in
[`node/podcontroller.go`](../../node/podcontroller.go)). If a provider implements `PodNotifier`, the
core uses async status notifications; otherwise it wraps the provider with a polling shim
(`syncProviderWrapper`). DRA support follows this same pattern.

## Proposal

Add two new optional interfaces to the `node` package that providers can implement to participate in
DRA:

1. **`ResourceSliceProvider`** -- for advertising available resources on the virtual node
2. **`ResourceClaimHandler`** -- for preparing/unpreparing resources when pods start and stop

If a provider implements these interfaces, the core will:
- Publish and manage `ResourceSlice` objects on behalf of the provider
- Watch `ResourceClaim` objects relevant to pods scheduled on this node
- Call `PrepareResources` before `CreatePod` and `UnprepareResources` after `DeletePod`

If a provider does not implement these interfaces, there is no change in behavior.

## Design Details

### New Optional Provider Interfaces

Add to [`node/podcontroller.go`](../../node/podcontroller.go) (alongside `PodNotifier`):

```go
import (
    resourceapi "k8s.io/api/resource/v1"
)

// ResourceSliceProvider is an optional interface that providers can implement to
// support Dynamic Resource Allocation (DRA). Providers that implement this interface
// advertise available hardware resources on the virtual node via ResourceSlice objects.
//
// The core will call GetResourceSlices to obtain the initial set of slices and will
// publish them to the Kubernetes API server. Providers should call the callback
// registered via NotifyResourceSlicesChanged whenever the available resources change.
type ResourceSliceProvider interface {
    // GetResourceSlices returns the set of ResourceSlice objects that represent
    // the devices/resources available on this virtual node.
    //
    // The returned slices should have their Spec populated but do not need
    // metadata (Name, Namespace, OwnerReferences) -- the core will set these.
    GetResourceSlices(ctx context.Context) ([]*resourceapi.ResourceSlice, error)

    // NotifyResourceSlicesChanged registers a callback that the provider should
    // invoke whenever the set of available resources changes. This will trigger
    // the core to re-fetch slices via GetResourceSlices and update the API server.
    //
    // NotifyResourceSlicesChanged must not block the caller.
    NotifyResourceSlicesChanged(ctx context.Context, cb func())
}

// ResourceClaimHandler is an optional interface that providers can implement to
// handle resource claim preparation and cleanup as part of the pod lifecycle.
//
// When a pod references one or more ResourceClaims, the core will call
// PrepareResources before calling CreatePod on the PodLifecycleHandler, and
// UnprepareResources after calling DeletePod.
type ResourceClaimHandler interface {
    // PrepareResources is called before a pod is created in the provider.
    // The provider should prepare the allocated resources (e.g., configure
    // device access, set up environment variables) and return any CDI device
    // IDs or environment variables that should be injected into the pod.
    //
    // The claims slice contains only the claims that have been allocated to
    // this node and are referenced by the pod.
    PrepareResources(ctx context.Context, pod *corev1.Pod, claims []*resourceapi.ResourceClaim) (*PreparedResources, error)

    // UnprepareResources is called after a pod is deleted from the provider.
    // The provider should release any resources that were prepared for the pod.
    UnprepareResources(ctx context.Context, pod *corev1.Pod, claims []*resourceapi.ResourceClaim) error
}

// PreparedResources holds the result of preparing resources for a pod.
type PreparedResources struct {
    // Devices maps container names to the list of CDI device IDs that should
    // be made available in the container.
    Devices map[string][]string

    // Env maps container names to additional environment variables that should
    // be set in the container.
    Env map[string][]corev1.EnvVar
}
```

### Core Integration Points

The DRA support touches three areas of the core:

**1. `PodController` (resource slice management)**

When `PodController.Run()` detects that the provider implements `ResourceSliceProvider`, it starts
a goroutine that:
- Calls `GetResourceSlices()` to get the initial set of slices
- Publishes them as `ResourceSlice` objects in the API server with appropriate `OwnerReferences`
  pointing to the virtual node (so they are garbage-collected if the node is deleted)
- Registers a callback via `NotifyResourceSlicesChanged()` that triggers a re-sync

This is analogous to how the core detects `PodNotifier` at startup in
[`node/podcontroller.go:296`](../../node/podcontroller.go):

```go
if p, ok := pc.provider.(asyncProvider); ok {
    provider = p
} else {
    wrapped := &syncProviderWrapper{...}
    ...
}
```

The DRA detection would follow the same pattern:

```go
if rsp, ok := pc.provider.(ResourceSliceProvider); ok {
    rsp.NotifyResourceSlicesChanged(ctx, func() {
        pc.syncResourceSlices.Enqueue(ctx, nodeName)
    })
    // Initial sync
    pc.syncResourceSlices.Enqueue(ctx, nodeName)
}
```

**2. `PodController` (claim watching)**

A new informer for `ResourceClaim` objects is added to `PodControllerConfig`. When the provider
implements `ResourceClaimHandler`, the core:
- Watches `ResourceClaim` objects in namespaces relevant to scheduled pods
- Resolves `ResourceClaimTemplate` references in pod specs to their corresponding `ResourceClaim`
  objects

**3. Pod lifecycle hooks (`node/pod.go`)**

The `createOrUpdatePod` function is extended to call `PrepareResources` before `CreatePod`:

```go
func (pc *PodController) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {
    // ... existing downward API resolution ...

    // DRA: Prepare resources if the provider supports it and the pod has claims
    if rch, ok := pc.provider.(ResourceClaimHandler); ok && podHasResourceClaims(pod) {
        claims, err := pc.resolveResourceClaims(ctx, pod)
        if err != nil {
            return err
        }
        prepared, err := rch.PrepareResources(ctx, pod, claims)
        if err != nil {
            pc.handleProviderError(ctx, span, err, pod)
            return err
        }
        // Inject CDI devices and env vars into the pod spec for the provider
        applyPreparedResources(podForProvider, prepared)
    }

    // ... existing CreatePod / UpdatePod logic ...
}
```

The `deletePod` function is extended to call `UnprepareResources` after `DeletePod`:

```go
func (pc *PodController) deletePod(ctx context.Context, pod *corev1.Pod) error {
    // ... existing DeletePod call ...

    // DRA: Unprepare resources if the provider supports it
    if rch, ok := pc.provider.(ResourceClaimHandler); ok && podHasResourceClaims(pod) {
        claims, err := pc.resolveResourceClaims(ctx, pod)
        if err != nil {
            log.G(ctx).WithError(err).Warn("Failed to resolve resource claims for cleanup")
        } else if err := rch.UnprepareResources(ctx, pod, claims); err != nil {
            log.G(ctx).WithError(err).Warn("Failed to unprepare resources")
        }
    }

    return nil
}
```

### ResourceSlice Lifecycle

```
Provider                         Core                          K8s API Server
   |                              |                                |
   |  GetResourceSlices()         |                                |
   |<-----------------------------|                                |
   |  returns [SliceA, SliceB]    |                                |
   |----------------------------->|                                |
   |                              |  Create ResourceSlice A        |
   |                              |------------------------------->|
   |                              |  Create ResourceSlice B        |
   |                              |------------------------------->|
   |                              |                                |
   | (resources change)           |                                |
   |  cb()                        |                                |
   |----------------------------->|                                |
   |  GetResourceSlices()         |                                |
   |<-----------------------------|                                |
   |  returns [SliceA, SliceC]    |                                |
   |----------------------------->|                                |
   |                              |  Update ResourceSlice A        |
   |                              |------------------------------->|
   |                              |  Delete ResourceSlice B        |
   |                              |------------------------------->|
   |                              |  Create ResourceSlice C        |
   |                              |------------------------------->|
```

### Pod Lifecycle Integration

The extended pod lifecycle with DRA:

```
K8s API Server       Core (PodController)        Provider
     |                       |                       |
     | Pod scheduled         |                       |
     |---------------------->|                       |
     |                       | resolveResourceClaims |
     |<----------------------| (fetch claims)        |
     |  ResourceClaim objs   |                       |
     |---------------------->|                       |
     |                       | PrepareResources()    |
     |                       |---------------------->|
     |                       |    PreparedResources   |
     |                       |<----------------------|
     |                       | (inject CDI/env)      |
     |                       | CreatePod()           |
     |                       |---------------------->|
     |                       |                       |
     | Pod deleted           |                       |
     |---------------------->|                       |
     |                       | DeletePod()           |
     |                       |---------------------->|
     |                       | UnprepareResources()  |
     |                       |---------------------->|
     |                       |                       |
```

### RBAC Changes

The virtual-kubelet ClusterRole needs additional permissions for DRA objects. Add to
[`hack/skaffold/virtual-kubelet/base.yml`](../../hack/skaffold/virtual-kubelet/base.yml):

```yaml
- apiGroups:
  - resource.k8s.io
  resources:
  - resourceslices
  verbs:
  - create
  - delete
  - get
  - list
  - update
  - watch
- apiGroups:
  - resource.k8s.io
  resources:
  - resourceclaims
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - resource.k8s.io
  resources:
  - resourceclaims/status
  verbs:
  - get
  - update
  - patch
- apiGroups:
  - resource.k8s.io
  resources:
  - deviceclasses
  verbs:
  - get
  - list
  - watch
```

### Changes to the Provider

This is an additive, opt-in change. Existing providers require zero modifications.

Providers that want DRA support implement one or both of the new interfaces:

```go
import (
    "context"
    resourceapi "k8s.io/api/resource/v1"
    "github.com/virtual-kubelet/virtual-kubelet/node"
)

// Example: A provider that advertises GPU resources
type MyProvider struct {
    // ... existing fields ...
}

// Implement ResourceSliceProvider to advertise GPUs
func (p *MyProvider) GetResourceSlices(ctx context.Context) ([]*resourceapi.ResourceSlice, error) {
    // Return slices describing available GPUs
    return []*resourceapi.ResourceSlice{
        {
            Spec: resourceapi.ResourceSliceSpec{
                Driver:   "gpu.myprovider.io",
                Pool: resourceapi.ResourcePool{
                    Name:               "gpu-pool",
                    Generation:         1,
                    ResourceSliceCount: 1,
                },
                Devices: []resourceapi.Device{
                    {
                        Name: "gpu-0",
                        Basic: &resourceapi.BasicDevice{
                            Attributes: map[resourceapi.QualifiedName]resourceapi.DeviceAttribute{
                                "gpu.myprovider.io/model": {StringValue: ptr("A100")},
                                "gpu.myprovider.io/memory": {StringValue: ptr("80Gi")},
                            },
                        },
                    },
                },
            },
        },
    }, nil
}

func (p *MyProvider) NotifyResourceSlicesChanged(ctx context.Context, cb func()) {
    p.onResourceChange = cb
}

// Implement ResourceClaimHandler to prepare/unprepare GPU access
func (p *MyProvider) PrepareResources(ctx context.Context, pod *corev1.Pod, claims []*resourceapi.ResourceClaim) (*node.PreparedResources, error) {
    // Set up GPU access for the pod in the backend
    return &node.PreparedResources{
        Devices: map[string][]string{
            "main-container": {"gpu.myprovider.io/gpu=gpu-0"},
        },
    }, nil
}

func (p *MyProvider) UnprepareResources(ctx context.Context, pod *corev1.Pod, claims []*resourceapi.ResourceClaim) error {
    // Release GPU access for the pod
    return nil
}
```

### Test Plan

- Add unit tests for `ResourceSliceProvider` detection and `ResourceSlice` lifecycle management
- Add unit tests for `ResourceClaimHandler` integration in `createOrUpdatePod` and `deletePod`
- Add unit tests verifying that providers not implementing DRA interfaces see no behavior change
- Extend the mock provider with an optional DRA-enabled variant for integration testing
- End-to-end test: deploy a pod with a `ResourceClaim` to a virtual node backed by a DRA-enabled
  mock provider, verify the claim is prepared and the pod is created successfully

## Alternatives Considered

**1. Leave DRA entirely to providers**

Each provider would independently implement ResourceSlice publishing, claim watching, and pod
lifecycle coordination. This was rejected because:
- It duplicates significant Kubernetes API plumbing across providers
- Pod lifecycle hooks (`createOrUpdatePod`, `deletePod`) live in the core and providers cannot
  easily inject prepare/unprepare steps at the right points without core cooperation
- Risk of inconsistent behavior across providers

**2. Add DRA as a required interface**

Adding `PrepareResources`/`UnprepareResources` to `PodLifecycleHandler` or `GetResourceSlices` to
`nodeutil.Provider`. This was rejected because:
- It would be a breaking API change
- Many providers do not need DRA support
- It violates the project's principle of keeping the interface surface area small

**3. Implement a full kubelet DRA gRPC plugin shim**

Virtual kubelet could implement the kubelet side of the DRA gRPC plugin protocol, allowing standard
DRA drivers to work with virtual nodes. This was rejected because:
- Virtual kubelet providers are not real nodes -- standard DRA drivers assume local device access
- The Go interface approach is simpler and more aligned with how virtual-kubelet already works
- Nothing prevents a provider from internally using a gRPC-based driver if it wants to
