package vkubelet

import (
	"context"
	"encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
)

type podStateMachineInterface interface {
	run(ctx context.Context)
	// updatePodControllerPod must not block. It is an updated state from the pod controller / apiserver.
	updatePodControllerPod(ctx context.Context, pod *v1.Pod)
	// updateProviderPod is called by providers to indicate that their pod has been updated. The pod is expected to have
	// a pod status in it.
	//
	// Kubelet <-> APIServer intemrediate line levels are not propogated
	// i.e. if a Pod goes from Initialiazed -> Running -> Failed, we may drop the status update that stated it was running.
	updateProviderPod(ctx context.Context, pod *v1.Pod)
}

type state func(ctx context.Context) (nextState state, err error)

// The pod state machine is the state and interaction model of a goroutine. It represents a pod, whether
// created from Kubernetes, or a provider at startup.
type podStateMachine struct {
	podLock sync.RWMutex

	providerPod             *v1.Pod
	providerDirtyCh         chan struct{}
	podControllerPod        *v1.Pod
	podControllerPodDirtyCh chan struct{}

	key   string
	state state

	pc        *PodController
	namespace string
	name      string

	sendUpdateRetry *time.Timer

	// This is the response from the patch of the last successful status update
	lastPatchResponsePod *v1.Pod
	// This is the last pod that we used to derive podstatus in order to send an update
	lastPodUsedForPatch *v1.Pod
}

func newPodStateMachineFromProvider(ctx context.Context, pod *v1.Pod, pc *PodController, key string) podStateMachineInterface {
	psm := newPodStateMachine(ctx, pod, pc, key)
	psm.state = psm.createdFromProviderState
	psm.providerPod = pod.DeepCopy()

	return psm
}
func newPodStateMachineFromAPIServer(ctx context.Context, pod *v1.Pod, pc *PodController, key string) podStateMachineInterface {
	psm := newPodStateMachine(ctx, pod, pc, key)
	psm.state = psm.createdFromAPIServerState
	psm.podControllerPod = pod.DeepCopy()
	return psm
}
func newPodStateMachine(ctx context.Context, pod *v1.Pod, pc *PodController, key string) *podStateMachine {
	timer := time.NewTimer(time.Duration(1<<63 - 1))
	if !timer.Stop() {
		<-timer.C
	}
	return &podStateMachine{
		pc:                      pc,
		key:                     key,
		namespace:               pod.Namespace,
		name:                    pod.Name,
		podControllerPodDirtyCh: make(chan struct{}, 1),
		providerDirtyCh:         make(chan struct{}, 1),
		sendUpdateRetry:         timer,
	}
}
func (psm *podStateMachine) run(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "run")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("key", psm.key))

	defer span.End()
	if psm.state == nil {
		panic("podStateMachine started with internal state of nil")
	}

	defer func() {
		psm.pc.server.lock.Lock()
		psm.pc.server.lock.Unlock()
		delete(psm.pc.server.podStateMachines, psm.key)
	}()

	psm.podLock.RLock()
	if pod := psm.podControllerPod; pod != nil {
		ctx = addPodAttributes(ctx, span, pod)
	} else if pod := psm.providerPod; pod != nil {
		ctx = addPodAttributes(ctx, span, pod)
	} else {
		psm.podLock.RUnlock()
		log.G(ctx).Error("Neither providerPod set nor podControllerPod, bailing out")
		return
	}
	psm.podLock.RUnlock()

	// Wait for the PC to become consistent before starting.
	select {
	case <-psm.pc.inSyncCh:
	case <-ctx.Done():
		log.G(ctx).WithError(ctx.Err()).Error("Pod state machine terminated before pod controller in-sync")
	}
	var err error
	nextState := psm.state
	// Termination can happen if the pod is in a terminal state. If the pod is deleted from API server and from the provider
	// then it is considered terminal. We will periodically call GetPod() on the provider, and when it returns pod not found
	// then we can shutdown
	for nextState != nil {
		if pod := psm.getProviderPod(); pod != nil {
			ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
				"podPhase": pod.Status.Reason,
				"reason":   pod.Status.Reason,
			}))
		} else if psm.getPodControllerPod(); pod != nil {
			ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
				"podPhase": pod.Status.Reason,
				"reason":   pod.Status.Reason,
			}))
		}

		prevState := nextState
		nextState, err = nextState(ctx)
		l := log.G(ctx).WithFields(
			map[string]interface{}{
				"prevState": runtime.FuncForPC(reflect.ValueOf(prevState).Pointer()).Name(),
				"nextState": runtime.FuncForPC(reflect.ValueOf(nextState).Pointer()).Name(),
			})
		if err != nil {
			l.WithError(err).Error("Encountered error in state transition")
		} else {
			l.Debug("State transition")
		}
	}
	if err == context.DeadlineExceeded || err == context.Canceled {
		log.G(ctx).Debug("Pod state machine context cancelled")
	} else if err != nil {
		log.G(ctx).WithError(err).Warn("Pod State Machine Terminating In Error")
		span.SetStatus(ocstatus.FromError(err))
	} else {
		log.G(ctx).Debug("Pod state machine terminating gracefully")
	}
}

