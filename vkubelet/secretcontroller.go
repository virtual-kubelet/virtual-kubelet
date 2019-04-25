package vkubelet

import (
	"context"
	"path"
	"strconv"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// SecretUpdater is the interface used for notifying that a secret has
// been updated, and which (pod) objects are referencing them.
type SecretUpdater interface {
	UpdateSecret(ctx context.Context, cm *v1.Secret, refs []string) error
}

type secretGetter interface {
	Get(namespace, name string) (*v1.Secret, error)
}

type secretInformer interface {
	AddUpdateHandler(func(oldObj, newOBj *v1.Secret))
	HasSynced() bool
}

type secretSync struct {
	i cache.SharedInformer
}

func (s *secretSync) HasSynced() bool {
	return s.i.HasSynced()
}

func (s *secretSync) AddUpdateHandler(f func(oldObj, newObj *v1.Secret)) {
	s.i.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(o, n interface{}) {
			f(o.(*v1.Secret), n.(*v1.Secret))
		},
	})
}

type secretStore struct {
	l corev1listers.SecretLister
}

func (s *secretStore) Get(namespace, name string) (*v1.Secret, error) {
	return s.l.Secrets(namespace).Get(name)
}

// NewSecretController creates a SecretController.
func NewSecretController(i corev1informers.SecretInformer, u SecretUpdater, refs *refCounter, q workqueue.RateLimitingInterface) *SecretController {
	return &SecretController{
		i:    &secretSync{i: i.Informer()},
		g:    &secretStore{l: i.Lister()},
		u:    u,
		q:    q,
		refs: refs,
	}
}

// SecretUpdateHandlerFunc a kind of function that is called when a secret
// is updated in Kubernetes
type SecretUpdateHandlerFunc func(*v1.Secret, *v1.Secret)

// SecretController is responsible for notifying the configured updater of
// updates to secrets *when the secrets are referenced by a running pod*
type SecretController struct {
	i    secretInformer
	g    secretGetter
	u    SecretUpdater
	q    workqueue.RateLimitingInterface
	h    SecretUpdateHandlerFunc
	refs *refCounter
}

// Run the SecretController to start syncing secret with the configured
// provider.
func (c *SecretController) Run(ctx context.Context, numWorkers int) error {
	defer c.q.ShutDown()

	// Wait for the caches to be synced before starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), c.i.HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}

	c.i.AddUpdateHandler(c.defaultUpdateHandler)

	log.G(ctx).WithField("numWorkers", numWorkers).Info("Starting secret workers")
	for i := 0; i < numWorkers; i++ {
		go c.runWorker(ctx, strconv.Itoa(i))
	}

	log.G(ctx).Info("started secret workers")
	<-ctx.Done()
	log.G(ctx).Info("shutting down secret workers")
	return nil
}

func (c *SecretController) defaultUpdateHandler(_, newObj *v1.Secret) {
	s := newObj.DeepCopy()
	key := path.Join(s.GetNamespace(), s.GetName())
	refs := c.refs.GetRefs(key)
	if len(refs) > 0 {
		c.q.AddRateLimited(key)
	}
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item on the work queue.
func (c *SecretController) runWorker(ctx context.Context, workerId string) {
	for c.processNextWorkItem(ctx, workerId) {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,by calling the syncHandler.
func (c *SecretController) processNextWorkItem(ctx context.Context, workerId string) bool {
	// We create a span only after popping from the queue so that we can get an adequate picture of how long it took to process the item.
	ctx, span := trace.StartSpan(ctx, "processNextWorkItem")
	defer span.End()

	ctx = span.WithField(ctx, "workerId", workerId)
	return handleQueueItem(ctx, c.q, c.syncHandler)
}

func (c *SecretController) syncHandler(ctx context.Context, key string) error {
	ctx, span := trace.StartSpan(ctx, "syncHandler")
	defer span.End()

	ctx = span.WithField(ctx, "key", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.G(ctx).Warn(pkgerrors.Wrapf(err, "invalid resource key: %q", key))
		return nil
	}

	refs := c.refs.GetRefs(path.Join(namespace, name))
	if len(refs) == 0 {
		// secret is not referenced so there is no need to sync it
		return nil
	}

	cm, err := c.g.Get(namespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		log.G(ctx).WithField("key", key).WithError(err).Error("Secret missing from k8s, not processing")
		// TODO: clean up references?
		// TODO: Seems like if we get here there is a bug
		return nil
	}

	err = c.u.UpdateSecret(ctx, cm, refs)
	if err != nil {
		return pkgerrors.Wrap(err, "error updating secret in provider")
	}
	return nil
}
