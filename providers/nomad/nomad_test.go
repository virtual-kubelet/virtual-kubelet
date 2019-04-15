package nomad

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Client provides a client to the Nomad API
type Client struct {
	config nomad.Config
}

func TestCreateGetDeletePod(t *testing.T) {
	provider, err := makeProvider(t)
	if err != nil {
		t.Fatal("unable to create mock provider", err)
	}

	nomadClient, nomadServer := makeClient(t, nil)
	defer nomadServer.Stop()

	provider.nomadClient = nomadClient
	provider.nomadAddress = nomadServer.HTTPAddr

	podName := "pod-" + uuid.New().String()
	podNamespace := "ns-" + uuid.New().String()

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				v1.Container{
					Name:  "nginx",
					Image: "nginx",
					LivenessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromString("8080"),
								Path: "/",
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
						TimeoutSeconds:      60,
						SuccessThreshold:    3,
						FailureThreshold:    5,
					},
				},
			},
		},
	}

	// Create pod
	err = provider.CreatePod(context.Background(), pod)
	if err != nil {
		t.Fatal("failed to create pod", err)
	}

	// Get pod
	pod, err = provider.GetPod(context.Background(), podNamespace, podName)
	if err != nil {
		t.Fatal("failed to get pod", err)
	}

	// Get pod tests
	// Validate pod spec
	assert.Check(t, pod != nil, "pod cannot be nil")
	assert.Check(t, pod.Spec.Containers != nil, "containers cannot be nil")
	assert.Check(t, is.Nil(pod.Annotations), "pod annotations should be nil")
	assert.Check(t, is.Equal(pod.Name, fmt.Sprintf("%s-%s", jobNamePrefix, podName)), "pod name should be equal")

	// Get pods
	pods, err := provider.GetPods(context.Background())
	if err != nil {
		t.Fatal("failed to get pods", err)
	}

	// TODO: finish adding a few more assertions
	assert.Check(t, is.Len(pods, 1), "number of pods should be 1")

	// Delete pod
	err = provider.DeletePod(context.Background(), pod)
	if err != nil {
		t.Fatal("failed to delete pod", err)
	}
}

func makeClient(t *testing.T, cb testutil.ServerConfigCallback) (*nomad.Client, *testutil.TestServer) {
	// Make client config
	conf := nomad.DefaultConfig()
	// Create server
	server := testutil.NewTestServer(t, cb)
	conf.Address = "http://" + server.HTTPAddr
	// Create client
	client, err := nomad.NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client, server
}

func makeProvider(t *testing.T) (*Provider, error) {
	// Set default region
	os.Setenv("NOMAD_REGION", "global")

	provider, err := NewProvider(nil, "fakeNomadNode", "linux")
	if err != nil {
		return nil, err
	}

	return provider, nil
}
