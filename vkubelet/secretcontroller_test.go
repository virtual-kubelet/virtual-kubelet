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

func TestSecretController(t *testing.T) {
	u := &testSecretUpdater{ch: make(chan secretUpdate)}
	si, sc := newTestSecretController(t, u)
	si.s.Add("test/1", &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "1"}})
	sc.q.Add("test/1")
	sc.refs.AddReference("test/1", "foo")
	sc.refs.AddReference("test/2", "foo")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sc.Run(ctx, 1)

	select {
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for secret update")
	case update := <-u.ch:
		assert.Equal(t, update.s.Namespace, "test")
		assert.Equal(t, update.s.Name, "1")
		assert.Equal(t, len(update.refs), 1)
		assert.Equal(t, update.refs[0], "foo")
	}
}

type testSecretUpdater struct {
	ch chan secretUpdate
}

type secretUpdate struct {
	s    *v1.Secret
	refs []string
}

func (u *testSecretUpdater) UpdateSecret(ctx context.Context, s *v1.Secret, refs []string) error {
	u.ch <- secretUpdate{s: s, refs: refs}
	return nil
}

func newTestSecretController(t *testing.T, u SecretUpdater) (*testSecretInformer, *SecretController) {
	i := &testSecretInformer{s: &testSecretStore{s: make(map[string]*v1.Secret)}}
	return i, &SecretController{
		q:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), t.Name()),
		i:    i,
		g:    i,
		u:    u,
		refs: newRefCounter(),
	}
}

type testSecretInformer struct {
	h func(o, n *v1.Secret)
	s *testSecretStore
}

func (i *testSecretInformer) Get(namespace, name string) (*v1.Secret, error) {
	s := i.s.Get(path.Join(namespace, name))
	if s == nil {
		return nil, errors.NewNotFound(schema.GroupResource{}, "not found")
	}
	return s, nil
}

func (i *testSecretInformer) AddUpdateHandler(h func(*v1.Secret, *v1.Secret)) {
	i.h = h
}

func (testSecretInformer) HasSynced() bool {
	return true
}

type testSecretStore struct {
	s map[string]*v1.Secret
}

func (s *testSecretStore) Get(key string) *v1.Secret {
	return s.s[key]
}

func (s *testSecretStore) Add(key string, secret *v1.Secret) {
	s.s[key] = secret
}
