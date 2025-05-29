package node

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	watchutils "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

var (
	_ record.EventRecorder = (*fakeDiscardingRecorder)(nil)
)

const (
	// There might be a constant we can already leverage here
	testNamespace        = "default"
	informerResyncPeriod = 1 * time.Second
	testNodeName         = "testnode"
	podSyncWorkers       = 3
)

func init() {
	klog.InitFlags(nil)
	// We neet to set log.L because new spans derive their loggers from log.L
	sl := logrus.StandardLogger()
	sl.SetLevel(logrus.DebugLevel)
	newLogger := logruslogger.FromLogrus(logrus.NewEntry(sl))
	log.L = newLogger
}

// fakeDiscardingRecorder discards all events. Silently.
type fakeDiscardingRecorder struct {
	logger log.Logger
}

func (r *fakeDiscardingRecorder) Event(object runtime.Object, eventType, reason, message string) {
	r.Eventf(object, eventType, reason, "%s", message)
}

func (r *fakeDiscardingRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	r.logger.WithFields(map[string]interface{}{
		"object":    object,
		"eventType": eventType,
		"message":   fmt.Sprintf(messageFmt, args...),
	}).Infof("Received event")
}

func (r *fakeDiscardingRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventType, reason, messageFmt string, args ...interface{}) {
	r.logger.WithFields(map[string]interface{}{
		"timestamp": timestamp.String(),
		"object":    object,
		"eventType": eventType,
		"message":   fmt.Sprintf(messageFmt, args...),
	}).Infof("Received past event")
}

func (r *fakeDiscardingRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	r.logger.WithFields(map[string]interface{}{
		"object":      object,
		"annotations": annotations,
		"eventType":   eventType,
		"message":     fmt.Sprintf(messageFmt, args...),
	}).Infof("Received annotated event")
}

