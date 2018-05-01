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

package backends

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	v1 "github.com/docker/docker/image"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/metadata"
)

func TestConvertV1ImageToDockerImage(t *testing.T) {
	now := time.Now()

	image := &metadata.ImageConfig{
		V1Image: v1.V1Image{
			ID:      "deadbeef",
			Size:    1024,
			Created: now,
			Parent:  "",
			Config: &container.Config{
				Labels: map[string]string{},
			},
		},
		ImageID:   "test_id",
		Digests:   []string{fmt.Sprintf("%s@sha:%s", "test_name", "12345")},
		Tags:      []string{fmt.Sprintf("%s:%s", "test_name", "test_tag")},
		Name:      "test_name",
		DiffIDs:   map[string]string{"test_diffid": "test_layerid"},
		History:   []v1.History{},
		Reference: "test_name:test_tag",
	}

	dockerImage := convertV1ImageToDockerImage(image)

	assert.Equal(t, image.ImageID, dockerImage.ID, "Error: expected id %s, got %s", image.ImageID, dockerImage.ID)
	assert.Equal(t, image.Size, dockerImage.VirtualSize, "Error: expected size %s, got %s", image.Size, dockerImage.VirtualSize)
	assert.Equal(t, image.Size, dockerImage.Size, "Error: expected size %s, got %s", image.Size, dockerImage.Size)
	assert.Equal(t, image.Created.Unix(), dockerImage.Created, "Error: expected created %s, got %s", image.Created, dockerImage.Created)
	assert.Equal(t, image.Parent, dockerImage.ParentID, "Error: expected parent %s, got %s", image.Parent, dockerImage.ParentID)
	assert.Equal(t, image.Config.Labels, dockerImage.Labels, "Error: expected labels %s, got %s", image.Config.Labels, dockerImage.Labels)
	assert.Equal(t, image.Digests[0], dockerImage.RepoDigests[0], "Error: expected digest %s, got %s", image.Digests[0], dockerImage.RepoDigests[0])
	assert.Equal(t, image.Tags[0], dockerImage.RepoTags[0], "Error: expected tag %s, got %s", image.Tags[0], dockerImage.RepoTags[0])
}
