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

package serial

import "github.com/vmware/vic/pkg/trace"

type RawAddr struct {
	Net  string
	Addr string
}

func (addr RawAddr) Network() string {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return addr.Net
}

func (addr RawAddr) String() string {
	if tracing {

		defer trace.End(trace.Begin(""))
	}
	return addr.Network() + "://" + addr.Addr
}

func NewRawAddr(net string, addr string) *RawAddr {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return &RawAddr{
		Net:  net,
		Addr: addr,
	}
}