func TestPodLifecycle(t *testing.T) {
	// We don't do the defer cancel() thing here because t.Run is non-blocking, so the parent context may be cancelled
	// before the children are finished and there is no way to do a "join" and wait for them without using a waitgroup,
	// at which point, it doesn't seem much better.
	ctx := context.Background()

	ctx = log.WithLogger(ctx, log.L)

	// isPodDeletedPermanentlyFunc is a condition func that waits until the pod is _deleted_
	isPodDeletedPermanentlyFunc := func(ctx context.Context, watcher watch.Interface) error {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher, func(ev watch.Event) (bool, error) {
			log.G(ctx).WithField("event", ev).Info("got event")
			// TODO(Sargun): The pod should have transitioned into some status around failed / succeeded
			// prior to being deleted.
			// In addition, we should check if the deletion timestamp gets set
			return ev.Type == watch.Deleted, nil
		})
		return watchErr
	}

	// isPodDeletedGracefullyFunc is a condition func that waits until the pod is in a terminal state, which is the VK's
	// action when the pod is deleted from the provider.
	isPodDeletedGracefullyFunc := func(ctx context.Context, watcher watch.Interface) error {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher, func(ev watch.Event) (bool, error) {
			log.G(ctx).WithField("event", ev).Info("got event")
			pod := ev.Object.(*corev1.Pod)
			return (pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded) && (pod.Status.Reason == mockProviderPodDeletedReason || pod.Status.Reason == statusTerminatedReason), nil
		})
		return watchErr
	}

	envs := map[string]func(*testing.T) testingProvider{
		"Async": func(t *testing.T) testingProvider {
			return newMockProvider()
		},
		"Sync": func(t *testing.T) testingProvider {
			if testing.Short() {
				t.Skip()
			}
			return newSyncMockProvider()
		},
	}

	for env, h := range envs {
		t.Run(env, func(t *testing.T) {

			// createStartDeleteScenario tests the basic flow of creating a pod, waiting for it to start, and deleting
			// it gracefully.
			t.Run("createStartDeleteScenario", func(t *testing.T) {
				assert.NilError(t, wireUpSystem(ctx, h(t), func(ctx context.Context, s *system) {
					testCreateStartDeleteScenario(ctx, t, s, isPodDeletedGracefullyFunc, true)
				}))
			})

			// createStartDeleteScenarioWithDeletionErrorNotFound tests the flow if the pod was not found in the provider
			// for some reason.
			t.Run("createStartDeleteScenarioWithDeletionErrorNotFound", func(t *testing.T) {
				mp := h(t)
				mp.setErrorOnDelete(errdefs.NotFound("not found"))
				assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
					testCreateStartDeleteScenario(ctx, t, s, isPodDeletedPermanentlyFunc, false)
				}))
			})

			// createStartDeleteScenarioWithDeletionRandomError tests the flow if the pod was unable to be deleted in the
			// provider.
			t.Run("createStartDeleteScenarioWithDeletionRandomError", func(t *testing.T) {
				mp := h(t)
				deletionFunc := func(ctx context.Context, watcher watch.Interface) error {
					return mp.getAttemptedDeletes().until(ctx, func(v int) bool { return v >= 2 })
				}
				mp.setErrorOnDelete(errors.New("random error"))
				assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
					testCreateStartDeleteScenario(ctx, t, s, deletionFunc, false)
					pods, err := s.client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{})
					assert.NilError(t, err)
					assert.Assert(t, is.Len(pods.Items, 1))
					assert.Assert(t, pods.Items[0].DeletionTimestamp != nil)
				}))
			})

			// danglingPodScenario tests if a pod is created in the provider prior to the pod controller starting,
			// and ensures the pod controller deletes the pod prior to continuing.
			t.Run("danglingPodScenario", func(t *testing.T) {
				t.Run("mockProvider", func(t *testing.T) {
					mp := h(t)
					assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
						testDanglingPodScenario(ctx, t, s, mp)
					}))
				})
			})

			// testDanglingPodScenarioWithDeletionTimestamp tests if a pod is created in the provider and on api server it had
			// deletiontimestamp set. It ensures deletion occurs.
			t.Run("testDanglingPodScenarioWithDeletionTimestamp", func(t *testing.T) {
				t.Run("mockProvider", func(t *testing.T) {
					mp := h(t)
					assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
						testDanglingPodScenarioWithDeletionTimestamp(ctx, t, s, mp)
					}))
				})
			})

			// failedPodScenario ensures that the VK ignores failed pods that were failed prior to the pod controller starting up.
			t.Run("failedPodScenario", func(t *testing.T) {
				t.Run("mockProvider", func(t *testing.T) {
					mp := h(t)
					assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
						testFailedPodScenario(ctx, t, s)
					}))
				})
			})

			// succeededPodScenario ensures that the VK ignores succeeded pods that were succeeded prior to the pod controller starting up.
			t.Run("succeededPodScenario", func(t *testing.T) {
				t.Run("mockProvider", func(t *testing.T) {
					mp := h(t)
					assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
						testSucceededPodScenario(ctx, t, s)
					}))
				})
			})

			// updatePodWhileRunningScenario updates a pod while the VK is running to ensure the update is propagated
			// to the provider.
			t.Run("updatePodWhileRunningScenario", func(t *testing.T) {
				t.Run("mockProvider", func(t *testing.T) {
					mp := h(t)
					assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
						testUpdatePodWhileRunningScenario(ctx, t, s, mp)
					}))
				})
			})
		})
	}
}

type testFunction func(ctx context.Context, s *system)
type system struct {
	pc                  *PodController
	client              kubernetes.Interface
	podControllerConfig PodControllerConfig
}

