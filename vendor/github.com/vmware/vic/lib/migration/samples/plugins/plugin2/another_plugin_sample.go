// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package plugin2

import (
	"context"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/migration/errors"
	"github.com/vmware/vic/lib/migration/manager"
	"github.com/vmware/vic/lib/migration/samples/config/v2"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// Sample plugin to migrate data in keyvalue store
// If there is any key/value change, should create a new keyvalue store file with version appendix, like .v2, to differentiate with old keyvalue store file
// Migrate keyvalue plugin should read configuration from input VirtualContainerHost configuration, and then read from old keyvalue store file directly
// After migration, write back to new datastore file with version appendix
// Data migration framework is not responsible for data roll back. With versioned datastore file, even roll back happens, old version's datastore file is still useable by old binary
// Make sure to delete existing new version datastore file, which might be a left over of last failed data migration attempt.
const (
	version = 2
	target  = manager.ApplianceConfigure

	KVStoreFolder = "kvStores"
	APIKV         = "apiKV"

	oldKey = "image.name"
	newKey = "image.tag"
)

func init() {
	log.Debugf("Registering plugin %s:%d", target, version)
	if err := manager.Migrator.Register(version, target, &NewImageMeta{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, version, err)
	}
}

// NewImageMeta is plugin for vic 0.8.0-GA version upgrade
type NewImageMeta struct {
}

func (p *NewImageMeta) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("%d", version)))
	if data == nil {
		return nil
	}
	vchConfMap := data.(map[string]string)
	// No plugin query keyvalue store yet, load from datastore file
	// get a ds helper for this ds url
	vchConf := &v2.VirtualContainerHostConfigSpec{}
	extraconfig.Decode(extraconfig.MapSource(vchConfMap), vchConf)

	imageURL := vchConf.ImageStores[0]
	// TODO: sample code, should get datastore from imageURL
	dsHelper, err := datastore.NewHelper(trace.NewOperation(ctx, "datastore helper creation"), s,
		s.Datastore, fmt.Sprintf("%s/%s", imageURL.Path, KVStoreFolder))
	if err != nil {
		return &errors.InternalError{
			Message: fmt.Sprintf("unable to get datastore helper for %s store creation: %s", APIKV, err.Error()),
		}
	}
	// restore the modified K/V store
	oldKeyValStore, err := kvstore.NewKeyValueStore(ctx, kvstore.NewDatastoreBackend(dsHelper), APIKV)
	if err != nil && !os.IsExist(err) {
		return &errors.InternalError{
			Message: fmt.Sprintf("unable to create %s datastore backed store: %s", APIKV, err.Error()),
		}
	}

	// create new k/v store with version appendix v2
	newDsFile := fmt.Sprintf("%s.v%d", APIKV, version)
	// try to remove new k/v store file in case it's created already
	dsHelper.Rm(ctx, newDsFile)
	newKeyValueStore, err := kvstore.NewKeyValueStore(ctx, kvstore.NewDatastoreBackend(dsHelper), newDsFile)
	if err != nil && !os.IsExist(err) {
		return &errors.InternalError{
			Message: fmt.Sprintf("unable to create %s datastore backed store: %s", newDsFile, err.Error()),
		}
	}

	// copy all key/value from old k/v store
	allKeyVals, err := oldKeyValStore.List(".*")
	if err != nil {
		return &errors.InternalError{
			Message: fmt.Sprintf("unable to list key/value store %s: %s", APIKV, err.Error()),
		}
	}

	for key, val := range allKeyVals {
		newKeyValueStore.Put(ctx, key, val)
	}
	val, err := newKeyValueStore.Get(oldKey)
	if err != nil && err != kvstore.ErrKeyNotFound {
		return &errors.InternalError{
			Message: fmt.Sprintf("failed to get %s from store %s: %s", oldKey, APIKV, err.Error()),
		}
	}
	// put the new key/value to store, and leave the old key/value there, in case upgrade failed, old binary still works well with half-changed store
	newKeyValueStore.Put(ctx, newKey, []byte(fmt.Sprintf("%s:%s", val, "latest")))
	// persist new data back to vsphere, framework does not take of it
	newKeyValueStore.Save(ctx)
	return nil
}