func (psm *podStateMachine) storeProviderPod(pod *v1.Pod) (*v1.Pod, bool) {
	psm.podLock.Lock()
	defer psm.podLock.Unlock()
	psm.providerPod = pod
	return pod, true
}

func (psm *podStateMachine) getProviderPod() *v1.Pod {
	psm.podLock.RLock()
	defer psm.podLock.RUnlock()
	return psm.providerPod
}

// GetPod returns the pod state machine's pod. It prefers to return the provider pod, but if that's not available,
// it will return the API Server pod.
// This in turn means that the pods returns by this may not include valid resource versions, or if there are updates
// to the pod like deletionTimestamp, they wont be reflected
func (psm *podStateMachine) GetPod() *v1.Pod {
	pod := psm.getProviderPod()
	if pod != nil {
		return pod
	}
	return psm.getPodControllerPod()
}

func (psm *podStateMachine) storePodControllerPod(pod *v1.Pod) (*v1.Pod, bool) {
	psm.podLock.Lock()
	defer psm.podLock.Unlock()

	var currentResourceVersion, newResourceVersion int
	if psm.podControllerPod.ObjectMeta.ResourceVersion != "" {
		if v, err := strconv.Atoi(psm.podControllerPod.ObjectMeta.ResourceVersion); err == nil {
			currentResourceVersion = v
		}
	}

	if pod.ObjectMeta.ResourceVersion != "" {
		if v, err := strconv.Atoi(pod.ObjectMeta.ResourceVersion); err == nil {
			newResourceVersion = v
		}
	}

	if newResourceVersion >= currentResourceVersion {
		psm.podControllerPod = pod
		return pod, true
	}
	return psm.podControllerPod, false
}

func (psm *podStateMachine) getPodControllerPod() *v1.Pod {
	psm.podLock.RLock()
	defer psm.podLock.RUnlock()
	return psm.podControllerPod
}

func (psm *podStateMachine) updateProviderPod(ctx context.Context, pod *v1.Pod) {
	if _, stored := psm.storeProviderPod(pod); !stored {
		return
	}
	select {
	case psm.providerDirtyCh <- struct{}{}:
	default:
	}
}

func (psm *podStateMachine) updatePodControllerPod(ctx context.Context, pod *v1.Pod) {
	if _, stored := psm.storePodControllerPod(pod.DeepCopy()); !stored {
		return
	}

	select {
	case psm.podControllerPodDirtyCh <- struct{}{}:
	default:
	}
}

func (psm *podStateMachine) providerHasPod(ctx context.Context) (bool, error) {
	pods, err := psm.pc.server.provider.GetPods(ctx)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		if pod.Name == psm.name && pod.Namespace == psm.namespace {
			return true, nil
			// TODO: Do not make this hardcoded
		}
	}
	return false, nil
}

