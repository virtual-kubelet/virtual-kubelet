package provider

import (
	"context"

	v1 "k8s.io/api/core/v1"

	"github.com/nuczzz/virtual-kubelet/node/nodeutil"
)

// Provider wraps the core provider type with an extra function needed to bootstrap the node
type Provider interface {
	nodeutil.Provider
	// ConfigureNode enables a provider to configure the node object that
	// will be used for Kubernetes.
	ConfigureNode(context.Context, *v1.Node)
}
