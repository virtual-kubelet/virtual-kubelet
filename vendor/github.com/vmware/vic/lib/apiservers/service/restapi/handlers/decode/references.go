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

package decode

import (
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/client"
	"github.com/vmware/vic/pkg/trace"
)

func FromManagedObject(op trace.Operation, finder client.Finder, t string, m *models.ManagedObject) (string, error) {
	if m == nil {
		return "", nil
	}

	if m.ID != "" {
		managedObjectReference := types.ManagedObjectReference{Type: t, Value: m.ID}
		element, err := finder.Element(op, managedObjectReference)

		if err != nil {
			return "", err
		}

		return element.Path, nil
	}

	return m.Name, nil
}
