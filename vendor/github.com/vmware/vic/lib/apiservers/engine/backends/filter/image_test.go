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

package filter

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/reference"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/metadata"
)

func loadImageCache(repo string, imageCount int, t *testing.T) {

	for i := 0; i < imageCount; i++ {
		id := fmt.Sprintf("120%d", i)
		tag := fmt.Sprintf("1.0%d", i+1)
		ref := fmt.Sprintf("%s:%s", repo, tag)
		img := &metadata.ImageConfig{
			ImageID:   id,
			Tags:      []string{tag},
			Name:      repo,
			Reference: ref,
		}
		img.Created = time.Now().UTC()
		img.ID = id
		img.Config = &container.Config{}
		cache.ImageCache().Add(img)

		named, err := reference.ParseNamed(ref)
		if err != nil {
			t.Fatalf("Error while parsing reference %s: %#v", ref, err)
		}
		cache.RepositoryCache().AddReference(named, id, true, id, false)
	}

	assert.Equal(t, imageCount, len(cache.ImageCache().GetImages()))
}

func TestValidateImageFilters(t *testing.T) {
	loadImageCache("busyboxy", 5, t)

	cmdFilters := filters.NewArgs()
	cmdFilters.Add("dangling", "true")
	_, err := ValidateImageFilters(cmdFilters, acceptedImageFilterTags, unSupportedImageFilters)
	assert.Error(t, err)

	cmdFilters.Del("dangling", "true")
	cmdFilters.Add("before", "1200")
	_, err = ValidateImageFilters(cmdFilters, acceptedImageFilterTags, unSupportedImageFilters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such image")

	cmdFilters.Del("before", "1200")
	cmdFilters.Add("since", "1200")
	_, err = ValidateImageFilters(cmdFilters, acceptedImageFilterTags, unSupportedImageFilters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such image")
}

func TestIncludeImage(t *testing.T) {
	cmdFilters := filters.NewArgs()

	cmdFilters.Add("before", "busyboxy:1.03")
	imageContext, err := ValidateImageFilters(cmdFilters, acceptedImageFilterTags, unSupportedImageFilters)
	assert.NoError(t, err)
	assert.Equal(t, "1202", *imageContext.BeforeID)

	imageContext.ID = "1202"
	action := IncludeImage(cmdFilters, imageContext)
	assert.Equal(t, ExcludeAction, action)

	imageContext.ID = "1200"
	action = IncludeImage(cmdFilters, imageContext)
	assert.Equal(t, IncludeAction, action)

	cmdFilters.Del("before", "busyboxy:1.03")

	cmdFilters.Add("since", "busyboxy:1.01")
	imageContext, err = ValidateImageFilters(cmdFilters, acceptedImageFilterTags, unSupportedImageFilters)
	assert.NoError(t, err)

	imageContext.ID = "1200"
	action = IncludeImage(cmdFilters, imageContext)
	assert.Equal(t, StopAction, action)

	cmdFilters.Del("since", "busyboxy:1.01")
	cmdFilters.Add("reference", "busy*")
	imageContext.SinceID = nil
	action = IncludeImage(cmdFilters, imageContext)
	assert.Equal(t, IncludeAction, action)

	// remove previous filter and reset tags / digests
	cmdFilters.Del("reference", "busy*")
	imageContext.Tags = []string{}
	imageContext.Digests = []string{}
	cmdFilters.Add("reference", "busyboxy:1.01")

	action = IncludeImage(cmdFilters, imageContext)
	assert.Equal(t, action, IncludeAction)
	assert.EqualValues(t, 1, len(imageContext.Tags))
}
