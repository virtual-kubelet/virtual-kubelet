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

package cache

import (
	"testing"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/uid"

	"github.com/docker/docker/reference"
	"github.com/stretchr/testify/assert"
)

func repoSetup() {
	rCache = &repoCache{
		client:              &client.PortLayer{},
		Repositories:        make(map[string]repository),
		Layers:              make(map[string]string),
		images:              make(map[string]string),
		referencesByIDCache: make(map[string]map[string]reference.Named),
	}
}

func TestRepo(t *testing.T) {
	repoSetup()

	notInRepo, _ := reference.ParseNamed("alpine")
	noImageID := uid.New()
	ref, _ := reference.ParseNamed("busybox:1.25.1")
	imageID := uid.New()
	layerID := uid.New()

	// add busybox:1.25.1
	err := RepositoryCache().AddReference(ref, imageID.String(), false, layerID.String(), false)
	assert.NoError(t, err)

	// Get will return the imageID for the named object
	n, err := RepositoryCache().Get(ref)
	assert.NoError(t, err)
	assert.Equal(t, imageID.String(), n)

	// Get all references
	refs := RepositoryCache().References(imageID.String())
	assert.Equal(t, 1, len(refs))

	// Get reference by Named
	associated := RepositoryCache().ReferencesByName(ref)
	assert.Equal(t, 1, len(associated))

	// Get tags for image
	tags := RepositoryCache().Tags(imageID.String())
	assert.Equal(t, 1, len(tags))

	// Get references for non-existent image
	refs = RepositoryCache().References(noImageID.String())
	assert.Equal(t, 0, len(refs))

	// Get reference by Named
	associated = RepositoryCache().ReferencesByName(notInRepo)
	assert.Equal(t, 0, len(associated))

	// get image id via layer id
	ig := RepositoryCache().GetImageID(layerID.String())
	assert.Equal(t, imageID.String(), ig)

	// remove busybox from the cache
	r, err := RepositoryCache().Remove(ref.String(), false)
	assert.NoError(t, err)
	assert.Equal(t, ref.String(), r)

	// busybox is removed, so this should fail
	x, err := RepositoryCache().Remove(ref.String(), false)
	assert.Error(t, err)
	assert.Equal(t, "", x)

	// add reference by digest
	ng, _ := reference.ParseNamed("nginx@sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64")
	err = RepositoryCache().AddReference(ng, imageID.String(), true, layerID.String(), false)
	assert.NoError(t, err)

	dd := RepositoryCache().Digests(imageID.String())
	assert.Equal(t, 1, len(dd))
	// remove the digest
	ngx, err := RepositoryCache().Remove(ng.String(), false)
	assert.NoError(t, err)
	assert.Equal(t, ng.String(), ngx)
	// nada
	nada := RepositoryCache().Digests(imageID.String())
	assert.Equal(t, 0, len(nada))

}
