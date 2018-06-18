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

package etcconf

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsumeEntry(t *testing.T) {
	r := NewResolvConf("")

	assert.Equal(t, r.Timeout(), DefaultTimeout)

	c := r.(EntryConsumer)
	var tests = []struct {
		in          string
		nameservers []net.IP
		timeout     time.Duration
		attempts    uint
	}{
		{"options", nil, DefaultTimeout, DefaultAttempts},
		{"nameserver", nil, DefaultTimeout, DefaultAttempts},
		{"options timeout", nil, DefaultTimeout, DefaultAttempts},
		{"options attempts", nil, DefaultTimeout, DefaultAttempts},
		{"options timeout:", nil, DefaultTimeout, DefaultAttempts},
		{"options attempts:", nil, DefaultTimeout, DefaultAttempts},
		{"options foo:1", nil, DefaultTimeout, DefaultAttempts},
		{"options timeout:10", nil, 10 * time.Second, DefaultAttempts},
		{"options attempts:5", nil, 10 * time.Second, 5},
		{"nameserver 10.10.10", nil, 10 * time.Second, 5},
		{"nameserver 10.10.10.10", []net.IP{net.ParseIP("10.10.10.10")}, 10 * time.Second, 5},
	}

	for _, te := range tests {
		c.ConsumeEntry(te.in)
		assert.EqualValues(t, te.nameservers, r.Nameservers())
		assert.Equal(t, te.timeout, r.Timeout())
		assert.Equal(t, te.attempts, r.Attempts())
	}
}
