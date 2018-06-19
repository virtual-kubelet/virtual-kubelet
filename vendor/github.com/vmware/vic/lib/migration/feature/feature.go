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

package feature

const (
	AddCommonSpecForVCHVersion = iota + 1
	AddCommonSpecForContainerVersion
	TasksSupportedVersion
	RenameSupportedVersion
	MigrateRegistryVersion
	ExecSupportedVersion
	VicadminProxyVarRenameVersion
	InsecureRegistriesTypeChangeVersion

	// ContainerCreateTimestampVersion represents the data version where a container's
	// create time is stored in nanoseconds (previously seconds) in the portlayer.
	ContainerCreateTimestampVersion

	// VCHFolderSupportVersion represents the VCH version that first introduced
	// VM folder support for the VCH.
	VCHFolderSupportVersion

	// Add new feature flag here

	// MaxPluginVersion must be the last
	MaxPluginVersion
)