// State helper functions
func (psm *podStateMachine) nonTerminalStateHelper(ctx context.Context, lastState state) (state, error) {
	select {
	case <-psm.providerDirtyCh:
		// We need to update the state of the pod in the kubelet apiserver
		pod := psm.getProviderPod()
		psm.sendUpdate(ctx)
		// Check if the provider pod has transitioned to a non-starting state
		switch pod.Status.Phase {
		case v1.PodPending:
			return lastState, nil
		case v1.PodRunning:
			return psm.runningState, nil
		case v1.PodFailed, v1.PodSucceeded:
			return psm.terminalState, nil
		default:
			log.G(ctx).WithField("state", pod.Status.Phase).Warn("Moved into unexpected state, not transitioning pod")
			return lastState, nil
		}
	case <-psm.sendUpdateRetry.C:
		psm.sendUpdate(ctx)
		return lastState, nil
	case <-psm.podControllerPodDirtyCh:
		pod := psm.getPodControllerPod()
		if pod.DeletionTimestamp != nil {
			return psm.deletePodState, nil
		}
		// TODO: This is really important in order to do graceful pod deletion
		log.G(ctx).WithField("pod", pod).Info("Received pod update")
		return lastState, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// States
func (psm *podStateMachine) createdFromProviderState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "createdFromProviderState")
	defer span.End()

	select {
	case <-psm.pc.inSyncCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if psm.getPodControllerPod() != nil {
		return psm.createdInProviderState, nil
	}

	pod := psm.getProviderPod()

	_, err := psm.pc.podsLister.Pods(pod.Namespace).Get(pod.Name)
	if errors.IsNotFound(err) {
		return psm.deleteDanglingPodState, nil
	}

	return psm.createdInProviderState, nil
}

// The provider never found out that this went poorly.
func (psm *podStateMachine) wrapPodCreationFailedState(origErr error) state {
	return func(ctx context.Context) (state, error) {
		return psm.podCreationFailedState(ctx, origErr)
	}
}

// You shouldn't (can't) refer to this state directly
func (psm *podStateMachine) podCreationFailedState(ctx context.Context, origErr error) (state, error) {
	ctx, span := trace.StartSpan(ctx, "podCreationFailedState")
	defer span.End()

	pod := psm.getPodControllerPod()
	if pod == nil {
		log.G(ctx).Warn("Pod state machine transitioned to podCreationFailedState without a pod controller provided pod")
		pod = psm.getProviderPod()
	}
	if pod == nil {
		panic("Could not fetch pod controller from pod controller or provider.")
	}

	pod.Status.Phase = v1.PodPending
	if pod.Spec.RestartPolicy == v1.RestartPolicyNever {
		pod.Status.Phase = v1.PodFailed
	}
	pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	pod.Status.Reason = podStatusReasonProviderFailed
	pod.Status.Message = origErr.Error()
	psm.sendUpdate(ctx)

	return psm.terminalState, nil
}

func (psm *podStateMachine) createdFromAPIServerState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "createdFromAPIServerState")
	defer span.End()

	pod, err := psm.pc.podsLister.Pods(psm.namespace).Get(psm.name)
	// Copy the pod object so we don't have to deal with manipulation of the pod cache
	pod = pod.DeepCopy()
	if errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Warn("Pod not found while launching")
		span.SetStatus(ocstatus.FromError(err))
		return nil, err
	}
	pod, _ = psm.storePodControllerPod(pod)

	// Check if this in a terminal state
	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed || pod.DeletionTimestamp != nil {
		return psm.waitForAPIServer, nil
	}

	if err := populateEnvironmentVariables(ctx, pod, psm.pc.server.resourceManager, psm.pc.recorder); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return psm.wrapPodCreationFailedState(err), err
	}
	// TODO: pod creation can be interrupted, if we get a termination / deletion message in the interim
	// This should be turned into an async invocation, which in turn can be cancelled if it fails.

	// We also pass a copy of the pod to the provider, because we can't trust the provider
	err = psm.pc.server.provider.CreatePod(ctx, pod.DeepCopy())
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		// The pod is failed. Mark it so, and update kubernetes to tell it it has failed.
		return psm.wrapPodCreationFailedState(err), nil
	}

	return psm.createdInProviderState, nil
}

func (psm *podStateMachine) createdInProviderState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "createdInProvider")
	defer span.End()

	return psm.nonTerminalStateHelper(ctx, psm.createdInProviderState)
}

func (psm *podStateMachine) runningState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "runningState")
	defer span.End()
	return psm.nonTerminalStateHelper(ctx, psm.runningState)
}

// waitForAPIServer means the pod does exists in apiserver, but does not exist in the provider.
// we will send queued up updates to the apiserver.
func (psm *podStateMachine) waitForAPIServer(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "waitForAPIServer")
	defer span.End()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-psm.providerDirtyCh:
		psm.sendUpdate(ctx)
		return psm.waitForAPIServer, nil
	case <-psm.sendUpdateRetry.C:
		psm.sendUpdate(ctx)
		return psm.waitForAPIServer, nil
	case <-psm.podControllerPodDirtyCh:
	case <-time.After(time.Minute):
	}

	if _, err := psm.pc.podsLister.Pods(psm.namespace).Get(psm.name); errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		log.G(ctx).WithError(err).Info("Received unexpected error in waitForAPIServer state")
		return psm.waitForAPIServer, nil
	}
	return psm.waitForAPIServer, nil
}

