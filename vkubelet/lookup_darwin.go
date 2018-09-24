package vkubelet

import (
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/aws"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azurebatch"
	"github.com/virtual-kubelet/virtual-kubelet/providers/huawei"
	"github.com/virtual-kubelet/virtual-kubelet/providers/hypersh"
	"github.com/virtual-kubelet/virtual-kubelet/providers/mock"
	"github.com/virtual-kubelet/virtual-kubelet/providers/sfmesh"
	"github.com/virtual-kubelet/virtual-kubelet/providers/web"
)

// Compile time proof that our implementations meet the Provider interface.
var _ Provider = (*alicloud.ECIProvider)(nil)
var _ Provider = (*aws.FargateProvider)(nil)
var _ Provider = (*azure.ACIProvider)(nil)
var _ Provider = (*hypersh.HyperProvider)(nil)
var _ Provider = (*web.BrokerProvider)(nil)
var _ Provider = (*mock.MockProvider)(nil)
var _ Provider = (*huawei.CCIProvider)(nil)
var _ Provider = (*azurebatch.Provider)(nil)
var _ Provider = (*sfmesh.SFMeshProvider)(nil)

func lookupProvider(provider, providerConfig string, rm *manager.ResourceManager, nodeName, operatingSystem, internalIP string, daemonEndpointPort int32) (Provider, error) {
	switch provider {
	case "alicloud":
		return alicloud.NewECIProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "aws":
		return aws.NewFargateProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "azure":
		return azure.NewACIProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "azurebatch":
		return azurebatch.NewBatchProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "hyper":
		return hypersh.NewHyperProvider(providerConfig, rm, nodeName, operatingSystem)
	case "web":
		return web.NewBrokerProvider(nodeName, operatingSystem, daemonEndpointPort)
	case "mock":
		return mock.NewMockProvider(providerConfig, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "huawei":
		return huawei.NewCCIProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	case "sfmesh":
		return sfmesh.NewSFMeshProvider(rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	default:
		return nil, errors.New("provider is not supported")
	}
}
