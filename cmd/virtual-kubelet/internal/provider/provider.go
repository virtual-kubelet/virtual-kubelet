package provider

import (
	"context"

	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	v1 "k8s.io/api/core/v1"
)

// Provider wraps the core provider type with an extra function needed to bootstrap the node
type Provider interface {
	nodeutil.Provider
	// ConfigureNode enables a provider to configure the node object that
	// will be used for Kubernetes.
	ConfigureNode(context.Context, *v1.Node)
}
