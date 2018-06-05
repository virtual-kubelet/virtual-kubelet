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

package imagec

import (
	"context"
	"io"

	log "github.com/Sirupsen/logrus"

	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"

	apiclient "github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/misc"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
)

var (
	ctx = context.TODO()
)

// PingPortLayer calls the _ping endpoint of the portlayer
func PingPortLayer(host string) (bool, error) {
	defer trace.End(trace.Begin(host))

	transport := rc.New(host, "/", []string{"http"})
	client := apiclient.New(transport, nil)

	ok, err := client.Misc.Ping(misc.NewPingParamsWithContext(ctx))
	if err != nil {
		return false, err
	}
	return ok.Payload == "OK", nil
}

// ListImages lists the images from given image store
func ListImages(host, storename string, images []*ImageWithMeta) (map[string]*models.Image, error) {
	defer trace.End(trace.Begin(storename))

	transport := rc.New(host, "/", []string{"http"})
	client := apiclient.New(transport, nil)

	ids := make([]string, len(images))

	for i := range images {
		ids = append(ids, images[i].ID)
	}

	imageList, err := client.Storage.ListImages(
		storage.NewListImagesParamsWithContext(ctx).WithStoreName(storename).WithIds(ids),
	)
	if err != nil {
		return nil, err
	}

	existingImages := make(map[string]*models.Image)
	for i := range imageList.Payload {
		v := imageList.Payload[i]
		existingImages[v.ID] = v
	}
	return existingImages, nil
}

// WriteImage writes the image to given image store
func WriteImage(host string, image *ImageWithMeta, data io.ReadCloser) error {
	defer trace.End(trace.Begin(image.ID))

	transport := rc.New(host, "/", []string{"http"})
	client := apiclient.New(transport, nil)

	transport.Consumers["application/json"] = runtime.JSONConsumer()
	transport.Producers["application/json"] = runtime.JSONProducer()
	transport.Consumers["application/octet-stream"] = runtime.ByteStreamConsumer()
	transport.Producers["application/octet-stream"] = runtime.ByteStreamProducer()

	key := new(string)
	blob := new(string)

	*key = metadata.MetaDataKey
	*blob = image.Meta

	r, err := client.Storage.WriteImage(
		storage.NewWriteImageParamsWithContext(ctx).
			WithImageID(image.ID).
			WithParentID(image.Parent).
			WithStoreName(image.Store).
			WithMetadatakey(key).
			WithMetadataval(blob).
			WithImageFile(data).
			WithSum(image.Layer.BlobSum),
	)
	if err != nil {
		log.Debugf("Creating an image failed: %s", err)
		return err
	}
	log.Printf("Created an image %#v", r.Payload)

	return nil

}
