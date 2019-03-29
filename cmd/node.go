package cmd

import (
	"context"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/version"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// vkVersion is a concatenation of the Kubernetes version the VK is built against, the string "vk" and the VK release version.
	// TODO @pires revisit after VK 1.0 is released as agreed in https://github.com/virtual-kubelet/virtual-kubelet/pull/446#issuecomment-448423176.
	vkVersion = strings.Join([]string{"v1.13.1", "vk", version.Version}, "-")
)

// NodeFromProvider builds a kubernetes node object from a provider
// This is a temporary solution until node stuff actually split off from the provider interface itself.
func NodeFromProvider(ctx context.Context, name string, taint *v1.Taint, p providers.Provider) *v1.Node {
	taints := make([]v1.Taint, 0)

	if taint != nil {
		taints = append(taints, *taint)
	}

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"type":                   "virtual-kubelet",
				"kubernetes.io/role":     "agent",
				"beta.kubernetes.io/os":  strings.ToLower(p.OperatingSystem()),
				"kubernetes.io/hostname": name,
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
			},
		},
		Spec: v1.NodeSpec{
			Taints: taints,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				OperatingSystem: p.OperatingSystem(),
				Architecture:    "amd64",
				KubeletVersion:  vkVersion,
			},
			Capacity:        p.Capacity(ctx),
			Allocatable:     p.Capacity(ctx),
			Conditions:      p.NodeConditions(ctx),
			Addresses:       p.NodeAddresses(ctx),
			DaemonEndpoints: *p.NodeDaemonEndpoints(ctx),
		},
	}
	return node
}