func (s *system) start(ctx context.Context) error {
	go s.pc.Run(ctx, podSyncWorkers) //nolint:errcheck
	select {
	case <-s.pc.Ready():
	case <-s.pc.Done():
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.pc.Err()
}

func wireUpSystem(ctx context.Context, provider PodLifecycleHandler, f testFunction) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create the fake client.
	client := fake.NewSimpleClientset()

	client.PrependReactor("update", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		var pod *corev1.Pod

		updateAction := action.(ktesting.UpdateAction)
		pod = updateAction.GetObject().(*corev1.Pod)

		resourceVersion, err := strconv.Atoi(pod.ResourceVersion)
		if err != nil {
			panic(errors.Wrap(err, "Could not parse resource version of pod"))
		}
		pod.ResourceVersion = strconv.Itoa(resourceVersion + 1)
		return false, nil, nil
	})

	return wireUpSystemWithClient(ctx, provider, client, f)
}

func wireUpSystemWithClient(ctx context.Context, provider PodLifecycleHandler, client kubernetes.Interface, f testFunction) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// This is largely copy and pasted code from the root command
	sharedInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client,
		informerResyncPeriod,
	)
	podInformer := sharedInformerFactory.Core().V1().Pods()

	eb := record.NewBroadcaster()
	eb.StartLogging(log.G(ctx).Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: client.CoreV1().Events(testNamespace)})
	fakeRecorder := &fakeDiscardingRecorder{
		logger: log.G(ctx),
	}

	secretInformer := sharedInformerFactory.Core().V1().Secrets()
	configMapInformer := sharedInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := sharedInformerFactory.Core().V1().Services()
	sys := &system{
		client: client,
		podControllerConfig: PodControllerConfig{
			PodClient:         client.CoreV1(),
			PodInformer:       podInformer,
			EventRecorder:     fakeRecorder,
			Provider:          provider,
			ConfigMapInformer: configMapInformer,
			SecretInformer:    secretInformer,
			ServiceInformer:   serviceInformer,
		},
	}

	var err error
	sys.pc, err = NewPodController(sys.podControllerConfig)
	if err != nil {
		return err
	}

	go sharedInformerFactory.Start(ctx.Done())
	sharedInformerFactory.WaitForCacheSync(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced); !ok {
		return errors.New("podinformer failed to sync")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	f(ctx, sys)

	// Shutdown the pod controller, and wait for it to exit
	cancel()
	return nil
}

func testFailedPodScenario(ctx context.Context, t *testing.T, s *system) {
	testTerminalStatePodScenario(ctx, t, s, corev1.PodFailed)
}

func testSucceededPodScenario(ctx context.Context, t *testing.T, s *system) {
	testTerminalStatePodScenario(ctx, t, s, corev1.PodSucceeded)
}
func testTerminalStatePodScenario(ctx context.Context, t *testing.T, s *system, state corev1.PodPhase) {

	t.Parallel()

	p1 := newPod()
	p1.Status.Phase = state
	// Create the Pod
	_, e := s.client.CoreV1().Pods(testNamespace).Create(ctx, p1, metav1.CreateOptions{})
	assert.NilError(t, e)

	// Start the pod controller
	assert.NilError(t, s.start(ctx))

	for !s.pc.syncPodsFromKubernetes.Empty() {
		time.Sleep(10 * time.Millisecond)
	}

	p2, err := s.client.CoreV1().Pods(testNamespace).Get(ctx, p1.Name, metav1.GetOptions{})
	assert.NilError(t, err)

	// Make sure the pods have not changed
	assert.DeepEqual(t, p1, p2)
}

func testDanglingPodScenario(ctx context.Context, t *testing.T, s *system, m testingProvider) {
	t.Parallel()

	pod := newPod()
	assert.NilError(t, m.CreatePod(ctx, pod))

	// Start the pod controller
	assert.NilError(t, s.start(ctx))

	assert.Assert(t, is.Equal(m.getDeletes().read(), 1))

}

func sendErr(ctx context.Context, ch chan error, err error) {
	select {
	case <-ctx.Done():
		log.G(ctx).WithError(err).Warn("timeout waiting to send test error")
	case ch <- err:
	}
}

