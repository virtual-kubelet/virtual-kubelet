package manager

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// ResourceManager works a cache for pods assigned to this virtual node within Kubernetes.
// New ResourceManagers should be created with the NewResourceManager() function.
type ResourceManager struct {
	sync.RWMutex
	k8sClient kubernetes.Interface

	pods         map[string]*v1.Pod
	deletingPods map[string]*v1.Pod
	configMapRef map[string]int64
	configMaps   map[string]*v1.ConfigMap
	secretRef    map[string]int64
	secrets      map[string]*v1.Secret
}

// NewResourceManager returns a ResourceManager with the internal maps initialized.
func NewResourceManager(k8sClient kubernetes.Interface) (*ResourceManager, error) {
	rm := ResourceManager{
		pods:         make(map[string]*v1.Pod, 0),
		deletingPods: make(map[string]*v1.Pod, 0),
		configMapRef: make(map[string]int64, 0),
		secretRef:    make(map[string]int64, 0),
		configMaps:   make(map[string]*v1.ConfigMap, 0),
		secrets:      make(map[string]*v1.Secret, 0),
		k8sClient:    k8sClient,
	}

	configW, err := rm.k8sClient.CoreV1().ConfigMaps(v1.NamespaceAll).Watch(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting config watch")
	}

	secretsW, err := rm.k8sClient.CoreV1().Secrets(v1.NamespaceAll).Watch(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting secrets watch")
	}

	go rm.watchConfigMaps(configW)
	go rm.watchSecrets(secretsW)

	tick := time.Tick(5 * time.Minute)
	go func() {
		for range tick {
			rm.Lock()
			for n, c := range rm.secretRef {
				if c <= 0 {
					delete(rm.secretRef, n)
				}
			}
			for n := range rm.secrets {
				if _, ok := rm.secretRef[n]; !ok {
					delete(rm.secrets, n)
				}
			}
			for n, c := range rm.configMapRef {
				if c <= 0 {
					delete(rm.configMapRef, n)
				}
			}
			for n := range rm.configMaps {
				if _, ok := rm.configMapRef[n]; !ok {
					delete(rm.configMaps, n)
				}
			}
			rm.Unlock()
		}
	}()

	return &rm, nil
}

// SetPods clears the internal cache and populates it with the supplied pods.
func (rm *ResourceManager) SetPods(pods *v1.PodList) {
	rm.Lock()
	defer rm.Unlock()

	for k, p := range pods.Items {
		podKey := rm.getStoreKey(p.Namespace, p.Name)
		if p.DeletionTimestamp != nil {
			rm.deletingPods[podKey] = &pods.Items[k]
		} else {
			rm.pods[podKey] = &pods.Items[k]
			rm.incrementRefCounters(&p)
		}
	}
}

// UpdatePod updates the supplied pod in the cache.
func (rm *ResourceManager) UpdatePod(p *v1.Pod) bool {
	rm.Lock()
	defer rm.Unlock()

	podKey := rm.getStoreKey(p.Namespace, p.Name)
	if p.DeletionTimestamp != nil {
		if old, ok := rm.pods[podKey]; ok {
			rm.deletingPods[podKey] = p

			rm.decrementRefCounters(old)
			delete(rm.pods, podKey)

			return true
		}

		if _, ok := rm.deletingPods[podKey]; ok {
			return false
		}

		return false
	}

	if old, ok := rm.pods[podKey]; ok {
		rm.decrementRefCounters(old)
		rm.pods[podKey] = p
		rm.incrementRefCounters(p)

		// NOTE(junjiez): no reconcile as we don't support update pod.
		return false
	}

	rm.pods[podKey] = p
	rm.incrementRefCounters(p)

	return true
}

// DeletePod removes the pod from the cache.
func (rm *ResourceManager) DeletePod(p *v1.Pod) bool {
	rm.Lock()
	defer rm.Unlock()

	podKey := rm.getStoreKey(p.Namespace, p.Name)
	if old, ok := rm.pods[podKey]; ok {
		rm.decrementRefCounters(old)
		delete(rm.pods, podKey)
		return true
	}

	if _, ok := rm.deletingPods[podKey]; ok {
		delete(rm.deletingPods, podKey)
	}

	return false
}

// GetPod retrieves the specified pod from the cache. It returns nil if a pod is not found.
func (rm *ResourceManager) GetPod(namespace, name string) *v1.Pod {
	rm.RLock()
	defer rm.RUnlock()

	if p, ok := rm.pods[rm.getStoreKey(namespace, name)]; ok {
		return p
	}

	return nil
}

