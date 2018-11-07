package vkubelet

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/mock"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	fakeClient kubernetes.Interface
)

func init() {
	fakeClient = fake.NewSimpleClientset()
}

// Tests calculate pod divergence
func TestCalculatePodDivergence(t *testing.T) {
	pod1 := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pod1"}}
	pod2 := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "pod2"}}
	pod3 := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns3", Name: "pod3"}}

	deletionTime := metav1.NewTime(time.Now())
	deletingPod1 := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns5", Name: "deletingPod1", DeletionTimestamp: &deletionTime}}
	deletingPod2 := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns6", Name: "deletingPod2", DeletionTimestamp: &deletionTime}}

	tt := []struct {
		name                 string
		podsInProvider       []*v1.Pod
		podsInResoureManager []*v1.Pod
		podsToDelete         []*v1.Pod
		podsToCreate         []*v1.Pod
	}{
		{
			"BothEmpty",
			nil,
			nil,
			nil,
			nil,
		},
		{
			"PodInProvider",
			[]*v1.Pod{pod1, pod2},
			nil,
			[]*v1.Pod{pod1, pod2},
			nil,
		},
		{
			"PodInResourceManager",
			nil,
			[]*v1.Pod{pod1, pod2},
			nil,
			[]*v1.Pod{pod1, pod2},
		},
		{
			"DeletingPodInResourceManager",
			nil,
			[]*v1.Pod{deletingPod1},
			[]*v1.Pod{deletingPod1},
			nil,
		},
		{
			"DeletingPodInBoth",
			[]*v1.Pod{deletingPod1},
			[]*v1.Pod{deletingPod1},
			[]*v1.Pod{deletingPod1},
			nil,
		},
		{
			"Mixed",
			[]*v1.Pod{pod1, pod2, deletingPod1},
			[]*v1.Pod{pod1, pod3, deletingPod1, deletingPod2},
			[]*v1.Pod{pod2, deletingPod1, deletingPod2},
			[]*v1.Pod{pod3},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			p := createMockProvider(tc.podsInProvider)
			rm := createTestResourceManager(tc.podsInResoureManager)

			s := createTestServer(p, rm)

			podsToDelete, podsToCreate, err := s.calculatePodDivergence(context.TODO())
			assert.Nil(t, err, "Calculate pod divergence should not fail.")
			assert.Equal(t, len(tc.podsToDelete), len(podsToDelete), "The number of pods to delete is not match.")
			assert.Equal(t, len(tc.podsToCreate), len(podsToCreate), "The number of pods to delete is not match.")
			for _, pod := range tc.podsToDelete {
				found := false
				for _, p := range podsToDelete {
					if p.GetNamespace() == pod.GetNamespace() && p.GetName() == pod.GetName() {
						found = true
						break;
					}
				}

				assert.True(t, found, "Missing pod to delete %s", pod.GetName())
			}

			for _, pod := range tc.podsToCreate {
				found := false
				for _, p := range podsToCreate {
					if p.GetNamespace() == pod.GetNamespace() && p.GetName() == pod.GetName() {
						found = true
						break;
					}
				}

				assert.True(t, found, "Missing pod to create %s", pod.GetName())
			}
		})
	}
}

func createTestServer(p providers.Provider, rm *manager.ResourceManager) *Server {
	return &Server{
		namespace:       "testns",
		nodeName:        "testnode",
		k8sClient:       fakeClient,
		podSyncWorkers:  10,
		podCh:           make(chan *podNotification, 10),
		provider:        p,
		resourceManager: rm,
	}
}

func createMockProvider(pods []*v1.Pod) *mock.MockProvider{
	podsMap := make(map[string]*v1.Pod, len(pods))
	for _, pod := range pods {
		podsMap[pod.GetNamespace() + "-" + pod.GetName()] = pod
	}

	return &mock.MockProvider{
		NodeName:           "testnode",
		OSType:             "testos",
		InternalIP:         "0.0.0.0",
		DaemonEndpointPort: 10255,
		Pods:               podsMap,
		Config:             mock.MockConfig{
			CPU:    "4",
			Memory: "14G",
			Pods:   "30",
		},
	}
}

func createTestResourceManager(pods []*v1.Pod) *manager.ResourceManager {
	podItems := make([]v1.Pod, 0, len(pods))
	for _, pod := range pods {
		podItems = append(podItems, *pod)
	}
	rm, err := manager.NewResourceManager(fakeClient)
	if err != nil {
		panic(err)
	}
	
	rm.SetPods(&v1.PodList{Items: podItems})

	return rm
}