func testDanglingPodScenarioWithDeletionTimestamp(ctx context.Context, t *testing.T, s *system, m testingProvider) {
	t.Parallel()

	pod := newPod()
	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", pod.ObjectMeta.Name).String(),
	}
	// Setup a watch (prior to pod creation, and pod controller startup)
	watcher, err := s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher.Stop()

	assert.NilError(t, m.CreatePod(ctx, pod))

	// Create the Pod
	podCopyWithDeletionTimestamp := pod.DeepCopy()
	var deletionGracePeriod int64 = 10
	podCopyWithDeletionTimestamp.DeletionGracePeriodSeconds = &deletionGracePeriod
	deletionTimestamp := metav1.NewTime(time.Now().Add(time.Second * time.Duration(deletionGracePeriod)))
	podCopyWithDeletionTimestamp.DeletionTimestamp = &deletionTimestamp
	_, e := s.client.CoreV1().Pods(testNamespace).Create(ctx, podCopyWithDeletionTimestamp, metav1.CreateOptions{})
	assert.NilError(t, e)

	watchErrCh := make(chan error)
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			func(ev watch.Event) (bool, error) {
				return ev.Type == watch.Deleted, nil
			})
		sendErr(ctx, watchErrCh, watchErr)
	}()

	// Start the pod controller
	assert.NilError(t, s.start(ctx))

	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)
	}
}

func testCreateStartDeleteScenario(ctx context.Context, t *testing.T, s *system, waitFunction func(ctx context.Context, watch watch.Interface) error, waitForPermanentDeletion bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p := newPod()

	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.ObjectMeta.Name).String(),
	}

	watchErrCh := make(chan error)

	// Setup a watch (prior to pod creation, and pod controller startup)
	watcher, err := s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher.Stop()
	// This ensures that the pod is created.
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be created
			// TODO(Sargun): Make this "smarter" about the status the pod is in.
			func(ev watch.Event) (bool, error) {
				pod := ev.Object.(*corev1.Pod)
				return pod.Name == p.Name, nil
			})

		sendErr(ctx, watchErrCh, watchErr)
	}()

	// Create the Pod
	_, e := s.client.CoreV1().Pods(testNamespace).Create(ctx, p, metav1.CreateOptions{})
	assert.NilError(t, e)
	log.G(ctx).Debug("Created pod")

	// This will return once
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)

	}

	// Setup a watch to check if the pod is in running
	watcher, err = s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher.Stop()
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be started
			func(ev watch.Event) (bool, error) {
				pod := ev.Object.(*corev1.Pod)
				return pod.Status.Phase == corev1.PodRunning, nil
			})

		sendErr(ctx, watchErrCh, watchErr)
	}()

	assert.NilError(t, s.start(ctx))

	// Wait for the pod to go into running
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)
	}

	// Setup a watch to look for the pod eventually going away completely
	watcher2, err := s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher2.Stop()
	waitDeleteCh := make(chan error)
	go func() {
		_, watchDeleteErr := watchutils.UntilWithoutRetry(ctx, watcher2, func(ev watch.Event) (bool, error) {
			return ev.Type == watch.Deleted, nil
		})
		sendErr(ctx, waitDeleteCh, watchDeleteErr)
	}()

	// Setup a watch prior to pod deletion
	watcher, err = s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher.Stop()
	go func() {
		sendErr(ctx, watchErrCh, waitFunction(ctx, watcher))
	}()

	// Delete the pod via deletiontimestamp

	// 1. Get the pod
	currentPod, err := s.client.CoreV1().Pods(testNamespace).Get(ctx, p.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	// 2. Set the pod's deletion timestamp, version, and so on
	var deletionGracePeriod int64 = 10
	currentPod.DeletionGracePeriodSeconds = &deletionGracePeriod
	deletionTimestamp := metav1.NewTime(time.Now().Add(time.Second * time.Duration(deletionGracePeriod)))
	currentPod.DeletionTimestamp = &deletionTimestamp
	// 3. Update (overwrite) the pod
	_, err = s.client.CoreV1().Pods(testNamespace).Update(ctx, currentPod, metav1.UpdateOptions{})
	assert.NilError(t, err)

	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)
	}

	if waitForPermanentDeletion {
		select {
		case <-ctx.Done():
			t.Fatalf("Context ended early: %s", ctx.Err().Error())
		case err = <-waitDeleteCh:
			assert.NilError(t, err)
		}
	}
}

