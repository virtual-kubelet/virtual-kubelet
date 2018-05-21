package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"
	"k8s.io/api/core/v1"

	"github.com/vmware/vic/pkg/trace"
)

var (
	testpod              *v1.Pod
)

func init() {
	testpod = &v1.Pod{}
}

func setup(t *testing.T, op trace.Operation) PodCache {
	c := NewVicPodCache()
	assert.NotNil(t, c, "NewPod did not return a valid cache")

	//populate with dummy data
	vp := pod.VicPod{
		ID: "123",
		Pod: testpod,
	}
	c.Add(op, "namespace1", "testpod1a", &vp)
	c.Add(op, "namespace1", "testpod1b", &vp)
	c.Add(op, "namespace2", "testpod2a", &vp)
	c.Add(op, "namespace2", "testpod2b", &vp)

	return c
}

func TestRehydrate(t *testing.T) {
	op := trace.NewOperation(context.Background(), "")

	c := NewVicPodCache()
	assert.NotNil(t, c, "NewPod did not return a valid cache")

	err := c.Rehydrate(op)
	assert.Nil(t, err, "PodCache.Rehydrate failed with error: %s", err)
}

func TestAdd(t *testing.T) {
	var err error
	op := trace.NewOperation(context.Background(), "")

	c := NewVicPodCache()
	assert.NotNil(t, c, "NewPod did not return a valid cache")

	//populate with dummy data
	vp := pod.VicPod{
		ID: "123",
		Pod: testpod,
	}

	// Positive cases
	err = c.Add(op, "namespace1", "testpod1a", &vp)
	assert.Nil(t, err, "PodCache.Add failed with error: %s", err)

	// Negative cases
	err = c.Add(op, "namespace1", "", &vp)
	assert.NotNil(t, err, "PodCache.Add expected error for empty name")
	assert.Equal(t, err, PodCachePodNameError)

	err = c.Add(op, "namespace1", "test2", nil)
	assert.NotNil(t, err, "PodCache.Add expected error for nil pod")
	assert.Equal(t, err, PodCacheNilPodError)
}

func TestGet(t *testing.T) {
	var err error
	var vpod *pod.VicPod
	op := trace.NewOperation(context.Background(), "")

	c := setup(t, op)

	// Positive cases
	vpod, err = c.Get(op, "namespace1", "testpod1a")
	assert.Nil(t, err, "PodCache.Get failed with error: %s", err)
	assert.NotNil(t, vpod, "PodCache.Get expected to return non-nil pod but received nil")

	vpod, err = c.Get(op, "namespace2", "testpod2a")
	assert.Nil(t, err, "PodCache.Get failed with error: %s", err)
	assert.NotNil(t, vpod, "PodCache.Get expected to return non-nil pod but received nil")

	// Negative cases
	vpod, err = c.Get(op, "namespace1", "")
	assert.Equal(t, err, PodCachePodNameError)
	assert.Nil(t, vpod, "PodCache.Get expected to return nil pod but received non-nil")

	//TODO: uncomment out once namespace support added to cache
	//vpod, err = c.Get(op, "namespace1", "testpod2a")
	//assert.NotNil(t, err, "PodCache.Get did not respect namespace: %s", err)

	//vpod, err = c.Get(op, "", "testpod1a")
	//assert.NotNil(t, err, "PodCache.Get did not respect namespace: %s", err)
}

func TestGetAll(t *testing.T) {
	op := trace.NewOperation(context.Background(), "")

	c := setup(t, op)

	vps := c.GetAll(op)
	assert.NotNil(t, vps, "PodCache.GetAll returned nil slice")
	assert.Len(t, vps, 4, "PodCache.Get did not return all pod definitions.  Returned %d pods.", len(vps))
}

func TestDelete(t *testing.T) {
	var err error
	op := trace.NewOperation(context.Background(), "")

	c := setup(t, op)

	// Positive cases
	err = c.Delete(op, "namespace1", "testpod1a")
	assert.Nil(t, err, "PodCache.Delete failed with error: %s", err)
	vps := c.GetAll(op)
	assert.Len(t, vps, 3, "PodCache.Delete did not delete pod.")

	// Negative cases
	err = c.Delete(op, "namespace2", "")
	assert.Equal(t, err, PodCachePodNameError)

	//TODO: uncomment the tests below once namespace support added to cache
	//vps = c.GetAll(op)
	//currCount := len(vps)
	//err = c.Delete(op, "", "testpod1b")
	//assert.NotNil(t, err, "PodCache.Delete expected to return error but received nil")
	//vps = c.GetAll(op)
	//assert.Len(t, vps, currCount, "PodCache.Delete ignored namespace")
}

