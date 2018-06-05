package operations

import (
	"fmt"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	vicerrors "github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
)

type PodStopper interface {
	Stop(op trace.Operation, id, name string) error
}

type VicPodStopper struct {
	client         *client.PortLayer
	isolationProxy proxy.IsolationProxy
}

type VicPodStopperError string

func (e VicPodStopperError) Error() string { return string(e) }

const (
	PodStopperPortlayerClientError = VicPodStopperError("PodStopper called with an invalid portlayer client")
	PodStopperIsolationProxyError  = VicPodStopperError("PodStopper called with an invalid isolation proxy")
	PodStopperInvalidPodIDError    = VicPodStopperError("PodStopper called with invalid PodID")
	PodStopperInvalidPodNameError  = VicPodStopperError("PodStopper called with invalid PodName")
)

func NewPodStopper(client *client.PortLayer, isolationProxy proxy.IsolationProxy) (PodStopper, error) {
	if client == nil {
		return nil, PodStopperPortlayerClientError
	} else if isolationProxy == nil {
		return nil, PodStopperIsolationProxyError
	}

	return &VicPodStopper{
		client:         client,
		isolationProxy: isolationProxy,
	}, nil
}

// Stop stops a pod but does not delete it
//
// arguments:
//		op		operation trace logger
//		id		pod id
//		name	pod name
// returns:
// 		error
func (v *VicPodStopper) Stop(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))

	operation := func() error {
		return v.stop(op, id, name)
	}

	config := retry.NewBackoffConfig()
	config.MaxElapsedTime = 10 * time.Minute
	if err := retry.DoWithConfig(operation, vicerrors.IsConflictError, config); err != nil {
		return err
	}
	return nil
}

func (v *VicPodStopper) stop(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))

	h, err := v.isolationProxy.Handle(op, id, name)
	if err != nil {
		return err
	}

	// Unbind the container to the scope
	h, ep, err := v.isolationProxy.UnbindScope(op, h, name)
	if err != nil {
		return err
	}

	op.Infof("Scope Unbind returned endpoints %# +v", ep)

	h, err = v.isolationProxy.SetState(op, h, name, "STOPPED")
	if err != nil {
		return err
	}

	err = v.isolationProxy.CommitHandle(op, h, id, -1)
	op.Infof("Commit handler returned %v", err)
	return err
}