func testUpdatePodWhileRunningScenario(ctx context.Context, t *testing.T, s *system, m testingProvider) {
	t.Parallel()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p := newPod()

	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.ObjectMeta.Name).String(),
	}

	watchErrCh := make(chan error)

	// Create a Pod
	_, e := s.client.CoreV1().Pods(testNamespace).Create(ctx, p, metav1.CreateOptions{})
	assert.NilError(t, e)

	// Setup a watch to check if the pod is in running
	watcher, err := s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
	assert.NilError(t, err)
	defer watcher.Stop()
	go func() {
		newPod, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be started
			func(ev watch.Event) (bool, error) {
				pod := ev.Object.(*corev1.Pod)
				return pod.Status.Phase == corev1.PodRunning, nil
			})
		// This deepcopy is required to please the race detector
		p = newPod.Object.(*corev1.Pod).DeepCopy()
		sendErr(ctx, watchErrCh, watchErr)
	}()

	// Start the pod controller
	assert.NilError(t, s.start(ctx))

	// Wait for pod to be in running
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)
	}

	// Update the pod
	version, err := strconv.Atoi(p.ResourceVersion)
	if err != nil {
		t.Fatalf("Could not parse pod's resource version: %s", err.Error())
	}

	p.ResourceVersion = strconv.Itoa(version + 1)
	var activeDeadlineSeconds int64 = 300
	p.Spec.ActiveDeadlineSeconds = &activeDeadlineSeconds

	log.G(ctx).WithField("pod", p).Info("Updating pod")
	_, err = s.client.CoreV1().Pods(p.Namespace).Update(ctx, p, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, m.getUpdates().until(ctx, func(v int) bool { return v > 0 }))
}

func BenchmarkCreatePods(b *testing.B) {
	sl := logrus.StandardLogger()
	sl.SetLevel(logrus.ErrorLevel)
	newLogger := logruslogger.FromLogrus(logrus.NewEntry(sl))

	ctx := context.Background()
	ctx = log.WithLogger(ctx, newLogger)

	assert.NilError(b, wireUpSystem(ctx, newMockProvider(), func(ctx context.Context, s *system) {
		benchmarkCreatePods(ctx, b, s)
	}))
}

func benchmarkCreatePods(ctx context.Context, b *testing.B, s *system) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	assert.NilError(b, s.start(ctx))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pod := newPod(randomizeUID, randomizeName)
		_, err := s.client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		assert.NilError(b, err)
		assert.NilError(b, ctx.Err())
	}
}

type podModifier func(*corev1.Pod)

func randomizeUID(pod *corev1.Pod) {
	pod.UID = uuid.NewUUID()
}

func randomizeName(pod *corev1.Pod) {
	name := fmt.Sprintf("pod-%s", uuid.NewUUID())
	pod.Name = name
}

func forRealAPIServer(pod *corev1.Pod) {
	pod.ResourceVersion = ""
	pod.UID = ""
}

func nameBasedOnTest(t *testing.T) podModifier {
	return func(pod *corev1.Pod) {
		pod.Name = kubernetesNameForTest(t)
	}
}

func newPod(podmodifiers ...podModifier) *corev1.Pod {
	var terminationGracePeriodSeconds int64 = 30
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "my-pod",
			Namespace:       testNamespace,
			UID:             "4f20ff31-7775-11e9-893d-000c29a24b34",
			ResourceVersion: "100",
		},
		Spec: corev1.PodSpec{
			NodeName:                      testNodeName,
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []corev1.Container{
				{
					Name:  "my-container",
					Image: "nginx",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
	for _, modifier := range podmodifiers {
		modifier(pod)
	}
	return pod
}
