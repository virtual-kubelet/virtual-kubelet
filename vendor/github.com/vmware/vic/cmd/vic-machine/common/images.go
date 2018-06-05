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

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

const (
	ApplianceImageKey  = "core"
	LinuxImageKey      = "linux"
	ApplianceImageName = "appliance.iso"
	LinuxImageName     = "bootstrap.iso"

	// An ISO 9660 sector is normally 2 KiB long. Although the specification allows for alternative sector sizes, you will rarely find anything other than 2 KiB.
	ISO9660SectorSize = 2048
	ISOVolumeSector   = 0x10
	PublisherOffset   = 318
)

var (
	images = map[string][]string{
		ApplianceImageKey: {ApplianceImageName},
		LinuxImageKey:     {LinuxImageName},
	}
)

type Images struct {
	ApplianceISO string
	BootstrapISO string
	OSType       string
}

func (i *Images) ImageFlags(hidden bool) []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "appliance-iso, ai",
			Value:       "",
			Usage:       "The appliance iso",
			Destination: &i.ApplianceISO,
			Hidden:      hidden,
		},
		cli.StringFlag{
			Name:        "bootstrap-iso, bi",
			Value:       "",
			Usage:       "The bootstrap iso",
			Destination: &i.BootstrapISO,
			Hidden:      hidden,
		},
	}
}

func (i *Images) CheckImagesFiles(op trace.Operation, force bool) (map[string]string, error) {
	defer trace.End(trace.Begin("", op))

	i.OSType = "linux"
	// detect images files
	osImgs, ok := images[i.OSType]
	if !ok {
		return nil, fmt.Errorf("Specified OS %q is not known to this installer", i.OSType)
	}

	imgs := make(map[string]string)
	result := make(map[string]string)
	if i.ApplianceISO == "" {
		i.ApplianceISO = images[ApplianceImageKey][0]
	}
	imgs[ApplianceImageName] = i.ApplianceISO

	if i.BootstrapISO == "" {
		i.BootstrapISO = osImgs[0]
	}
	imgs[LinuxImageName] = i.BootstrapISO

	for name, img := range imgs {
		_, err := os.Stat(img)
		if os.IsNotExist(err) {
			var dir string
			dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
			_, err = os.Stat(filepath.Join(dir, img))
			if err == nil {
				img = filepath.Join(dir, img)
			}
		}

		if os.IsNotExist(err) {
			op.Warnf("\t\tUnable to locate %s in the current or installer directory.", img)
			return nil, err
		}

		version, err := i.checkImageVersion(op, img, force)
		if err != nil {
			op.Error(err)
			return nil, err
		}
		versionedName := fmt.Sprintf("%s-%s", version, name)
		result[versionedName] = img
		if name == ApplianceImageName {
			i.ApplianceISO = versionedName
		} else {
			i.BootstrapISO = versionedName
		}
	}

	return result, nil
}

// GetImageVersion will read iso file version from Primary Volume Descriptor, field "Publisher Identifier"
func (i *Images) GetImageVersion(op trace.Operation, img string) (string, error) {
	defer trace.End(trace.Begin("", op))

	f, err := os.Open(img)
	if err != nil {
		return "", errors.Errorf("failed to open iso file %q: %s", img, err)
	}
	defer f.Close()

	// System area goes from sectors 0x00 to 0x0F. Volume descriptors can be
	// found starting at sector 0x10

	_, err = f.Seek(int64(ISOVolumeSector*ISO9660SectorSize)+PublisherOffset, 0)
	if err != nil {
		return "", errors.Errorf("failed to locate iso version section in file %q: %s", img, err)
	}
	publisherBytes := make([]byte, 128)
	size, err := f.Read(publisherBytes)
	if err != nil {
		return "", errors.Errorf("failed to read iso version in file %q: %s", img, err)
	}
	if size == 0 {
		return "", errors.Errorf("version is not set in iso file %q", img)
	}

	versions := strings.Fields(string(publisherBytes[:size]))
	if len(versions) > 0 {
		return versions[len(versions)-1], nil
	}
	return "version-unknown", nil
}

func (i *Images) checkImageVersion(op trace.Operation, img string, force bool) (string, error) {
	defer trace.End(trace.Begin("", op))

	ver, err := i.GetImageVersion(op, img)
	if err != nil {
		return "", err
	}
	sv := i.getNoCommitHashVersion(op, ver)
	if sv == "" {
		op.Debugf("Version is not set in %q", img)
		ver = ""
	}

	installerSV := i.getNoCommitHashVersion(op, version.GetBuild().ShortVersion())

	// here compare version without last commit hash, to make developer life easier
	if !strings.EqualFold(installerSV, sv) {
		message := fmt.Sprintf("iso file %q version %q inconsistent with installer version %q", img, strings.ToLower(ver), version.GetBuild().ShortVersion())
		if !force {
			return "", errors.Errorf("%s. Specify --force to force create. ", message)
		}
		op.Warn(message)
	}
	return ver, nil
}

func (i *Images) getNoCommitHashVersion(op trace.Operation, version string) string {
	defer trace.End(trace.Begin("", op))

	j := strings.LastIndex(version, "-")
	if j == -1 {
		return ""
	}
	return version[:j]
}
