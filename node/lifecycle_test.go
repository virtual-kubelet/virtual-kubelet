package node

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	watchutils "k8s.io/client-go/tools/watch"
	"k8s.io/klog"
)

var (
	_ record.EventRecorder = (*fakeDiscardingRecorder)(nil)
)

const (
	// There might be a constant we can already leverage here
	testNamespace        = "default"
	informerResyncPeriod = time.Duration(1 * time.Second)
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
	r.Eventf(object, eventType, reason, message)
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

	t.Run("createStartDeleteScenario", func(t *testing.T) {
		t.Run("mockProvider", func(t *testing.T) {

			assert.NilError(t, wireUpSystem(ctx, newMockProvider(), func(ctx context.Context, s *system) {
				testCreateStartDeleteScenario(ctx, t, s)
			}))
		})

		if testing.Short() {
			return
		}
		t.Run("mockV0Provider", func(t *testing.T) {
			assert.NilError(t, wireUpSystem(ctx, newMockV0Provider(), func(ctx context.Context, s *system) {
				testCreateStartDeleteScenario(ctx, t, s)
			}))
		})
	})

	t.Run("danglingPodScenario", func(t *testing.T) {
		t.Parallel()
		t.Run("mockProvider", func(t *testing.T) {
			mp := newMockProvider()
			assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
				testDanglingPodScenario(ctx, t, s, mp.mockV0Provider)
			}))
		})

		if testing.Short() {
			return
		}

		t.Run("mockV0Provider", func(t *testing.T) {
			mp := newMockV0Provider()
			assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
				testDanglingPodScenario(ctx, t, s, mp)
			}))
		})
	})

	t.Run("failedPodScenario", func(t *testing.T) {
		t.Parallel()
		t.Run("mockProvider", func(t *testing.T) {
			mp := newMockProvider()
			assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
				testFailedPodScenario(ctx, t, s)
			}))
		})

		if testing.Short() {
			return
		}

		t.Run("mockV0Provider", func(t *testing.T) {
			assert.NilError(t, wireUpSystem(ctx, newMockV0Provider(), func(ctx context.Context, s *system) {
				testFailedPodScenario(ctx, t, s)
			}))
		})
	})

	t.Run("succeededPodScenario", func(t *testing.T) {
		t.Parallel()
		t.Run("mockProvider", func(t *testing.T) {
			mp := newMockProvider()
			assert.NilError(t, wireUpSystem(ctx, mp, func(ctx context.Context, s *system) {
				testSucceededPodScenario(ctx, t, s)
			}))
		})
		if testing.Short() {
			return
		}
		t.Run("mockV0Provider", func(t *testing.T) {
			assert.NilError(t, wireUpSystem(ctx, newMockV0Provider(), func(ctx context.Context, s *system) {
				testSucceededPodScenario(ctx, t, s)
			}))
		})
	})
}

type testFunction func(ctx context.Context, s *system)
type system struct {
	retChan             chan error
	pc                  *PodController
	client              *fake.Clientset
	podControllerConfig PodControllerConfig
}

func (s *system) start(ctx context.Context) chan error {
	podControllerErrChan := make(chan error)
	go func() {
		podControllerErrChan <- s.pc.Run(ctx, podSyncWorkers)
	}()

	// We need to wait for the pod controller to start. If there is an error before the pod controller starts, or
	// the context is cancelled. If the context is cancelled, the startup will be aborted, and the pod controller
	// will return an error, so we don't need to wait on ctx.Done()
	select {
	case <-s.pc.Ready():
		// This listens for errors, or exits in the future.
		go func() {
			podControllerErr := <-podControllerErrChan
			s.retChan <- podControllerErr
		}()
	// If there is an error before things are ready, we need to forward it immediately
	case podControllerErr := <-podControllerErrChan:
		s.retChan <- podControllerErr
	}
	return s.retChan
}

