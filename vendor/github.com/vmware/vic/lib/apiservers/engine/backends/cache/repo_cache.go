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

package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/vmware/vic/lib/apiservers/engine/backends/kv"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/reference"

	log "github.com/Sirupsen/logrus"
)

// repoCache is a cache of the docker repository information.
// This info will help to provide proper tag and digest support
//
// The cache will be persisted to disk via the portlayer k/v
// store and will be restored at system start
//
// This code is a heavy leverage of docker's reference store:
// github.com/docker/docker/reference/store.go

var (
	rCache  *repoCache
	repoKey = "repositories"
)

// Repo provides the set of methods which can operate on a tag store.
type Repo interface {
	References(imageID string) []reference.Named
	ReferencesByName(ref reference.Named) []Association
	Delete(ref reference.Named, save bool) (bool, error)
	Get(ref reference.Named) (string, error)

	Save() error
	GetImageID(layerID string) string
	Tags(imageID string) []string
	Digests(imageID string) []string
	AddReference(ref reference.Named, imageID string, force bool, layerID string, save bool) error

	// Remove will remove from the cache and returns the
	// stringified Named if successful -- save bool instructs
	// func to persist to portlayer k/v or not
	Remove(ref string, save bool) (string, error)
}

type repoCache struct {
	// client is needed for k/v store operations
	client *client.PortLayer

	mu sync.RWMutex
	// repositories is a map of repositories, indexed by name.
	Repositories map[string]repository
	// referencesByIDCache is a cache of references indexed by imageID
	referencesByIDCache map[string]map[string]reference.Named
	// Layers is a map of layerIDs to imageIDs
	// TODO: we might be able to remove this later -- currently
	// needed because an ImageID isn't generated for every pull
	Layers map[string]string
	// images is a map of imageIDs to layerIDs
	// TODO: much like the Layers map this might be able to be
	// removed
	images map[string]string
}

// Repository maps tags to image IDs. The key is a a stringified Reference,
// including the repository name.
type repository map[string]string

var (
	// ErrDoesNotExist returned if a reference is not found in the
	// store.
	ErrDoesNotExist = errors.New("reference does not exist")
)

// An Association is a tuple associating a reference with an image ID.
type Association struct {
	Ref     reference.Named
	ImageID string
}

type lexicalRefs []reference.Named

func (a lexicalRefs) Len() int           { return len(a) }
func (a lexicalRefs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a lexicalRefs) Less(i, j int) bool { return a[i].String() < a[j].String() }

type lexicalAssociations []Association

func (a lexicalAssociations) Len() int           { return len(a) }
func (a lexicalAssociations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a lexicalAssociations) Less(i, j int) bool { return a[i].Ref.String() < a[j].Ref.String() }

// RepositoryCache returns a ref to the repoCache interface
func RepositoryCache() Repo {
	return rCache
}

func init() {
	rCache = &repoCache{
		Repositories:        make(map[string]repository),
		Layers:              make(map[string]string),
		images:              make(map[string]string),
		referencesByIDCache: make(map[string]map[string]reference.Named),
	}
}

// NewRespositoryCache will create a new repoCache or rehydrate
// an existing repoCache from the portlayer k/v store
func NewRepositoryCache(client *client.PortLayer) error {
	rCache.client = client

	val, err := kv.Get(client, repoKey)
	if err != nil && err != kv.ErrKeyNotFound {
		return err
	}
	if val != "" {
		if err = json.Unmarshal([]byte(val), rCache); err != nil {
			return fmt.Errorf("Failed to unmarshal repository cache: %s", err)
		}
		// hydrate refByIDCache
		for _, repository := range rCache.Repositories {
			for refStr, refID := range repository {
				// #nosec: Errors unhandled.
				ref, _ := reference.ParseNamed(refStr)
				if rCache.referencesByIDCache[refID] == nil {
					rCache.referencesByIDCache[refID] = make(map[string]reference.Named)
				}
				rCache.referencesByIDCache[refID][refStr] = ref
			}
		}
		// hydrate image -> layer cache
		for image, layer := range rCache.Layers {
			rCache.images[image] = layer
		}

		log.Infof("found %d repositories", len(rCache.Repositories))
		log.Infof("found %d image layers", len(rCache.Layers))
	}
	return nil
}

// Save will persist the repository cache to the
// portlayer k/v
func (store *repoCache) Save() error {
	b, err := json.Marshal(store)
	if err != nil {
		log.Errorf("Unable to marshal repository cache: %s", err.Error())
		return err
	}

	err = kv.Put(store.client, repoKey, string(b))
	if err != nil {
		log.Errorf("Unable to save repository cache: %s", err.Error())
		return err
	}

	return nil
}

