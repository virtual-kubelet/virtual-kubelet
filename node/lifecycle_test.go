package node

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/examples/providers/mock"
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

	newLogger := logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	logrus.SetLevel(logrus.DebugLevel)
	ctx = log.WithLogger(ctx, newLogger)

	mockProvider, err := mock.NewMockProviderMockConfig(mock.MockConfig{}, testNodeName, "linux", "1.2.3.4", 0)
	assert.NilError(t, err)
	mockV0Provider, err := mock.NewMockV0ProviderMockConfig(mock.MockConfig{}, testNodeName, "linux", "1.2.3.4", 0)

	t.Run("mockProvider", func(t2 *testing.T) {
		testPodLifecycle(t2, ctx, mockProvider)
	})
	t.Run("mockV0Provider", func(t2 *testing.T) {
		testPodLifecycle(t2, ctx, mockV0Provider)
	})
}

func testPodLifecycle(t *testing.T, ctx context.Context, provider PodLifecycleHandler) {
	t.Parallel()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Right now, new loggers that are created from spans are broken since log.L isn't set.

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

	config := PodControllerConfig{
		PodClient:         client.CoreV1(),
		PodInformer:       podInformer,
		EventRecorder:     fakeRecorder,
		Provider:          provider,
		ConfigMapInformer: configMapInformer,
		SecretInformer:    secretInformer,
		ServiceInformer:   serviceInformer,
	}

	pc, err := NewPodController(config)
	assert.NilError(t, err)

	p := corev1.Pod{
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

	listOptions := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.ObjectMeta.Name).String(),
	}

	watchErrCh := make(chan error)

	// Setup a watch (prior to pod creation, and pod controller startup)
	watcher, err := client.CoreV1().Pods(testNamespace).Watch(listOptions)
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

	go podInformerFactory.Start(ctx.Done())
	go scmInformerFactory.Start(ctx.Done())

	// Create the Pod
	_, e := client.CoreV1().Pods(testNamespace).Create(&p)
	assert.NilError(t, e)

	// This will return once
	select {
	case <-ctx.Done():
		t.Fatalf("Context ended early: %s", ctx.Err().Error())
	case err = <-watchErrCh:
		assert.NilError(t, err)

	}

	// Setup a watch to check if the pod is in running
	watcher, err = client.CoreV1().Pods(testNamespace).Watch(listOptions)
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
	podControllerErrCh := make(chan error, 1)
	go func() {
		podControllerErrCh <- pc.Run(ctx, podSyncWorkers)
	}()

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
	watcher, err = client.CoreV1().Pods(testNamespace).Watch(listOptions)
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
	currentPod, err := client.CoreV1().Pods(testNamespace).Get(p.Name, metav1.GetOptions{})
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
	_, err = client.CoreV1().Pods(testNamespace).Update(currentPod)
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

	cancel()
	assert.NilError(t, <-podControllerErrCh)
}
