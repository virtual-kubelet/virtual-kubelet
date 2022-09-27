package node

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bombsimon/logrusr/v3"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestEnvtest(t *testing.T) {
	if !*enableEnvTest || os.Getenv("VK_ENVTEST") != "" {
		t.Skip("test only runs when -envtest is passed or if VK_ENVTEST is set to a non-empty value")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := &envtest.Environment{}
	_, err := env.Start()
	assert.NilError(t, err)
	defer func() {
		assert.NilError(t, env.Stop())
	}()

	t.Log("Env test environment ready")
	t.Run("E2ERunWithoutLeases", wrapE2ETest(ctx, env, func(ctx context.Context, t *testing.T, environment *envtest.Environment) {
		testNodeE2ERun(t, env, false)
	}))
	t.Run("E2ERunWithLeases", wrapE2ETest(ctx, env, func(ctx context.Context, t *testing.T, environment *envtest.Environment) {
		testNodeE2ERun(t, env, true)
	}))

	t.Run("E2EPodStatusUpdate", wrapE2ETest(ctx, env, testPodStatusUpdate))
}

func kubernetesNameForTest(t *testing.T) string {
	name := t.Name()
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func wrapE2ETest(ctx context.Context, env *envtest.Environment, f func(context.Context, *testing.T, *envtest.Environment)) func(*testing.T) {
	return func(t *testing.T) {
		log.G(ctx)
		sl := logrus.StandardLogger()
		sl.SetLevel(logrus.DebugLevel)
		logger := logruslogger.FromLogrus(sl.WithField("test", t.Name()))
		ctx = log.WithLogger(ctx, logger)

		// The following requires that E2E tests are performed *sequentially*
		log.L = logger
		klog.SetLogger(logrusr.New(sl))
		f(ctx, t, env)
	}
}

func testPodStatusUpdate(ctx context.Context, t *testing.T, env *envtest.Environment) {
	provider := newMockProvider()

	clientset, err := kubernetes.NewForConfig(env.Config)
	assert.NilError(t, err)
	pods := clientset.CoreV1().Pods(testNamespace)

	assert.NilError(t, wireUpSystemWithClient(ctx, provider, clientset, func(ctx context.Context, s *system) {
		p := newPod(forRealAPIServer, nameBasedOnTest(t))
		// In real API server, we don't set the resource version
		p.ResourceVersion = ""
		newPod, err := pods.Create(ctx, p, metav1.CreateOptions{})
		assert.NilError(t, err)

		key, err := buildKey(newPod)
		assert.NilError(t, err)

		listOptions := metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", p.ObjectMeta.Name).String(),
		}

		// Setup a watch to check if the pod is in running
		watcher, err := s.client.CoreV1().Pods(testNamespace).Watch(ctx, listOptions)
		assert.NilError(t, err)
		defer watcher.Stop()
		// Start the pod controller
		assert.NilError(t, s.start(ctx))
		var serverPod *corev1.Pod
		for {
			select {
			case <-ctx.Done():
				t.Fatalf("Context ended early: %s", ctx.Err().Error())
			case ev := <-watcher.ResultChan():
				serverPod = ev.Object.(*corev1.Pod)
				if serverPod.Status.Phase == corev1.PodRunning {
					goto running
				}
			}
		}
	running:
		t.Log("Observed pod in running state")

		providerPod, ok := provider.pods.Load(key)
		assert.Assert(t, ok)
		providerPodCopy := providerPod.(*corev1.Pod).DeepCopy()
		providerPodCopy.Status = serverPod.Status
		if providerPodCopy.Annotations == nil {
			providerPodCopy.Annotations = make(map[string]string, 1)
		}
		providerPodCopy.Annotations["testannotation"] = "testvalue"
		provider.notifier(providerPodCopy)

		for {
			select {
			case <-ctx.Done():
				t.Fatalf("Context ended early: %s", ctx.Err().Error())
			case ev := <-watcher.ResultChan():
				annotations := ev.Object.(*corev1.Pod).Annotations
				if annotations != nil && annotations["testannotation"] == "testvalue" {
					return
				}
			}
		}
	}))
}

func testNodeE2ERun(t *testing.T, env *envtest.Environment, withLeases bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset, err := kubernetes.NewForConfig(env.Config)
	assert.NilError(t, err)
	nodes := clientset.CoreV1().Nodes()
	_, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	testProvider := NewNaiveNodeProvider()

	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubernetesNameForTest(t),
		},
	}

	testNodeCopy := testNode.DeepCopy()

	opts := []NodeControllerOpt{}
	leasesClient := clientset.CoordinationV1().Leases(corev1.NamespaceNodeLease)
	if withLeases {
		opts = append(opts, WithNodeEnableLeaseV1(leasesClient, 0))
	}
	node, err := NewNodeController(testProvider, testNode, nodes, opts...)
	assert.NilError(t, err)

	chErr := make(chan error, 1)
	go func() {
		chErr <- node.Run(ctx)
	}()

	log.G(ctx).Debug("Waiting for node ready")
	select {
	case <-node.Ready():
	case err = <-chErr:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	now := time.Now()
	var n *corev1.Node
	for time.Since(now) < time.Minute*5 {
		n, err = nodes.Get(ctx, testNodeCopy.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err == nil {
			t.Log(n)
			goto node_found
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Node never found")

node_found:
	if withLeases {
		for time.Since(now) < time.Minute*5 {
			l, err := leasesClient.Get(ctx, testNodeCopy.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				continue
			}
			if err == nil {
				t.Log(l)
				assert.Assert(t, is.Len(l.OwnerReferences, 1))
				assert.Assert(t, is.Equal(l.OwnerReferences[0].Name, n.Name))
				assert.Assert(t, is.Equal(l.OwnerReferences[0].UID, n.UID))
				goto lease_found
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

lease_found:
	cancel()
	err = <-chErr
	assert.NilError(t, err)
}