func (store *repoCache) AddReference(ref reference.Named, imageID string, force bool, layerID string, save bool) error {
	if ref.Name() == string(digest.Canonical) {
		return errors.New("refusing to create an ambiguous tag using digest algorithm as name")
	}
	var err error
	store.mu.Lock()
	defer store.mu.Unlock()

	// does this repo (i.e. busybox) exist?
	repository, exists := store.Repositories[ref.Name()]
	if !exists || repository == nil {
		repository = make(map[string]string)
		store.Repositories[ref.Name()] = repository
	}

	refStr := ref.String()
	oldID, exists := repository[refStr]

	if exists {
		if oldID == imageID {
			log.Debugf("Image %s is already tagged as %s", oldID, ref.String())
			return nil
		}

		// force only works for tags
		if digested, isDigest := ref.(reference.Canonical); isDigest {
			log.Debugf("Unable to overwrite %s with digest %s", oldID, digested.Digest().String())

			return fmt.Errorf("Cannot overwrite digest %s", digested.Digest().String())
		}

		if !force {
			log.Debugf("Refusing to overwrite %s with %s unless force is specified", oldID, ref.String())

			return fmt.Errorf("Conflict: Tag %s is already set to image %s, if you want to replace it, please use -f option", ref.String(), oldID)
		}

		if store.referencesByIDCache[oldID] != nil {
			delete(store.referencesByIDCache[oldID], refStr)
			if len(store.referencesByIDCache[oldID]) == 0 {
				delete(store.referencesByIDCache, oldID)
			}
		}
	}

	repository[refStr] = imageID
	if store.referencesByIDCache[imageID] == nil {
		store.referencesByIDCache[imageID] = make(map[string]reference.Named)
	}
	store.referencesByIDCache[imageID][refStr] = ref

	if layerID != "" {
		store.Layers[layerID] = imageID
		store.images[imageID] = layerID
	}
	// should we save this input?
	if save {
		err = store.Save()
	}

	return err
}

// Remove is a convenience function to allow the passing of a properly
// formed string that can be parsed into a Named object.
//
// Examples:
// Tags: busybox:1.25.1
// Digest: nginx@sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64
func (store *repoCache) Remove(ref string, save bool) (string, error) {
	n, err := reference.ParseNamed(ref)
	if err != nil {
		return "", err
	}

	_, err = store.Delete(n, save)
	if err != nil {
		return "", err
	}

	return n.String(), nil
}

// Delete deletes a reference from the store. It returns true if a deletion
// happened, or false otherwise.
func (store *repoCache) Delete(ref reference.Named, save bool) (bool, error) {
	ref = reference.WithDefaultTag(ref)

	store.mu.Lock()
	defer store.mu.Unlock()
	var err error
	// return code -- assume success
	rtc := true
	repoName := ref.Name()

	repository, exists := store.Repositories[repoName]
	if !exists {
		return false, ErrDoesNotExist
	}
	refStr := ref.String()
	if imageID, exists := repository[refStr]; exists {
		delete(repository, refStr)
		if len(repository) == 0 {
			delete(store.Repositories, repoName)
		}
		if store.referencesByIDCache[imageID] != nil {
			delete(store.referencesByIDCache[imageID], refStr)
			if len(store.referencesByIDCache[imageID]) == 0 {
				delete(store.referencesByIDCache, imageID)
			}
		}
		if layer, exists := store.images[imageID]; exists {
			delete(store.Layers, imageID)
			delete(store.images, layer)
		}
		if save {
			err = store.Save()
			if err != nil {
				rtc = false
			}
		}
		return rtc, err
	}

	return false, ErrDoesNotExist
}

// GetImageID will return the imageID associated with the
// specified layerID
func (store *repoCache) GetImageID(layerID string) string {
	var imageID string
	store.mu.RLock()
	defer store.mu.RUnlock()
	if image, exists := store.Layers[layerID]; exists {
		imageID = image
	}
	return imageID
}

// Get returns the imageID for a parsed reference
func (store *repoCache) Get(ref reference.Named) (string, error) {
	ref = reference.WithDefaultTag(ref)

	store.mu.RLock()
	defer store.mu.RUnlock()

	repository, exists := store.Repositories[ref.Name()]
	if !exists || repository == nil {
		return "", ErrDoesNotExist
	}
	imageID, exists := repository[ref.String()]
	if !exists {
		return "", ErrDoesNotExist
	}

	return imageID, nil
}

// Tags returns a slice of tags for the specified imageID
func (store *repoCache) Tags(imageID string) []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	var tags []string
	for _, ref := range store.referencesByIDCache[imageID] {
		if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
			tags = append(tags, tagged.String())
		}
	}
	return tags
}

// Digests returns a slice of digests for the specified imageID
func (store *repoCache) Digests(imageID string) []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	var digests []string
	for _, ref := range store.referencesByIDCache[imageID] {
		if d, isCanonical := ref.(reference.Canonical); isCanonical {
			digests = append(digests, d.String())
		}
	}
	return digests
}

// References returns a slice of references to the given imageID. The slice
// will be nil if there are no references to this imageID.
func (store *repoCache) References(imageID string) []reference.Named {
	store.mu.RLock()
	defer store.mu.RUnlock()

	// Convert the internal map to an array for two reasons:
	// 1) We must not return a mutable
	// 2) It would be ugly to expose the extraneous map keys to callers.

	var references []reference.Named
	for _, ref := range store.referencesByIDCache[imageID] {
		references = append(references, ref)
	}

	sort.Sort(lexicalRefs(references))

	return references
}

// ReferencesByName returns the references for a given repository name.
// If there are no references known for this repository name,
// ReferencesByName returns nil.
func (store *repoCache) ReferencesByName(ref reference.Named) []Association {
	store.mu.RLock()
	defer store.mu.RUnlock()

	repository, exists := store.Repositories[ref.Name()]
	if !exists {
		return nil
	}

	var associations []Association
	for refStr, refID := range repository {
		ref, err := reference.ParseNamed(refStr)
		if err != nil {
			// Should never happen
			return nil
		}
		associations = append(associations,
			Association{
				Ref:     ref,
				ImageID: refID,
			})
	}

	sort.Sort(lexicalAssociations(associations))

	return associations
}