func wireUpSystem(ctx context.Context, provider PodLifecycleHandler, f testFunction) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create the fake client.
	client := fake.NewSimpleClientset()

	// This is largely copy and pasted code from the root command
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		client,
		informerResyncPeriod,
		kubeinformers.WithNamespace(testNamespace),
	)
	podInformer := podInformerFactory.Core().V1().Pods()

	scmInformerFactory := kubeinformers.NewSharedInformerFactory(client, informerResyncPeriod)

	eb := record.NewBroadcaster()
	eb.StartLogging(log.G(ctx).Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: client.CoreV1().Events(testNamespace)})
	fakeRecorder := &fakeDiscardingRecorder{
		logger: log.G(ctx),
	}

	secretInformer := scmInformerFactory.Core().V1().Secrets()
	configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := scmInformerFactory.Core().V1().Services()
	sys := &system{
		client:  client,
		retChan: make(chan error, 1),
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

	go scmInformerFactory.Start(ctx.Done())
	go podInformerFactory.Start(ctx.Done())

	f(ctx, sys)

	// Shutdown the pod controller, and wait for it to exit
	cancel()
	return <-sys.retChan
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
	_, e := s.client.CoreV1().Pods(testNamespace).Create(p1)
	assert.NilError(t, e)

	// Start the pod controller
	s.start(ctx)

	for s.pc.k8sQ.Len() > 0 {
	}

	p2, err := s.client.CoreV1().Pods(testNamespace).Get(p1.Name, metav1.GetOptions{})
	assert.NilError(t, err)

	// Make sure the pods have not changed
	assert.DeepEqual(t, p1, p2)
}

func testDanglingPodScenario(ctx context.Context, t *testing.T, s *system, m *mockV0Provider) {
	t.Parallel()

	pod := newPod()
	assert.NilError(t, m.CreatePod(ctx, pod))

	// Start the pod controller
	s.start(ctx)

	assert.Assert(t, m.deletes == 1)

}

func testCreateStartDeleteScenario(ctx context.Context, t *testing.T, s *system) {
	t.Parallel()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p := newPod()

	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.ObjectMeta.Name).String(),
	}

	watchErrCh := make(chan error)

	// Setup a watch (prior to pod creation, and pod controller startup)
	watcher, err := s.client.CoreV1().Pods(testNamespace).Watch(listOptions)
	assert.NilError(t, err)

	// This ensures that the pod is created.
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be created
			// TODO(Sargun): Make this "smarter" about the status the pod is in.
			func(ev watch.Event) (bool, error) {
				pod := ev.Object.(*corev1.Pod)
				return pod.Name == p.ObjectMeta.Name, nil
			})

		watchErrCh <- watchErr
	}()

	// Create the Pod
	_, e := s.client.CoreV1().Pods(testNamespace).Create(p)
	assert.NilError(t, e)

	// This will return once
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)

	}

	// Setup a watch to check if the pod is in running
	watcher, err = s.client.CoreV1().Pods(testNamespace).Watch(listOptions)
	assert.NilError(t, err)
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be started
			func(ev watch.Event) (bool, error) {
				pod := ev.Object.(*corev1.Pod)
				return pod.Status.Phase == corev1.PodRunning, nil
			})

		watchErrCh <- watchErr
	}()

	// Start the pod controller
	podControllerErrCh := s.start(ctx)

	// Wait for pod to be in running
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-podControllerErrCh:
		assert.NilError(t, err)
		t.Fatal("Pod controller exited prematurely without error")
	case err = <-watchErrCh:
		assert.NilError(t, err)

	}

	// Setup a watch prior to pod deletion
	watcher, err = s.client.CoreV1().Pods(testNamespace).Watch(listOptions)
	assert.NilError(t, err)
	go func() {
		_, watchErr := watchutils.UntilWithoutRetry(ctx, watcher,
			// Wait for the pod to be started
			func(ev watch.Event) (bool, error) {
				log.G(ctx).WithField("event", ev).Info("got event")
				// TODO(Sargun): The pod should have transitioned into some status around failed / succeeded
				// prior to being deleted.
				// In addition, we should check if the deletion timestamp gets set
				return ev.Type == watch.Deleted, nil
			})
		watchErrCh <- watchErr
	}()

	// Delete the pod

	// 1. Get the pod
	currentPod, err := s.client.CoreV1().Pods(testNamespace).Get(p.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	// 2. Set the pod's deletion timestamp, version, and so on
	curVersion, err := strconv.Atoi(currentPod.ResourceVersion)
	assert.NilError(t, err)
	currentPod.ResourceVersion = strconv.Itoa(curVersion + 1)
	var deletionGracePeriod int64 = 30
	currentPod.DeletionGracePeriodSeconds = &deletionGracePeriod
	deletionTimestamp := metav1.NewTime(time.Now().Add(time.Second * time.Duration(deletionGracePeriod)))
	currentPod.DeletionTimestamp = &deletionTimestamp
	// 3. Update (overwrite) the pod
	_, err = s.client.CoreV1().Pods(testNamespace).Update(currentPod)
	assert.NilError(t, err)

	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-podControllerErrCh:
		assert.NilError(t, err)
		t.Fatal("Pod controller exited prematurely without error")
	case err = <-watchErrCh:
		assert.NilError(t, err)

	}
}

func newPod() *corev1.Pod {
	return &corev1.Pod{
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
			NodeName: testNodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
}
