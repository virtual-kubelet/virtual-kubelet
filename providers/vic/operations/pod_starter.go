package operations

import (
	"context"
	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/trace"
)

type PodStarter interface {
	Start(op trace.Operation, id, name string) error
}

type VicPodStarter struct {
	client         *client.PortLayer
	isolationProxy proxy.IsolationProxy
	imageStore     proxy.ImageStore
}

type VicPodStarterError string

func (e VicPodStarterError) Error() string { return string(e) }

const (
	PodStarterPortlayerClientError = VicPodStarterError("PodStarter called with an invalid portlayer client")
	PodStarterIsolationProxyError  = VicPodStarterError("PodStarter called with an invalid isolation proxy")
	PodStarterInvalidPodIDError    = VicPodStarterError("PodStarter called with invalid Pod ID")
	PodStarterInvalidPodNameError  = VicPodStarterError("PodStarter called with invalid Pod name")
)

func NewPodStarter(client *client.PortLayer, isolationProxy proxy.IsolationProxy) (PodStarter, error) {
	defer trace.End(trace.Begin("", context.Background()))

	if client == nil {
		return nil, PodStarterPortlayerClientError
	}
	if isolationProxy == nil {
		return nil, PodStarterIsolationProxyError
	}

	return &VicPodStarter{
		client:         client,
		isolationProxy: isolationProxy,
	}, nil
}

// Start starts up the pod vm
//
// arguments:
//		op		operation trace logger
//		id		pod id
//		name	pod name
// returns:
//		error
func (v *VicPodStarter) Start(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))

	h, err := v.isolationProxy.Handle(op, id, name)
	if err != nil {
		return err
	}

	// Bind the container to the scope
	h, ep, err := v.isolationProxy.BindScope(op, h, name)
	if err != nil {
		return err
	}

	op.Debugf("*** Scope bind returned endpoints %#v", ep)

	defer func() {
		if err != nil {
			op.Debugf("Unbinding %s due to error - %s", id, err.Error())
			v.isolationProxy.UnbindScope(op, h, name)
		}
	}()

	h, err = v.isolationProxy.SetState(op, h, name, "RUNNING")
	if err != nil {
		return err
	}

	// map ports

	err = v.isolationProxy.CommitHandle(op, h, id, -1)

	return nil
}
