package vkubelet

import (
	"context"
	"fmt"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func addPodAttributes(span *trace.Span, pod *corev1.Pod) {
	span.AddAttributes(
		trace.StringAttribute("uid", string(pod.GetUID())),
		trace.StringAttribute("namespace", pod.GetNamespace()),
		trace.StringAttribute("name", pod.GetName()),
		trace.StringAttribute("phase", string(pod.Status.Phase)),
		trace.StringAttribute("reason", pod.Status.Reason),
	)
}

func (s *Server) onAddPod(ctx context.Context, obj interface{}) {
	ctx, span := trace.StartSpan(ctx, "onAddPod")
	defer span.End()
	logger := log.G(ctx).WithField("method", "onAddPod")

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: fmt.Sprintf("Unexpected object from event: %T", obj)})
		logger.Errorf("obj is not of a valid type: %T", obj)
		return
	}

	addPodAttributes(span, pod)

	logger.Debugf("Receive added pod '%s/%s' ", pod.GetNamespace(), pod.GetName())

	if s.resourceManager.UpdatePod(pod) {
		span.Annotate(nil, "Add pod to synchronizer channel.")
		select {
		case <-ctx.Done():
			logger = logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
			logger.WithError(ctx.Err()).Debug("Cancel send pod event due to cancelled context")
			return
		case s.podCh <- &podNotification{pod: pod, ctx: ctx}:
		}
	}
}

func (s *Server) onUpdatePod(ctx context.Context, obj interface{}) {
	ctx, span := trace.StartSpan(ctx, "onUpdatePod")
	defer span.End()
	logger := log.G(ctx).WithField("method", "onUpdatePod")

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: fmt.Sprintf("Unexpected object from event: %T", obj)})
		logger.Errorf("obj is not of a valid type: %T", obj)
		return
	}

	addPodAttributes(span, pod)

	logger.Debugf("Receive updated pod '%s/%s'", pod.GetNamespace(), pod.GetName())

	if s.resourceManager.UpdatePod(pod) {
		span.Annotate(nil, "Add pod to synchronizer channel.")
		select {
		case <-ctx.Done():
			logger = logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
			logger.WithError(ctx.Err()).Debug("Cancel send pod event due to cancelled context")
			return
		case s.podCh <- &podNotification{pod: pod, ctx: ctx}:
		}
	}
}

func (s *Server) onDeletePod(ctx context.Context, obj interface{}) {
	ctx, span := trace.StartSpan(ctx, "onDeletePod")
	defer span.End()
	logger := log.G(ctx).WithField("method", "onDeletePod")

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		delta, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: fmt.Sprintf("Unexpected object from event: %T", obj)})
			logger.Errorf("obj is not of a valid type: %T", obj)
			return
		}

		if pod, ok = delta.Obj.(*corev1.Pod); !ok {
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: fmt.Sprintf("Unexpected object from event: %T", obj)})
			logger.Errorf("obj is not of a valid type: %T", obj)
			return
		}
	}

	addPodAttributes(span, pod)

	logger.Debugf("Receive deleted pod '%s/%s'", pod.GetNamespace(), pod.GetName())

	if s.resourceManager.DeletePod(pod) {
		span.Annotate(nil, "Add pod to synchronizer channel.")
		select {
		case <-ctx.Done():
			logger = logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
			logger.WithError(ctx.Err()).Debug("Cancel send pod event due to cancelled context")
			return
		case s.podCh <- &podNotification{pod: pod, ctx: ctx}:
		}
	}
}

func (s *Server) startPodSynchronizer(ctx context.Context, id int) {
	logger := log.G(ctx).WithField("method", "startPodSynchronizer").WithField("podSynchronizer", id)
	logger.Debug("Start pod synchronizer")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stop pod syncronizer")
			return
		case event := <-s.podCh:
			s.syncPod(event.ctx, event.pod)
		}
	}
}

func (s *Server) syncPod(ctx context.Context, pod *corev1.Pod) {
	ctx, span := trace.StartSpan(ctx, "syncPod")
	defer span.End()
	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())

	addPodAttributes(span, pod)

	if pod.DeletionTimestamp != nil {
		span.Annotate(nil, "Delete pod")
		logger.Debugf("Deleting pod")
		if err := s.deletePod(ctx, pod); err != nil {
			logger.WithError(err).Error("Failed to delete pod")
		}
	} else {
		span.Annotate(nil, "Create pod")
		logger.Debugf("Creating pod")
		if err := s.createPod(ctx, pod); err != nil {
			logger.WithError(err).Errorf("Failed to create pod")
		}
	}
}

func (s *Server) createPod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "createPod")
	defer span.End()
	addPodAttributes(span, pod)

	if err := s.populateEnvironmentVariables(pod); err != nil {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: err.Error()})
		return err
	}

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
		podPhase := corev1.PodPending
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			podPhase = corev1.PodFailed
		}

		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = podPhase
		pod.Status.Reason = podStatusReasonProviderFailed
		pod.Status.Message = origErr.Error()

		_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		if err != nil {
			logger.WithError(err).Warn("Failed to update pod status")
		} else {
			span.Annotate(nil, "Updated k8s pod status")
		}

		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: origErr.Error()})
		return origErr
	}
	span.Annotate(nil, "Created pod in provider")

	logger.Info("Pod created")

	return nil
}

