// Copyright 2018 VMware, Inc. All Rights Reserved.
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

package encode

import (
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/apiservers/service/models"
)

func AsManagedObjectID(mobid string) string {
	moref := new(types.ManagedObjectReference)
	ok := moref.FromString(mobid)
	if !ok {
		return "" // TODO (#6717): Handle? (We probably don't want to let this fail the request, but may want to convey that something unexpected happened.)
	}

	return moref.Value
}

// common provides an interface for the relevant parts of object.Common
type common interface {
	Reference() types.ManagedObjectReference
	Name() string
}

func AsManagedObject(object common) models.ManagedObject {
	return models.ManagedObject{
		Name: object.Name(),
		ID:   object.Reference().Value,
	}
}
