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

package common

import (
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/urfave/cli.v1"
)

const (
	dsInputFormat  = "<datastore url w/ path>:label"
	nfsInputFormat = "nfs://<host>/<url-path>?<mount option as query parameters>:<label>"
)

type VolumeStores struct {
	VolumeStores cli.StringSlice `arg:"volume-store"`
}

func (v *VolumeStores) Flags() []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "volume-store, vs",
			Value: &v.VolumeStores,
			Usage: "Specify a list of location and label for volume store, nfs stores can have mount options specified as query parameters in the url target. \n\t Examples for a vsphere backed volume store are:  \"datastore/path:label\" or \"datastore:label\" or \"ds://my-datastore-name:store-label\"\n\t Examples for nfs back volume stores are: \"nfs://127.0.0.1/path/to/share/point?uid=1234&gid=5678&proto=tcp:my-volume-store-label\" or \"nfs://my-store/path/to/share/point:my-label\"",
		},
	}
}

func (v *VolumeStores) ProcessVolumeStores() (map[string]*url.URL, error) {
	volumeLocations := make(map[string]*url.URL)

	for _, arg := range v.VolumeStores {
		urlTarget, rawTarget, label, err := processVolumeStoreParam(arg)
		if err != nil {
			return nil, err
		}

		switch urlTarget.Scheme {
		case NfsScheme:
			// nothing needs to be done here. parsing the url is enough for pre-validation checking of an nfs target.
		case EmptyScheme, DsScheme:
			// a datastore target is our default assumption
			urlTarget.Scheme = DsScheme
			if err := CheckUnsupportedCharsDatastore(rawTarget); err != nil {
				return nil, fmt.Errorf("--volume-store contains unsupported characters for datastore target: %s Allowed characters are alphanumeric, space and symbols - _ ( ) / : ,", err)
			}

			if len(urlTarget.RawQuery) > 0 {
				return nil, fmt.Errorf("volume store input must be in format datastore/path:label or %s", nfsInputFormat)
			}

		default:
			return nil, fmt.Errorf("%s", "Please specify a datastore or nfs target. See -vs usage for examples.")
		}

		volumeLocations[label] = urlTarget
	}

	return volumeLocations, nil
}

// processVolumeStoreParam will pull apart the raw input for -vs and return the parts for the actual store that are needed for validation
func processVolumeStoreParam(rawVolumeStore string) (*url.URL, string, string, error) {
	errVolStoreFormat := fmt.Errorf("volume store input must be in format %s or %s", dsInputFormat, nfsInputFormat)
	splitMeta := strings.Split(rawVolumeStore, ":")
	if len(splitMeta) < 2 {
		return nil, "", "", errVolStoreFormat
	}

	// divide out the label with the target
	lastIndex := len(splitMeta)
	label := splitMeta[lastIndex-1]
	rawTarget := strings.Join(splitMeta[0:lastIndex-1], ":")
	if label == "" || rawTarget == "" {
		return nil, "", "", errVolStoreFormat
	}

	// This case will check if part of the url is assigned as the label (e.g. ds://No.label.target/some/path)
	if err := CheckUnsupportedChars(label); err != nil {
		return nil, "", "", errVolStoreFormat
	}

	// raw target input should be in the form of a url
	stripRawTarget := rawTarget

	if strings.HasPrefix(stripRawTarget, DsScheme+"://") {
		stripRawTarget = strings.Replace(rawTarget, DsScheme+"://", "", -1)
	}

	urlTarget, err := url.Parse(stripRawTarget)
	if err != nil {
		return nil, "", "", fmt.Errorf("parsed url for option --volume-store could not be parsed as a url, valid inputs are datastore/path:label or %s. See -h for usage examples.", nfsInputFormat)
	}

	return urlTarget, rawTarget, label, nil
}
