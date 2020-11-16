package e2e_test

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bombsimon/logrusr"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	klogv1 "k8s.io/klog"
	klogv2 "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var enableEnvTest = flag.Bool("envtest", false, "Enable envtest based tests")

func TestMain(m *testing.M) {
	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klogv1.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		flag.Var(f.Value, "klog."+f.Name, f.Usage)
	})
	flag.Parse()
	os.Exit(m.Run())
}

func TestEnvtest(t *testing.T) {
	if !*enableEnvTest || os.Getenv("VK_ENVTEST") != "" {
		t.Skip("test only runs when -envtest is passed or if VK_ENVTEST is set to a non-empty value")
	}
	env := &envtest.Environment{}
	_, err := env.Start()
	assert.NilError(t, err)
	defer func() {
		assert.NilError(t, env.Stop())
	}()

	t.Log("Env test environment ready")
	t.Run("E2ERunWithoutLeases", func(t *testing.T) {
		testNodeE2ERun(t, env, false)
	})
	t.Run("E2ERunWithLeases", func(t *testing.T) {
		testNodeE2ERun(t, env, true)
	})
}

func nodeNameForTest(t *testing.T) string {
	name := t.Name()
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func testNodeE2ERun(t *testing.T, env *envtest.Environment, withLeases bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sl := logrus.StandardLogger()
	sl.SetLevel(logrus.DebugLevel)
	klogv2.SetLogger(logrusr.NewLogger(sl))
	newLogger := logruslogger.FromLogrus(logrus.NewEntry(sl))
	ctx = log.WithLogger(ctx, newLogger)

	clientset, err := kubernetes.NewForConfig(env.Config)
	assert.NilError(t, err)
	nodes := clientset.CoreV1().Nodes()
	_, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	testProvider := node.NewNaiveNodeProvider()

	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeNameForTest(t),
		},
	}

	testNodeCopy := testNode.DeepCopy()

	opts := []node.NodeControllerOpt{}
	leasesClient := clientset.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease)
	if withLeases {
		opts = append(opts, node.WithNodeEnableLeaseV1Beta1(leasesClient, nil))
	}
	node, err := node.NewNodeController(testProvider, testNode, nodes, opts...)
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
