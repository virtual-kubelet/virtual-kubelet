package vkubelet

import (
	"context"
	"path"
	"testing"
	"time"

	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
)

func TestConfigMapController(t *testing.T) {
	u := &testConfigMapUpdater{ch: make(chan configMapUpdate)}
	si, sc := newTestConfigMapController(t, u)
	si.s.Add("test/1", &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "1"}})
	sc.q.Add("test/1")
	sc.refs.AddReference("test/1", "foo")
	sc.refs.AddReference("test/2", "foo")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sc.Run(ctx, 1)

	select {
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for configMap update")
	case update := <-u.ch:
		assert.Equal(t, update.cm.Namespace, "test")
		assert.Equal(t, update.cm.Name, "1")
		assert.Equal(t, len(update.refs), 1)
		assert.Equal(t, update.refs[0], "foo")
	}
}

type testConfigMapUpdater struct {
	ch chan configMapUpdate
}

type configMapUpdate struct {
	cm   *v1.ConfigMap
	refs []string
}

func (u *testConfigMapUpdater) UpdateConfigMap(ctx context.Context, cm *v1.ConfigMap, refs []string) error {
	u.ch <- configMapUpdate{cm: cm, refs: refs}
	return nil
}

func newTestConfigMapController(t *testing.T, u ConfigMapUpdater) (*testConfigMapInformer, *ConfigMapController) {
	i := &testConfigMapInformer{s: &testConfigMapStore{s: make(map[string]*v1.ConfigMap)}}
	return i, &ConfigMapController{
		q:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), t.Name()),
		i:    i,
		g:    i,
		u:    u,
		refs: newRefCounter(),
	}
}

type testConfigMapInformer struct {
	h func(o, n *v1.ConfigMap)
	s *testConfigMapStore
}

func (i *testConfigMapInformer) Get(namespace, name string) (*v1.ConfigMap, error) {
	cm := i.s.Get(path.Join(namespace, name))
	if cm == nil {
		return nil, errors.NewNotFound(schema.GroupResource{}, "not found")
	}
	return cm, nil
}

func (i *testConfigMapInformer) AddUpdateHandler(h func(*v1.ConfigMap, *v1.ConfigMap)) {
	i.h = h
}

func (testConfigMapInformer) HasSynced() bool {
	return true
}

type testConfigMapStore struct {
	s map[string]*v1.ConfigMap
}

func (s *testConfigMapStore) Get(key string) *v1.ConfigMap {
	return s.s[key]
}

func (s *testConfigMapStore) Add(key string, configMap *v1.ConfigMap) {
	s.s[key] = configMap
}
