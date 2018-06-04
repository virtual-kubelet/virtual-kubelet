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

package network

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-connections/nat"
)

type Port string

const NilPort = Port("")

// PortFromMapping constructs the full form of a port mapping
// This has been added to help migrate towards consistent data returns for Endpoint structures
func PortFromMapping(mapping nat.PortMapping) Port {
	p := fmt.Sprintf("%s:%s", mapping.Binding.HostPort, string(mapping.Port))
	return Port(p)
}

func ParsePort(p string) (Port, error) {
	if _, err := Port(p).Port(); err != nil {
		return NilPort, err
	}
	proto := Port(p).Proto()
	if proto == "" {
		return NilPort, fmt.Errorf("bad port spec %s", p)
	}

	return Port(p), nil
}

func (p Port) Proto() string {
	parts := strings.Split(string(p), ":")
	proto, _ := nat.SplitProtoPort(parts[len(parts)-1])
	return proto
}

func (p Port) Port() (uint16, error) {
	parts := strings.Split(string(p), ":")
	_, port := nat.SplitProtoPort(parts[len(parts)-1])
	if port == "" {
		return 0, fmt.Errorf("bad port spec %s", p)
	}

	pout, err := strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("bad port spec %s", p)
	}

	return uint16(pout), nil
}

func (p Port) String() string {
	parts := strings.Split(string(p), ":")
	return string(parts[len(parts)-1])
}

func (p Port) FullString() string {
	return string(p)
}
