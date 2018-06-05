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

package util

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/vmware/govmomi/object"
)

const (
	StorageURLPath = "storage"
	ImageURLPath   = StorageURLPath + "/images"
	VolumeURLPath  = StorageURLPath + "/volumes"
)

// ImageStoreNameToURL parses the image URL in the form /storage/images/<image store>/<image name>
func ImageStoreNameToURL(storeName string) (*url.URL, error) {
	a := ServiceURL(ImageURLPath)
	AppendDir(a, storeName)
	return a, nil
}

func ImageStoreName(u *url.URL) (string, error) {
	// Check the path isn't malformed.
	if !filepath.IsAbs(u.Path) {
		return "", errors.New("invalid uri path")
	}

	segments := strings.Split(filepath.Clean(u.Path), "/")[1:]

	if len(segments) < 3 ||
		segments[0] != filepath.Clean(StorageURLPath) {
		return "", errors.New("not a storage path")
	}

	if segments[1] != "images" {
		return "", errors.New("not an imagestore path")
	}

	if len(segments) < 2 {
		return "", errors.New("uri path mismatch")
	}

	return segments[2], nil
}

// ImageURL converts a store and image name into an URL that is an internal imagestore representation
// NOTE: this is NOT a datastore URL and cannot be used in any calls that expect a ds:// scheme
func ImageURL(storeName, imageName string) (*url.URL, error) {
	if imageName == "" {
		return nil, fmt.Errorf("image ID missing")
	}

	u, err := ImageStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}
	AppendDir(u, imageName)
	return u, nil
}

// ImageDatastoreURL takes a datastore path object and converts it into a stable URL for with a "ds" scheme
func ImageDatastoreURL(path *object.DatastorePath) *url.URL {
	return &url.URL{
		Scheme: "ds",
		Path:   path.String(),
	}
}

// VolumeStoreNameToURL parses the volume URL in the form /storage/volumes/<volume store>/<volume name>
func VolumeStoreNameToURL(storeName string) (*url.URL, error) {
	a := ServiceURL(VolumeURLPath)
	AppendDir(a, storeName)
	return a, nil
}

func VolumeStoreName(u *url.URL) (string, error) {
	// Check the path isn't malformed.
	if !filepath.IsAbs(u.Path) {
		return "", errors.New("invalid uri path")
	}

	segments := strings.Split(filepath.Clean(u.Path), "/")[1:]

	if len(segments) < 3 ||
		segments[0] != filepath.Clean(StorageURLPath) {
		return "", errors.New("not a storage path")
	}

	if segments[1] != "volumes" {
		return "", errors.New("not an volumestore path")
	}

	if len(segments) < 2 {
		return "", errors.New("uri path mismatch")
	}

	return segments[2], nil
}

func VolumeURL(storeName, volumeName string) (*url.URL, error) {
	u, err := VolumeStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}
	AppendDir(u, volumeName)
	return u, nil
}

func AppendDir(u *url.URL, dir string) {
	u.Path = path.Join(u.Path, dir)
}
