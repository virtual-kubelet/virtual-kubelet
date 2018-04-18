package manager

import (
	"log"
	"sync"
	"time"

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
	configMapRef map[string]int64
	configMaps   map[string]*v1.ConfigMap
	secretRef    map[string]int64
	secrets      map[string]*v1.Secret
}

// NewResourceManager returns a ResourceManager with the internal maps initialized.
func NewResourceManager(k8sClient kubernetes.Interface) *ResourceManager {
	rm := ResourceManager{
		pods:         make(map[string]*v1.Pod, 0),
		configMapRef: make(map[string]int64, 0),
		secretRef:    make(map[string]int64, 0),
		configMaps:   make(map[string]*v1.ConfigMap, 0),
		secrets:      make(map[string]*v1.Secret, 0),
		k8sClient:    k8sClient,
	}

	go rm.watchConfigMaps()
	go rm.watchSecrets()

	tick := time.Tick(5 * time.Minute)
	go func() {
		for range tick {
			rm.Lock()
			for n, c := range rm.secretRef {
				if c <= 0 {
					delete(rm.secrets, n)
				}
			}
			for n := range rm.secrets {
				if _, ok := rm.secretRef[n]; !ok {
					delete(rm.secrets, n)
				}
			}
			for n, c := range rm.configMapRef {
				if c <= 0 {
					delete(rm.configMaps, n)
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

	return &rm
}

// SetPods clears the internal cache and populates it with the supplied pods.
func (rm *ResourceManager) SetPods(pods *v1.PodList) {
	rm.Lock()
	defer rm.Unlock()

	rm.pods = make(map[string]*v1.Pod, len(pods.Items))
	rm.configMapRef = make(map[string]int64, 0)
	rm.secretRef = make(map[string]int64, 0)
	rm.configMaps = make(map[string]*v1.ConfigMap, len(pods.Items))
	rm.secrets = make(map[string]*v1.Secret, len(pods.Items))

	for _, p := range pods.Items {
		if p.Status.Phase == v1.PodSucceeded {
			continue
		}
		tmp := p
		rm.pods[p.Name] = &tmp

		rm.incrementRefCounters(&tmp)
	}
}

// AddPod adds a pod to the internal cache.
func (rm *ResourceManager) AddPod(p *v1.Pod) {
	rm.Lock()
	defer rm.Unlock()
	if p.Status.Phase == v1.PodSucceeded {
		return
	}

	if _, ok := rm.pods[p.Name]; ok {
		rm.UpdatePod(p)
		return
	}

	rm.pods[p.Name] = p
	rm.incrementRefCounters(p)
}

// UpdatePod updates the supplied pod in the cache.
func (rm *ResourceManager) UpdatePod(p *v1.Pod) {
	rm.Lock()
	defer rm.Unlock()

	if p.Status.Phase == v1.PodSucceeded {
		delete(rm.pods, p.Name)
	}

	if old, ok := rm.pods[p.Name]; ok {
		rm.decrementRefCounters(old)
	}
	rm.incrementRefCounters(p)

	rm.pods[p.Name] = p
}

// DeletePod removes the pod from the cache.
func (rm *ResourceManager) DeletePod(p *v1.Pod) {
	rm.Lock()
	defer rm.Unlock()

	if old, ok := rm.pods[p.Name]; ok {
		rm.decrementRefCounters(old)
		delete(rm.pods, p.Name)
	}
}

// GetPod retrieves the specified pod from the cache. It returns nil if a pod is not found.
func (rm *ResourceManager) GetPod(name string) *v1.Pod {
	rm.RLock()
	defer rm.RUnlock()

	if p, ok := rm.pods[name]; ok {
		return p
	}

	return nil
}

// GetPods returns a list of all known pods assigned to this virtual node.
func (rm *ResourceManager) GetPods() []*v1.Pod {
	rm.RLock()
	defer rm.RUnlock()

	pods := make([]*v1.Pod, 0, len(rm.pods))
	for _, p := range rm.pods {
		pods = append(pods, p)
	}

	return pods
}

// GetConfigMap returns the specified ConfigMap from Kubernetes. It retrieves it from cache if there
func (rm *ResourceManager) GetConfigMap(name, namespace string) (*v1.ConfigMap, error) {
	rm.Lock()
	defer rm.Unlock()

	if cm, ok := rm.configMaps[name]; ok {
		return cm, nil
	}

	var opts metav1.GetOptions
	cm, err := rm.k8sClient.CoreV1().ConfigMaps(namespace).Get(name, opts)
	if err != nil {
		return nil, err
	}
	rm.configMaps[name] = cm

	return cm, err
}

// GetSecret returns the specified ConfigMap from Kubernetes. It retrieves it from cache if there
func (rm *ResourceManager) GetSecret(name, namespace string) (*v1.Secret, error) {
	rm.Lock()
	defer rm.Unlock()

	if secret, ok := rm.secrets[name]; ok {
		return secret, nil
	}

	var opts metav1.GetOptions
	secret, err := rm.k8sClient.CoreV1().Secrets(namespace).Get(name, opts)
	if err != nil {
		return nil, err
	}
	rm.secrets[name] = secret

	return secret, err
}

// watchConfigMaps monitors the kubernetes API for modifications and deletions of configmaps
// it evicts them from the internal cache
func (rm *ResourceManager) watchConfigMaps() {
	var opts metav1.ListOptions
	w, err := rm.k8sClient.CoreV1().ConfigMaps(v1.NamespaceAll).Watch(opts)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case ev, ok := <-w.ResultChan():
			if !ok {
				return
			}

			rm.Lock()
			switch ev.Type {
			case watch.Modified:
				delete(rm.configMaps, ev.Object.(*v1.ConfigMap).Name)
			case watch.Deleted:
				delete(rm.configMaps, ev.Object.(*v1.ConfigMap).Name)
			}
			rm.Unlock()
		}
	}
}

// watchSecretes monitors the kubernetes API for modifications and deletions of secrets
// it evicts them from the internal cache
func (rm *ResourceManager) watchSecrets() {
	var opts metav1.ListOptions
	w, err := rm.k8sClient.CoreV1().ConfigMaps(v1.NamespaceAll).Watch(opts)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case ev, ok := <-w.ResultChan():
			if !ok {
				return
			}

			rm.Lock()
			switch ev.Type {
			case watch.Modified:
				delete(rm.configMaps, ev.Object.(*v1.ConfigMap).Name)
			case watch.Deleted:
				delete(rm.configMaps, ev.Object.(*v1.ConfigMap).Name)
			}
			rm.Unlock()
		}
	}
}

func (rm *ResourceManager) incrementRefCounters(p *v1.Pod) {
	for _, c := range p.Spec.Containers {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				rm.configMapRef[e.ValueFrom.ConfigMapKeyRef.Name]++
			}

			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				rm.secretRef[e.ValueFrom.SecretKeyRef.Name]++
			}
		}
	}

	for _, v := range p.Spec.Volumes {
		if v.VolumeSource.Secret != nil {
			rm.secretRef[v.VolumeSource.Secret.SecretName]++
		}
	}
}

func (rm *ResourceManager) decrementRefCounters(p *v1.Pod) {
	for _, c := range p.Spec.Containers {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				rm.configMapRef[e.ValueFrom.ConfigMapKeyRef.Name]--
			}

			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				rm.secretRef[e.ValueFrom.SecretKeyRef.Name]--
			}
		}
	}

	for _, v := range p.Spec.Volumes {
		if v.VolumeSource.Secret != nil {
			rm.secretRef[v.VolumeSource.Secret.SecretName]--
		}
	}
}
