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

package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
)

type ImageStore interface {
	Get(op trace.Operation, idOrRef, tag string, actuate bool) (*metadata.ImageConfig, error)
	GetImages(op trace.Operation) []*metadata.ImageConfig
	PullImage(op trace.Operation, image, tag, username, password string) error
}

type VicImageStore struct {
	client        *client.PortLayer
	personaAddr   string
	portlayerAddr string
}

func NewImageStore(plClient *client.PortLayer, personaAddr, portlayerAddr string) (ImageStore, error) {
	err := cache.InitializeImageCache(plClient)
	if err != nil {
		return nil, err
	}

	vs := &VicImageStore{
		client:        plClient,
		personaAddr:   personaAddr,
		portlayerAddr: portlayerAddr,
	}

	return vs, nil
}

// Get returns an ImageConfig.  If the config is not cached, VicImageStore can request
// imagec to pull the image if actuate is set to true.
func (v *VicImageStore) Get(op trace.Operation, idOrRef, tag string, realize bool) (*metadata.ImageConfig, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("Get - %s:%s", idOrRef, tag), op))

	c, err := cache.ImageCache().Get(idOrRef)
	if err != nil && realize {
		err = v.PullImage(op, idOrRef, tag, "", "")
		if err == nil {
			//TODO:  Find a better way to get update imageconfig instead of this hammer
			err := cache.InitializeImageCache(v.client)
			if err != nil {
				return nil, err
			}
			c, err = cache.ImageCache().Get(idOrRef)
			if err != nil {
				return nil, err
			}
		}
	}

	return c, nil
}

func (v *VicImageStore) GetImages(op trace.Operation) []*metadata.ImageConfig {
	defer trace.End(trace.Begin("", op))

	return cache.ImageCache().GetImages()
}

// PullImage pulls images using the docker persona.  It simply issues a pull rest call to the persona.
// This lets the persona be the imagec server and keeps both the kubelet and docker persona up to date
// when the kubelet pulls an image.
func (v *VicImageStore) PullImage(op trace.Operation, image, tag, username, password string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Get - %s:%s", image, tag), op))

	pullClient := &http.Client{Timeout: 60 * time.Second}
	var personaServer string
	if tag == "" {
		personaServer = fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s", v.personaAddr, image)
	} else {
		personaServer = fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s&tag=%s", v.personaAddr, image, tag)
	}
	op.Infof("POST %s", personaServer)
	reader := bytes.NewBuffer([]byte(""))
	resp, err := pullClient.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from docker pull: error = %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error from docker pull: status = %d", resp.StatusCode)
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading docker pull response: error = %s", err.Error())
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}
	op.Infof("Response from docker pull: body = %s", string(body))

	return nil
}
