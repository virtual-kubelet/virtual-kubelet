// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package volume

import (
	"net/url"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/portlayer/util"
)

func TestVolumeParseURL(t *testing.T) {
	util.DefaultHost, _ = url.Parse("http://foo.com/")

	in, _ := url.Parse(util.DefaultHost.String())
	in.Path = "/" + path.Join("storage", "volumes", "volStore", "volName")

	v := &Volume{}
	err := v.Parse(in)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, "volName", v.ID) {
		return
	}

	volStore, _ := url.Parse(util.DefaultHost.String())
	util.AppendDir(volStore, "/storage/volumes/volStore")
	if !assert.Equal(t, volStore.String(), v.Store.String()) {
		return
	}

	util.AppendDir(volStore, "volName")
	if !assert.Equal(t, volStore.String(), v.SelfLink.String()) {
		return
	}

}
