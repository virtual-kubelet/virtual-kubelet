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

package placement

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/test"
)

func TestRandomRecommendHost(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestRandomRecommendHost")

	model, server, sess := test.VpxModelSetup(op, t)
	defer func() {
		model.Remove()
		server.Close()
	}()

	cls := sess.Cluster

	hosts, err := cls.Hosts(op)
	assert.NoError(t, err)

	rhp, err := NewRandomHostPolicy(op, cls)
	assert.NoError(t, err)

	assert.False(t, rhp.CheckHost(op, nil))
	h, err := rhp.RecommendHost(op, nil)
	assert.NoError(t, err)

	top := h[0].Reference().String()
	found := false
	for _, host := range hosts {
		if h[0].Reference().String() == host.Reference().String() {
			found = true

			// remove this host for the next test
			h = h[1:]
			break
		}
	}
	assert.True(t, found)

	// try with a subset
	x, err := rhp.RecommendHost(op, h)
	assert.NoError(t, err)
	assert.Len(t, x, len(hosts)-1)
	for _, host := range x {
		assert.NotEqual(t, top, host.Reference().String())
	}
}
