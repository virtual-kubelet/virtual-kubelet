package cache

import (
	"fmt"
	"sync"

	"github.com/vmware/vic/pkg/trace"

	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"
)

type PodCache interface {
	Rehydrate(op trace.Operation) error
	Get(op trace.Operation, namespace, name string) (*vicpod.VicPod, error)
	GetAll(op trace.Operation) []*vicpod.VicPod
	Add(op trace.Operation, namespace, name string, pod *vicpod.VicPod) error
	Delete(op trace.Operation, namespace, name string) error
}

type VicPodCache struct {
	cache map[string]*vicpod.VicPod
	lock  sync.Mutex
}

type CacheError string

func (c CacheError) Error() string {return string(c)}

const (
	PodCachePodNameError = CacheError("PodCache called with empty pod name")
	PodCacheNilPodError = CacheError("PodCache called with nil pod")
)

func NewVicPodCache() PodCache {
	v := &VicPodCache{}

	v.cache = make(map[string]*vicpod.VicPod, 0)

	return v
}

// Rehydrate replenishes the cache in the event of a virtual kubelet restart.
//	NOT YET IMPLEMENTED
//
// arguments:
//		op		operation trace logger
// returns:
// 		error
func (v *VicPodCache) Rehydrate(op trace.Operation) error {
	return nil
}

// Get returns the pod definition for a running pod
//
// arguments:
//		op			operation trace logger
//		namespace	namespace of the pod.  Empty namespace assumes default.
//		name		name of the pod
// returns:
// 		error
func (v *VicPodCache) Get(op trace.Operation, namespace, name string) (*vicpod.VicPod, error) {
	defer trace.End(trace.Begin(name, op))

	if name == "" {
		op.Errorf(PodCachePodNameError.Error())
		return nil, PodCachePodNameError
	}

	//TODO: handle namespaces

	pod, ok := v.cache[name]
	if !ok {
		err := fmt.Errorf("Pod %s not found in cache", name)

		op.Info(err)
		return nil, err
	}

	return pod, nil
}

// GetAll returns the pod definitions for all running pods
//
// arguments:
//		op			operation trace logger
// returns:
// 		error
func (v *VicPodCache) GetAll(op trace.Operation) []*vicpod.VicPod {
	defer trace.End(trace.Begin("", op))
	defer v.lock.Unlock()
	v.lock.Lock()

	list := make([]*vicpod.VicPod, 0)

	for _, vp := range v.cache {
		list = append(list, vp)
	}

	return list
}

// Add saves the pod definition of a running pod
//
// arguments:
//		op			operation trace logger
//		namespace	namespace of the pod.  Empty namespace assumes default.
//		name		name of the pod
//		pod			pod definition
// returns:
// 		error
func (v *VicPodCache) Add(op trace.Operation, namespace, name string, pod *vicpod.VicPod) error {
	defer trace.End(trace.Begin(name, op))
	defer v.lock.Unlock()
	v.lock.Lock()

	if name == "" {
		op.Errorf(PodCachePodNameError.Error())
		return PodCachePodNameError
	}
	if pod == nil {
		op.Errorf(PodCacheNilPodError.Error())
		return PodCacheNilPodError
	}

	//TODO: handle namespaces

	_, ok := v.cache[name]
	if ok {
		err := fmt.Errorf("Pod %s already cached.  Duplicate pod.", name)

		op.Error(err)
		return err
	}

	v.cache[name] = pod
	return nil
}

// Delete removes a pod definition from the cache.  It does not stop/delete the
//	actual pod.
//
// arguments:
//		op			operation trace logger
//		namespace	namespace of the pod.  Empty namespace assumes default.
//		name		name of the pod
// returns:
// 		error
func (v *VicPodCache) Delete(op trace.Operation, namespace, name string) error {
	defer trace.End(trace.Begin(name, op))
	defer v.lock.Unlock()
	v.lock.Lock()

	if name == "" {
		op.Errorf(PodCachePodNameError.Error())
		return PodCachePodNameError
	}

	//TODO: handle namespaces
	delete(v.cache, name)

	return nil
}
