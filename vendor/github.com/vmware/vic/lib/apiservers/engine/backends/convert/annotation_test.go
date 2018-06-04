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

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

func TestSetContainerAnnotation(t *testing.T) {

	config := &models.ContainerCreateConfig{}
	labels := make(map[string]string)
	labels["environment"] = "dev"

	err := SetContainerAnnotation(nil, AnnotationKeyLabels, labels)
	assert.NoError(t, err)

	err = SetContainerAnnotation(config, AnnotationKeyLabels, &labels)
	assert.NoError(t, err)

	var myLabels map[string]string

	err = ContainerAnnotation(myLabels, AnnotationKeyLabels, &myLabels)
	assert.NoError(t, err)

	err = ContainerAnnotation(config.Annotations, AnnotationKeyLabels, &myLabels)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(myLabels))

	err = ContainerAnnotation(config.Annotations, AnnotationKeyLabels, myLabels)
	assert.Error(t, err)
}