func (s *Server) deletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	addPodAttributes(span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: delErr.Error()})
		return delErr
	}
	span.Annotate(nil, "Deleted pod from provider")

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
	if !errors.IsNotFound(delErr) {
		var grace int64
		if err := s.k8sClient.CoreV1().Pods(pod.GetNamespace()).Delete(pod.GetName(), &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && errors.IsNotFound(err) {
			if errors.IsNotFound(err) {
				span.Annotate(nil, "Pod does not exist in k8s, nothing to delete")
				return nil
			}

			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
			return fmt.Errorf("Failed to delete kubernetes pod: %s", err)
		}
		span.Annotate(nil, "Deleted pod from k8s")

		s.resourceManager.DeletePod(pod)
		span.Annotate(nil, "Deleted pod from internal state")
		logger.Info("Pod deleted")
	}

	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updatePodStatuses")
	defer span.End()

	// Update all the pods with the provider status.
	pods := s.resourceManager.GetPods()
	span.AddAttributes(trace.Int64Attribute("nPods", int64(len(pods))))

	for _, pod := range pods {
		select {
		case <-ctx.Done():
			span.Annotate(nil, ctx.Err().Error())
			return
		default:
		}

		if err := s.updatePodStatus(ctx, pod); err != nil {
			logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace()).WithField("status", pod.Status.Phase).WithField("reason", pod.Status.Reason)
			logger.Error(err)
		}
	}
}

func (s *Server) updatePodStatus(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "updatePodStatus")
	defer span.End()
	addPodAttributes(span, pod)

	if pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed ||
		pod.Status.Reason == podStatusReasonProviderFailed {
		return nil
	}

	status, err := s.provider.GetPodStatus(ctx, pod.Namespace, pod.Name)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "error retreiving pod status")
	}

	// Update the pod's status
	if status != nil {
		pod.Status = *status
	} else {
		// Only change the status when the pod was already up
		// Only doing so when the pod was successfully running makes sure we don't run into race conditions during pod creation.
		if pod.Status.Phase == corev1.PodRunning || pod.ObjectMeta.CreationTimestamp.Add(time.Minute).Before(time.Now()) {
			// Set the pod to failed, this makes sure if the underlying container implementation is gone that a new pod will be created.
			pod.Status.Phase = corev1.PodFailed
			pod.Status.Reason = "NotFound"
			pod.Status.Message = "The pod status was not found and may have been deleted from the provider"
			for i, c := range pod.Status.ContainerStatuses {
				pod.Status.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    -137,
					Reason:      "NotFound",
					Message:     "Container was not found and was likely deleted",
					FinishedAt:  metav1.NewTime(time.Now()),
					StartedAt:   c.State.Running.StartedAt,
					ContainerID: c.ContainerID,
				}
				pod.Status.ContainerStatuses[i].State.Running = nil
			}
		}
	}

	if _, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "error while updating pod status in kubernetes")
	}

	span.Annotate([]trace.Attribute{
		trace.StringAttribute("new phase", string(pod.Status.Phase)),
		trace.StringAttribute("new reason", pod.Status.Reason),
	}, "updated pod status in kubernetes")
	return nil
}

// watchForPodEvent waits for pod changes from kubernetes and updates the details accordingly in the local state.
// This returns after a single pod event.
func (s *Server) watchForPodEvent(ctx context.Context) error {
	opts := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", s.nodeName).String(),
	}

	pods, err := s.k8sClient.CoreV1().Pods(s.namespace).List(opts)
	if err != nil {
		return pkgerrors.Wrap(err, "error getting pod list")
	}

	s.resourceManager.SetPods(pods)
	s.reconcile(ctx)

	opts.ResourceVersion = pods.ResourceVersion

	var controller cache.Controller
	_, controller = cache.NewInformer(

		&cache.ListWatch{

			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if controller != nil {
					opts.ResourceVersion = controller.LastSyncResourceVersion()
				}

				return s.k8sClient.Core().Pods(s.namespace).List(opts)
			},

			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if controller != nil {
					opts.ResourceVersion = controller.LastSyncResourceVersion()
				}

				return s.k8sClient.Core().Pods(s.namespace).Watch(opts)
			},
		},

		&corev1.Pod{},

		time.Minute,

		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				s.onAddPod(ctx, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				s.onUpdatePod(ctx, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				s.onDeletePod(ctx, obj)
			},
		},
	)

	for i := 0; i < s.podSyncWorkers; i++ {
		go s.startPodSynchronizer(ctx, i)
	}

	log.G(ctx).Info("Start to run pod cache controller.")
	controller.Run(ctx.Done())

	return ctx.Err()
}