// terminalState indicates that locally, the pod has entered into a terminal state, and we need to update the pod in
// api server, if the pod disappears from the provider, we can transition to waitForAPIServer. If the pod is no longer
// in api server, we can move to waitForPodDeletionState
func (psm *podStateMachine) terminalState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "terminalState")
	defer span.End()

	if hasPod, err := psm.providerHasPod(ctx); err != nil {
		log.G(ctx).WithError(err).Warn("Unable to determine if provider has pod")
	} else if !hasPod {
		return psm.waitForAPIServer, nil
	}

	select {
	case <-time.After(10 * time.Second):
		return psm.terminalState, nil
	case <-psm.sendUpdateRetry.C:
		psm.sendUpdate(ctx)
		return psm.terminalState, nil
	case <-psm.providerDirtyCh:
		psm.sendUpdate(ctx)
		return psm.terminalState, nil
	case <-psm.podControllerPodDirtyCh:
		// Check if the pod disappeared from API Server. Otherwise if we got an update from apiserver,
		// it is inconsequential as we have no way of feeding that back down to the VK.
		if _, err := psm.pc.podsLister.Pods(psm.namespace).Get(psm.name); errors.IsNotFound(err) {
			return psm.waitForPodDeletionState, nil
		} else {
			log.G(ctx).WithError(err).Warn("Cannot list pods from pod lister")
		}
		return psm.terminalState, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// deletePodState terminates the pod in the provider. It transitions into terminalState once the pod
// is marked with a terminal state in the provider.
func (psm *podStateMachine) deletePodState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "waitForAPIServer")
	defer span.End()

	if hasPod, err := psm.providerHasPod(ctx); err != nil {
		log.G(ctx).WithError(err).Warn("Unable to determine if provider has pod")
	} else if !hasPod {
		return psm.terminalState, nil
	}

	pod := psm.getProviderPod()
	if pod == nil {
		panic("Could not get provider pod")
	}
	err := psm.pc.server.provider.DeletePod(ctx, pod)
	if err == nil {
		return psm.terminalState, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	nextState, err := psm.nonTerminalStateHelper(ctx, psm.deletePodState)
	// Timeout -- our poor man's restart
	if err == ctx.Err() {
		return psm.deletePodState, nil
	}
	return nextState, err
}

