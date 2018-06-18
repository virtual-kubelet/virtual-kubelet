// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagec

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/apiservers/engine/backends/kv"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/trace"
)

// LCache is an in-memory cache to account for existing image layers
// It is used primarily by imagec when coordinating layer downloads
// The cache is initially hydrated by way of the image cache at startup
type LCache struct {
	m      sync.RWMutex
	layers map[string]*ImageWithMeta

	client *client.PortLayer
	dirty  bool
}

// LayerNotFoundError is returned when a layer does not exist in the cache
type LayerNotFoundError struct{}

func (e LayerNotFoundError) Error() string {
	return "Layer does not exist"
}

const (
	layerCacheKey = "layers"
)

var (
	layerCache *LCache
)

func init() {
	layerCache = &LCache{
		layers: make(map[string]*ImageWithMeta),
	}
}

// LayerCache returns a reference to the layer cache
func LayerCache() *LCache {
	return layerCache
}

// InitializeLayerCache will create a new layer cache or rehydrate an
// existing layer cache from the portlayer k/v store
func InitializeLayerCache(client *client.PortLayer) error {
	defer trace.End(trace.Begin(""))

	log.Debugf("Initializing layer cache")

	layerCache.client = client

	val, err := kv.Get(client, layerCacheKey)
	if err != nil && err != kv.ErrKeyNotFound {
		return err
	}

	l := struct {
		Layers map[string]*ImageWithMeta
	}{}

	if val != "" {
		if err = json.Unmarshal([]byte(val), &l); err != nil {
			return fmt.Errorf("Failed to unmarshal layer cache: %s", err)
		}

		layerCache.layers = l.Layers
	}

	return nil
}

// Add adds a new layer to the cache
func (lc *LCache) Add(layer *ImageWithMeta) {
	defer trace.End(trace.Begin(""))
	lc.m.Lock()
	defer lc.m.Unlock()

	lc.layers[layer.ID] = layer
	lc.dirty = true
}

// Remove removes a layer from the cache
func (lc *LCache) Remove(id string) {
	defer trace.End(trace.Begin(""))
	lc.m.Lock()
	defer lc.m.Unlock()

	if _, ok := lc.layers[id]; ok {
		delete(lc.layers, id)
		lc.dirty = true
	}
}

// Commit marks a layer as downloaded
func (lc *LCache) Commit(layer *ImageWithMeta) error {
	defer trace.End(trace.Begin(""))
	lc.m.Lock()
	defer lc.m.Unlock()

	lc.layers[layer.ID] = layer
	lc.layers[layer.ID].Downloading = false
	lc.dirty = true

	return lc.save()
}

// Get returns a cached layer, or LayerNotFoundError if it doesn't exist
func (lc *LCache) Get(id string) (*ImageWithMeta, error) {
	defer trace.End(trace.Begin(""))
	lc.m.RLock()
	defer lc.m.RUnlock()

	layer, ok := lc.layers[id]
	if !ok {
		return nil, LayerNotFoundError{}
	}

	return layer, nil
}

func (lc *LCache) save() error {
	defer trace.End(trace.Begin(""))

	if !lc.dirty {
		return nil
	}

	m := struct {
		Layers map[string]*ImageWithMeta
	}{
		Layers: lc.layers,
	}

	bytes, err := json.Marshal(m)
	if err != nil {
		log.Errorf("Unable to marshal layer cache: %s", err.Error())
		return err
	}

	err = kv.Put(lc.client, layerCacheKey, string(bytes))
	if err != nil {
		log.Errorf("Unable to save layer cache: %s", err.Error())
		return err
	}

	lc.dirty = false
	return nil
}

// Save will persist the image cache to the portlayer k/v store
func (lc *LCache) Save() error {
	defer trace.End(trace.Begin(""))
	lc.m.Lock()
	defer lc.m.Unlock()

	return lc.save()
}
