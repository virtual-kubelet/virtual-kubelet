package operations

import (
	"fmt"
	"net"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/trace"

	"k8s.io/api/core/v1"
)

type PodStatus interface {
	GetStatus(op trace.Operation, namespace string, name string, hostAddress string) (*v1.PodStatus, error)
}

type VicPodStatus struct {
	client         *client.PortLayer
	isolationProxy proxy.IsolationProxy
}

type VicPodStatusError string

func (e VicPodStatusError) Error() string { return string(e) }

const (
	PodStatusPortlayerClientError = VicPodStatusError("PodStatus called with an invalid portlayer client")
	PodStatusIsolationProxyError  = VicPodStatusError("PodStatus called with an invalid isolation proxy")
	PodStatusInvalidPodIDError    = VicPodStatusError("PodStatus called with invalid PodID")
	PodStatusInvalidPodNameError  = VicPodStatusError("PodStatus called with invalid PodName")
)

func NewPodStatus(client *client.PortLayer, isolationProxy proxy.IsolationProxy) (PodStatus, error) {
	if client == nil {
		return nil, PodStatusPortlayerClientError
	} else if isolationProxy == nil {
		return nil, PodStatusIsolationProxyError
	}

	return &VicPodStatus{
		client:         client,
		isolationProxy: isolationProxy,
	}, nil
}

// Gets pod status does not delete it
//
// arguments:
//		op		operation trace logger
//		id		pod id
//		name	pod name
// returns:
// 		error
func (v *VicPodStatus) GetStatus(op trace.Operation, id, name string, hostAddress string) (*v1.PodStatus, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))

	return v.getStatus(op, id, name, hostAddress)
}

func (v *VicPodStatus) getStatus(op trace.Operation, id, name string, hostAddress string) (*v1.PodStatus, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))
	// Start out with unknown
	phase := v1.PodUnknown
	podReady := v1.ConditionUnknown
	podInitialized := v1.ConditionUnknown
	podScheduled := v1.ConditionUnknown

	// Get current state
	state, err := v.isolationProxy.State(op, id, name)
	if err == nil {
		podScheduled = v1.ConditionTrue
		switch state {
		case "Starting":
			// if we are starting let the user know they must use the force
			phase = v1.PodPending
			podInitialized = v1.ConditionFalse
			podReady = v1.ConditionFalse
		case "Running":
			phase = v1.PodRunning
			podInitialized = v1.ConditionTrue
			podReady = v1.ConditionTrue
		case "Stopping":
			phase = v1.PodRunning
			podReady = v1.ConditionFalse
			podInitialized = v1.ConditionTrue
		case "Stopped":
			phase = v1.PodSucceeded
			podReady = v1.ConditionFalse
			podInitialized = v1.ConditionTrue
		case "Removing":
			phase = v1.PodSucceeded
			podReady = v1.ConditionFalse
			podInitialized = v1.ConditionTrue
		case "Removed":
			phase = v1.PodSucceeded
			podReady = v1.ConditionFalse
			podInitialized = v1.ConditionTrue
		}
	}

	status := &v1.PodStatus{
		Phase: phase,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: podInitialized,
			},
			{
				Type:   v1.PodReady,
				Status: podReady,
			},
			{
				Type:   v1.PodScheduled,
				Status: podScheduled,
			},
		},
	}
	addresses, err := v.getIPAddresses(op, id, name)
	if err == nil && len(addresses) > 0 {
		status.HostIP = hostAddress
		status.PodIP = addresses[0]
	} else {
		status.HostIP = "0.0.0.0"
		status.PodIP = "0.0.0.0"
	}

	return status, nil
}

func (v *VicPodStatus) getIPAddresses(op trace.Operation, id, name string) ([]string, error) {
	defer trace.End(trace.Begin(id, op))

	apAddresses, err := v.isolationProxy.EpAddresses(op, id, name)
	if err != nil {
		return nil, err
	}

	IPAddresses := make([]string, 0)
	for _, epAddr := range apAddresses {
		if epAddr != "" {
			ip, _, err := net.ParseCIDR(epAddr)
			if err == nil {
				IPAddresses = append(IPAddresses, ip.String())
			}
		}
	}

	return IPAddresses, err
}