// GetPods returns a list of all known pods assigned to this virtual node.
func (rm *ResourceManager) GetPods() []*v1.Pod {
	rm.RLock()
	defer rm.RUnlock()

	pods := make([]*v1.Pod, 0, len(rm.pods)+len(rm.deletingPods))
	for _, p := range rm.pods {
		pods = append(pods, p)
	}
	for _, p := range rm.deletingPods {
		pods = append(pods, p)
	}

	return pods
}

// GetConfigMap returns the specified ConfigMap from Kubernetes. It retrieves it from cache if there
func (rm *ResourceManager) GetConfigMap(name, namespace string) (*v1.ConfigMap, error) {
	rm.Lock()
	defer rm.Unlock()

	configMapKey := rm.getStoreKey(namespace, name)
	if cm, ok := rm.configMaps[configMapKey]; ok {
		return cm, nil
	}

	var opts metav1.GetOptions
	cm, err := rm.k8sClient.CoreV1().ConfigMaps(namespace).Get(name, opts)
	if err != nil {
		return nil, err
	}
	rm.configMaps[configMapKey] = cm

	return cm, err
}

// GetSecret returns the specified ConfigMap from Kubernetes. It retrieves it from cache if there
func (rm *ResourceManager) GetSecret(name, namespace string) (*v1.Secret, error) {
	rm.Lock()
	defer rm.Unlock()

	secretkey := rm.getStoreKey(namespace, name)
	if secret, ok := rm.secrets[secretkey]; ok {
		return secret, nil
	}

	var opts metav1.GetOptions
	secret, err := rm.k8sClient.CoreV1().Secrets(namespace).Get(name, opts)
	if err != nil {
		return nil, err
	}
	rm.secrets[secretkey] = secret

	return secret, err
}

// watchConfigMaps monitors the kubernetes API for modifications and deletions of configmaps
// it evicts them from the internal cache
func (rm *ResourceManager) watchConfigMaps(w watch.Interface) {

	for {
		select {
		case ev, ok := <-w.ResultChan():
			if !ok {
				return
			}

			rm.Lock()
			configMapkey := rm.getStoreKey(ev.Object.(*v1.ConfigMap).Namespace, ev.Object.(*v1.ConfigMap).Name)
			switch ev.Type {
			case watch.Modified:
				delete(rm.configMaps, configMapkey)
			case watch.Deleted:
				delete(rm.configMaps, configMapkey)
			}
			rm.Unlock()
		}
	}
}

// watchSecretes monitors the kubernetes API for modifications and deletions of secrets
// it evicts them from the internal cache
func (rm *ResourceManager) watchSecrets(w watch.Interface) {

	for {
		select {
		case ev, ok := <-w.ResultChan():
			if !ok {
				return
			}

			rm.Lock()
			secretKey := rm.getStoreKey(ev.Object.(*v1.Secret).Namespace, ev.Object.(*v1.Secret).Name)
			switch ev.Type {
			case watch.Modified:
				delete(rm.secrets, secretKey)
			case watch.Deleted:
				delete(rm.secrets, secretKey)
			}
			rm.Unlock()
		}
	}
}

func (rm *ResourceManager) incrementRefCounters(p *v1.Pod) {
	for _, c := range p.Spec.Containers {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				configMapKey := rm.getStoreKey(p.Namespace, e.ValueFrom.ConfigMapKeyRef.Name)
				rm.configMapRef[configMapKey]++
			}

			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				secretKey := rm.getStoreKey(p.Namespace, e.ValueFrom.SecretKeyRef.Name)
				rm.secretRef[secretKey]++
			}
		}
	}

	for _, v := range p.Spec.Volumes {
		if v.VolumeSource.Secret != nil {
			secretKey := rm.getStoreKey(p.Namespace, v.VolumeSource.Secret.SecretName)
			rm.secretRef[secretKey]++
		}
	}
}

func (rm *ResourceManager) decrementRefCounters(p *v1.Pod) {
	for _, c := range p.Spec.Containers {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				configMapKey := rm.getStoreKey(p.Namespace, e.ValueFrom.ConfigMapKeyRef.Name)
				rm.configMapRef[configMapKey]--
			}

			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				secretKey := rm.getStoreKey(p.Namespace, e.ValueFrom.SecretKeyRef.Name)
				rm.secretRef[secretKey]--
			}
		}
	}

	for _, v := range p.Spec.Volumes {
		if v.VolumeSource.Secret != nil {
			secretKey := rm.getStoreKey(p.Namespace, v.VolumeSource.Secret.SecretName)
			rm.secretRef[secretKey]--
		}
	}
}

// getStoreKey return the key with namespace for store objects from different namespaces
func (rm *ResourceManager) getStoreKey(namespace, name string) string {
	return namespace + "_" + name
}
