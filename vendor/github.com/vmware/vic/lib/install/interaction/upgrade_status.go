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

package interaction

import (
	"fmt"

	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type upgradeStatus int

const (
	UpToDate upgradeStatus = iota
	Unknown
	InProgress
	Upgradeable
	Newer
	Invalid
)

func determineUpgradeStatus(op trace.Operation, vch *vm.VirtualMachine, installerVer, vchVer *version.Build) (upgradeStatus, error) {
	if sameVer := installerVer.Equal(vchVer); sameVer {
		return UpToDate, nil
	}

	upgrading, err := vch.VCHUpdateStatus(op)
	if err != nil {
		return Unknown, err
	}
	if upgrading {
		return InProgress, nil
	}

	canUpgrade, err := installerVer.IsNewer(vchVer)
	if err != nil {
		return Unknown, err
	}
	if canUpgrade {
		return Upgradeable, nil
	}

	oldInstaller, err := installerVer.IsOlder(vchVer)
	if err != nil {
		return Unknown, err
	}
	if oldInstaller {
		return Newer, nil
	}

	// can't get here
	return Invalid, nil
}

// GetUpgradeStatusShortMessage returns a succinct message describing the upgrade state, suitable for use in a CLI command operating on multiple VCHs.
func GetUpgradeStatusShortMessage(op trace.Operation, vch *vm.VirtualMachine, installerVer, vchVer *version.Build) string {
	status, err := determineUpgradeStatus(op, vch, installerVer, vchVer)

	switch status {
	case UpToDate:
		return "Up to date"
	case Upgradeable:
		return fmt.Sprintf("Upgradeable to %s", installerVer.ShortVersion())
	case InProgress:
		return "Upgrade in progress"
	case Newer:
		return "VCH has newer version"
	case Unknown:
		return fmt.Sprintf("Unknown: %s", err)
	default:
		return "Invalid upgrade status"
	}
}

// LogUpgradeStatusLongMessage returns a verbose message describing the upgrade state, suitable for use in a CLI command operating on a single VCH.
func LogUpgradeStatusLongMessage(op trace.Operation, vch *vm.VirtualMachine, installerVer, vchVer *version.Build) {
	status, err := determineUpgradeStatus(op, vch, installerVer, vchVer)

	switch status {
	case UpToDate:
		op.Info("Installer has same version as VCH")
		op.Info("No upgrade available with this installer version")
	case Upgradeable:
		op.Info("Upgrade available")
	case InProgress:
		op.Info("Upgrade/configure in progress")
	case Newer:
		op.Info("Installer has older version than VCH")
		op.Info("No upgrade available with this installer version")
	case Unknown:
		op.Errorf("Unable to determine upgrade status: %s", err)
	default:
		op.Warn("Invalid upgrade status")
	}
}
