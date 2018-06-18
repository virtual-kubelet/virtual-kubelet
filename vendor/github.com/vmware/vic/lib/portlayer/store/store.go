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

package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sync"

	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/session"
)

type StoreManager struct {
	m            sync.RWMutex
	dsStores     map[string]kvstore.KeyValueStore
	datastoreURL url.URL
}

var (
	mgr         *StoreManager
	initializer struct {
		once sync.Once
		err  error
	}
)

const (
	// available via portLayer API
	APIKV = "apiKV"
)

var (
	ErrDoesNotExist  = errors.New("requested store does not exist")
	ErrDuplicateName = errors.New("duplicate store name")
	ErrInvalidName   = errors.New("invalid store name, must be regexp(^[a-zA-Z0-9_]*$) compliant")
)

// Init will initialize the package vars and create the default portLayerKV store
//
// Note: The imgStoreURL is provided by the portlayer init function and is currently
// based on the image-store specified at appliance creation via vic-machine.  That URL
// is the starting point for the datastore persistence path and does not mean that the
// k/v stores are presisted w/the images.
func Init(ctx context.Context, session *session.Session, imgStoreURL *url.URL) error {
	defer trace.End(trace.Begin(imgStoreURL.String()))

	initializer.once.Do(func() {
		var err error
		defer func() {
			initializer.err = err
		}()

		mgr = &StoreManager{
			dsStores:     make(map[string]kvstore.KeyValueStore),
			datastoreURL: *imgStoreURL,
		}
		//create or restore the api accessible datastore backed k/v store
		_, err = NewDatastoreKeyValue(ctx, session, APIKV)
	})

	return initializer.err
}

// Store will return the requested store
func Store(name string) (kvstore.KeyValueStore, error) {
	mgr.m.RLock()
	defer mgr.m.RUnlock()

	if kv, exists := mgr.dsStores[name]; exists {
		return kv, nil
	}

	return nil, ErrDoesNotExist
}

// NewDatastoreKeyValue will validate the supplied name and create a datastore
// backed key / value store
//
// The file will be located at the init datastoreURL  -- currently that's in the
// appliance directory under the {dsFolder} folder (i.e. [datastore]vch-appliance/{dsFolder}/{name})
func NewDatastoreKeyValue(ctx context.Context, session *session.Session, name string) (kvstore.KeyValueStore, error) {
	defer trace.End(trace.Begin(name))

	mgr.m.Lock()
	defer mgr.m.Unlock()

	// validate the name
	err := validateStoreName(name)
	if err != nil {
		return nil, err
	}
	// get a ds helper for this ds url
	dsHelper, err := datastore.NewHelper(trace.NewOperation(ctx, "datastore helper creation"), session,
		session.Datastore, fmt.Sprintf("%s/%s", mgr.datastoreURL.Path, constants.KVStoreFolder))
	if err != nil {
		return nil, fmt.Errorf("unable to get datastore helper for %s store creation: %s", name, err.Error())
	}

	// create or restore the specified K/V store
	keyVal, err := kvstore.NewKeyValueStore(ctx, kvstore.NewDatastoreBackend(dsHelper), name)
	if err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("unable to create %s datastore backed store: %s", name, err.Error())
	}
	// throw it in the store map
	mgr.dsStores[name] = keyVal

	return keyVal, nil
}

// validateStoreName will validate that the store name is not in use
// and follows the regexp []
func validateStoreName(name string) error {
	// is the name already in use
	if _, dupe := mgr.dsStores[name]; dupe {
		return ErrDuplicateName
	}
	// compliant w/regexp
	re := regexp.MustCompile("^[a-zA-Z0-9_\\.]*$")
	if !re.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}
