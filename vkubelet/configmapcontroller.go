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

// ConfigMapUpdater is the interface used for notifying that a config map has
// been updated, and which (pod) objects are referencing them.
type ConfigMapUpdater interface {
	UpdateConfigMap(ctx context.Context, cm *v1.ConfigMap, refs []string) error
}

type configMapGetter interface {
	Get(namespace, name string) (*v1.ConfigMap, error)
}

type configMapInformer interface {
	HasSynced() bool
	AddUpdateHandler(func(*v1.ConfigMap, *v1.ConfigMap))
}

type configMapSync struct {
	i cache.SharedInformer
}

func (s *configMapSync) HasSynced() bool {
	return s.i.HasSynced()
}

func (s *configMapSync) AddUpdateHandler(f func(oldObj, newObj *v1.ConfigMap)) {
	s.i.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(o, n interface{}) {
			f(o.(*v1.ConfigMap), n.(*v1.ConfigMap))
		},
	})
}

type configMapStore struct {
	l corev1listers.ConfigMapLister
}

func (s *configMapStore) Get(namespace, name string) (*v1.ConfigMap, error) {
	return s.l.ConfigMaps(namespace).Get(name)
}

// NewConfigMapController creates a ConfigMapController.
func NewConfigMapController(i corev1informers.ConfigMapInformer, u ConfigMapUpdater, refs *refCounter, q workqueue.RateLimitingInterface) *ConfigMapController {
	return &ConfigMapController{
		i:    &configMapSync{i.Informer()},
		g:    &configMapStore{i.Lister()},
		u:    u,
		q:    q,
		refs: refs,
	}
}

// ConfigMapController is responsible for notifying the configured updater of
// updates to config maps *when the config maps are referenced by a running pod*
type ConfigMapController struct {
	i    configMapInformer
	g    configMapGetter
	u    ConfigMapUpdater
	q    workqueue.RateLimitingInterface
	refs *refCounter
}

// Run the ConfigMapController to start syncing config maps with the configured
// provider.
func (c *ConfigMapController) Run(ctx context.Context, numWorkers int) error {
	defer c.q.ShutDown()

	// Wait for the caches to be synced before starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), c.i.HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}

	c.i.AddUpdateHandler(c.defaultUpdateHandler)

	log.G(ctx).WithField("numWorkers", numWorkers).Info("Starting configmap workers")
	for i := 0; i < numWorkers; i++ {
		go c.runWorker(ctx, strconv.Itoa(i))
	}

	log.G(ctx).Info("started configmap workers")
	<-ctx.Done()
	log.G(ctx).Info("shutting down configmap workers")
	return nil
}

func (c *ConfigMapController) defaultUpdateHandler(_, newObj *v1.ConfigMap) {
	cm := newObj.DeepCopy()
	key := path.Join(cm.GetNamespace(), cm.GetName())
	refs := c.refs.GetRefs(key)
	if len(refs) > 0 {
		c.q.AddRateLimited(key)
	}
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item on the work queue.
func (c *ConfigMapController) runWorker(ctx context.Context, workerId string) {
	for c.processNextWorkItem(ctx, workerId) {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,by calling the syncHandler.
func (c *ConfigMapController) processNextWorkItem(ctx context.Context, workerId string) bool {
	// We create a span only after popping from the queue so that we can get an adequate picture of how long it took to process the item.
	ctx, span := trace.StartSpan(ctx, "processNextWorkItem")
	defer span.End()

	ctx = span.WithField(ctx, "workerId", workerId)
	return handleQueueItem(ctx, c.q, c.syncHandler)
}

func (c *ConfigMapController) syncHandler(ctx context.Context, key string) error {
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
		// configmap is not referenced so there is no need to sync it
		return nil
	}

	cm, err := c.g.Get(namespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		log.G(ctx).WithField("key", key).WithError(err).Error("ConfigMap missing from k8s, not processing")
		// TODO: clean up references?
		// TODO: Seems like if we get here there is a bug
		return nil
	}

	err = c.u.UpdateConfigMap(ctx, cm, refs)
	if err != nil {
		return pkgerrors.Wrap(err, "error updating configmap in provider")
	}
	return nil
}