func (psm *podStateMachine) deleteDanglingPodState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "deleteDanglingPodState")
	defer span.End()

	pod := psm.getProviderPod()
	err := psm.pc.server.provider.DeletePod(ctx, pod)
	if err == nil {
		return psm.waitForPodDeletionState, nil
	}

	// Even if err is not nil, check if the pod was deleted
	hasPod, err2 := psm.providerHasPod(ctx)
	if err2 != nil {
		log.G(ctx).WithError(err2).Error("Error fetching pods from providers")
	} else if !hasPod {
		return psm.waitForPodDeletionState, nil
	}

	span.SetStatus(ocstatus.FromError(err))
	log.G(ctx).WithError(err).Error("Could not delete pod from provider, retrying")

	// TODO: Do not make this hardcoded
	select {
	case <-time.After(10 * time.Second):
		return psm.deleteDanglingPodState, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (psm *podStateMachine) waitForPodDeletionState(ctx context.Context) (state, error) {
	ctx, span := trace.StartSpan(ctx, "waitForPodDeletionState")
	defer span.End()

	hasPod, err := psm.providerHasPod(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Error("Error fetching pods from providers")
		span.SetStatus(ocstatus.FromError(err))
	} else if !hasPod {
		return nil, nil
	}

	// TODO: Do not make this hardcoded
	select {
	case <-time.After(10 * time.Second):
		return psm.waitForPodDeletionState, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendUpdate tries to take our provider state and send an update based on it to the apiserver.
func (psm *podStateMachine) sendUpdate(ctx context.Context) {
	// TODO: Timeouts
	ctx, span := trace.StartSpan(ctx, "sendUpdate")
	defer span.End()

	// It might have an error for another reason that we aren't worried about. Nonetheless, surpress the update
	// if API server doesn't know about this pod
	if _, err := psm.pc.podsLister.Pods(psm.namespace).Get(psm.name); errors.IsNotFound(err) {
		return
	}

	pod := psm.getProviderPod()
	if pod == nil {
		pod = psm.getPodControllerPod()
		if pod == nil {
			panic("Could not fetch pod status to send")
		}
		log.G(ctx).Debug("Could not fetch pod status from provider, updating from pod controller")

	}
	// We copy the pods in order to make sure the provider / podinformer cache doesn't mutate them underneath us
	// and so we can mutate it freely
	pod = pod.DeepCopy()
	if psm.shouldSuppressUpdate(ctx, pod) {
		return
	}

	pod.ObjectMeta.ResourceVersion = ""
	patch, err := json.Marshal(pod)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).WithError(err).Error("Unable to serialize patch JSON")
		// TODO: Make this not time.second
		psm.sendUpdateRetry.Reset(time.Second)
		return
	}

	newPod, err := psm.pc.server.k8sClient.CoreV1().Pods(pod.Namespace).Patch(pod.Name, types.MergePatchType, patch, "status")
	if errors.IsNotFound(err) {
		log.G(ctx).Warn("Trying to send update for non-existent pod")
		return
	} else if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).WithError(err).Error("error while patching pod status in kubernetes")
		psm.sendUpdateRetry.Reset(time.Second)
		return
	}
	// We don't need to copy it here, because we should never mutate the pod that we put into podControllerPod. In addition,
	// we do a deepcopy before we copy it from the cache into our state
	psm.lastPatchResponsePod = newPod
	psm.lastPodUsedForPatch = pod

	log.G(ctx).WithFields(log.Fields{
		"old phase":  string(pod.Status.Phase),
		"old reason": pod.Status.Reason,
		"new phase":  string(newPod.Status.Phase),
		"new reason": newPod.Status.Reason,
	}).Debug("Updated pod status in kubernetes")
	psm.storePodControllerPod(newPod)

}

func (psm *podStateMachine) shouldSuppressUpdate(ctx context.Context, newPod *v1.Pod) bool {
	ctx, span := trace.StartSpan(ctx, "shouldSuppressUpdate")
	defer span.End()

	// Algorithm goes:
	// Have we ever sent an update? If not send it.
	// Is the pod controller's pod version greater than than the version of the response we got from the last successful patch? If so, send it
	// Is the content of this pod's status different than the content of the last pod we used?

	// Have we ever sent an update?
	if psm.lastPatchResponsePod == nil || psm.lastPodUsedForPatch == nil {
		log.G(ctx).Debug("Sending update because last pod status update is nil")
		return false
	}

	// Now we check to see if the pod controller pod has a status that is the same as ours. If not, the status could
	// have been updated elsewhere, and the provider is trying to resync. That, or the provider is just being dumb
	pcPod := psm.getPodControllerPod()
	if pcPod == nil {
		log.G(ctx).Debug("Provider pod is nil")
		return false
	}

	// Generally, this shouldn't happen
	if pcPod.ObjectMeta.ResourceVersion == "" {
		return false
	}
	if psm.lastPatchResponsePod.ObjectMeta.ResourceVersion == "" {
		return false
	}

	pcPodResourceVersion, err := strconv.Atoi(pcPod.ObjectMeta.ResourceVersion)
	if err != nil {
		err := pkgerrors.Wrap(err, "Failed to parse resource version of pod-controller provided pod")
		log.G(ctx).WithField("resourceVersion", pcPod.ResourceVersion).WithError(err).Warn()
		span.SetStatus(ocstatus.FromError(err))
	}

	lastPatchResponsePodVersion, err := strconv.Atoi(psm.lastPatchResponsePod.ObjectMeta.ResourceVersion)
	if err != nil {
		err := pkgerrors.Wrap(err, "Failed to parse resource version of patch-response provided pod")
		log.G(ctx).WithField("resourceVersion", pcPod.ResourceVersion).WithError(err).Warn()
		span.SetStatus(ocstatus.FromError(err))
	}

	// The pod controller somehow has a newer version than what we got from the patch request
	if pcPodResourceVersion > lastPatchResponsePodVersion {
		return false
	}

	if !reflect.DeepEqual(newPod.Status, psm.lastPodUsedForPatch.Status) {
		return false
	}

	return true
}
