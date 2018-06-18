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
	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"

	apiclient "github.com/vmware/vic/lib/apiservers/portlayer/client"
)

func NewPortLayerClient(portLayerAddr string) *apiclient.PortLayer {
	t := rc.New(portLayerAddr, "/", []string{"http"})
	t.Consumers["application/x-tar"] = runtime.ByteStreamConsumer()
	t.Consumers["application/octet-stream"] = runtime.ByteStreamConsumer()
	t.Producers["application/x-tar"] = runtime.ByteStreamProducer()
	t.Producers["application/octet-stream"] = runtime.ByteStreamProducer()

	portLayerClient := apiclient.New(t, nil)

	return portLayerClient
}
