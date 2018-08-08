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
	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/install/management"
)

// FromDeletionSpecification parses a DeletionSpecification into granular deletion settings expressed via iotas
func FromDeletionSpecification(specification *models.DeletionSpecification) (deleteContainers *management.DeleteContainers, deleteVolumeStores *management.DeleteVolumeStores) {
	if specification != nil {
		if specification.Containers != nil {
			var dc management.DeleteContainers

			switch *specification.Containers {
			case models.DeletionSpecificationContainersAll:
				dc = management.AllContainers
			case models.DeletionSpecificationContainersOff:
				dc = management.PoweredOffContainers
			default:
				panic("Deletion API handler received unexpected input")
			}

			deleteContainers = &dc
		}

		if specification.VolumeStores != nil {
			var dv management.DeleteVolumeStores

			switch *specification.VolumeStores {
			case models.DeletionSpecificationVolumeStoresAll:
				dv = management.AllVolumeStores
			case models.DeletionSpecificationVolumeStoresNone:
				dv = management.NoVolumeStores
			default:
				panic("Deletion API handler received unexpected input")
			}

			deleteVolumeStores = &dv
		}
	}

	return
}
