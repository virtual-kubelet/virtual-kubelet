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
	"encoding/base64"
	"encoding/json"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

const (
	AnnotationKeyLabels     = "docker.labels"
	AnnotationKeyAutoRemove = "docker.autoremove"
)

// SetContainerAnnotation encodes a docker specific attribute into a vSphere annotation.  These vSphere
// annotations are stored in the VM vmx file
func SetContainerAnnotation(config *models.ContainerCreateConfig, key string, value interface{}) error {
	var err error

	if config == nil || value == nil {
		return nil
	}

	if config.Annotations == nil {
		config.Annotations = make(map[string]string)
	}

	// Encoding the labels map into a blob that can be stored as ansi regardless
	// of what encoding the input labels are.  We do this by first marshaling to
	// to a json byte array to get a self describing encoding and then encoding
	// to base64.  We could use another encoding for the self describing part,
	// such as Golang GOB, but this data will be pushed over to a standard REST
	// server so we use standard web standards instead.
	if valueBytes, merr := json.Marshal(value); merr == nil {
		blob := base64.StdEncoding.EncodeToString(valueBytes)
		config.Annotations[key] = blob
	} else {
		err = merr
		log.Errorf("Unable to marshal annotation %s to json: %s", key, err)
	}

	return err
}

// ContainerAnnotation will convert a vSphere annotation into a docker specific attribute
func ContainerAnnotation(annotations map[string]string, key string, value interface{}) error {
	var err error

	if len(annotations) == 0 || value == nil {
		return nil
	}

	if blob, ok := annotations[key]; ok {
		if annotationBytes, decodeErr := base64.StdEncoding.DecodeString(blob); decodeErr == nil {
			if err = json.Unmarshal(annotationBytes, value); err != nil {
				log.Errorf("Unable to unmarshal %s: %s", key, err)
			}
		} else {
			err = decodeErr
			log.Errorf("Unable to decode container annotations: %s", err)
		}
	}

	return err
}
