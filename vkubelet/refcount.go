package vkubelet

import (
	"path"
	"sync"

	v1 "k8s.io/api/core/v1"
)

type refCounter struct {
	mu   sync.Mutex
	refs map[string]map[string]struct{}
}

func newRefCounter() *refCounter {
	return &refCounter{refs: make(map[string]map[string]struct{})}
}

func (c *refCounter) AddReference(key, ref string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.refs[key] == nil {
		c.refs[key] = make(map[string]struct{})
	}

	c.refs[key][ref] = struct{}{}
}

func (c *refCounter) Dereference(key, ref string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.refs[key], ref)

	if len(c.refs[key]) > 0 {
		return
	}

	delete(c.refs, key)
}

func (c *refCounter) GetRefs(key string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	objs := c.refs[key]
	if len(objs) == 0 {
		return nil
	}

	ls := make([]string, 0, len(objs))
	for k := range objs {
		ls = append(ls, k)
	}
	return ls
}

func (s *Server) deleteReferencesFromPod(p *v1.Pod) {
	configRefs := make(map[string]struct{})
	secretRefs := make(map[string]struct{})
	getPodRefs(p, configRefs, secretRefs)

	obj := path.Join(p.GetNamespace(), p.GetName())
	for r := range configRefs {
		s.configMapRefs.Dereference(path.Join(p.GetNamespace(), r), obj)
	}
	for r := range secretRefs {
		s.secretRefs.Dereference(path.Join(p.GetNamespace(), r), obj)
	}
}

func getPodRefs(p *v1.Pod, configRefs, secretRefs map[string]struct{}) {
	for _, v := range p.Spec.Volumes {
		if v.VolumeSource.ConfigMap != nil {
			configRefs[v.Name] = struct{}{}
		}
		if v.VolumeSource.Secret != nil {
			secretRefs[v.Name] = struct{}{}
		}
	}

	for _, c := range p.Spec.InitContainers {
		getRefsForContainer(&c, configRefs, secretRefs)
	}

	for _, c := range p.Spec.Containers {
		getRefsForContainer(&c, configRefs, secretRefs)
	}

}

func (s *Server) addPodReferences(p *v1.Pod) {
	configRefs := make(map[string]struct{})
	secretRefs := make(map[string]struct{})
	getPodRefs(p, configRefs, secretRefs)

	obj := path.Join(p.GetNamespace(), p.GetName())

	for r := range configRefs {
		s.configMapRefs.AddReference(path.Join(p.GetNamespace(), r), obj)
	}
	for r := range secretRefs {
		s.secretRefs.AddReference(path.Join(p.GetNamespace(), r), obj)
	}
}

func getRefsForContainer(c *v1.Container, configRefs, secretRefs map[string]struct{}) {
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			continue
		}
		if ref := e.ValueFrom.ConfigMapKeyRef; ref != nil {
			configRefs[ref.Name] = struct{}{}
		}
		if ref := e.ValueFrom.SecretKeyRef; ref != nil {
			secretRefs[ref.Name] = struct{}{}
		}
	}
}
