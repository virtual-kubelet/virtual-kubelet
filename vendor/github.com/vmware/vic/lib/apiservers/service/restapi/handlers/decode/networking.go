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

package decode

import (
	"fmt"
	"strings"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/apiservers/service/models"
)

func FromCIDR(m *models.CIDR) string {
	if m == nil {
		return ""
	}

	return string(*m)
}

func FromCIDRs(m *[]models.CIDR) *[]string {
	s := make([]string, 0, len(*m))
	for _, d := range *m {
		s = append(s, FromCIDR(&d))
	}

	return &s
}

func FromIPAddress(m *models.IPAddress) string {
	if m == nil {
		return ""
	}

	return string(*m)
}

func FromIPAddresses(m []models.IPAddress) []string {
	s := make([]string, 0, len(m))
	for _, ip := range m {
		s = append(s, FromIPAddress(&ip))
	}

	return s
}

func FromGateway(m *models.Gateway) string {
	if m == nil {
		return ""
	}

	if m.RoutingDestinations == nil {
		return fmt.Sprintf("%s",
			m.Address,
		)
	}

	return fmt.Sprintf("%s:%s",
		strings.Join(*FromCIDRs(&m.RoutingDestinations), ","),
		m.Address,
	)
}

func FromImageFetchProxy(p *models.VCHRegistryImageFetchProxy) common.Proxies {
	http := string(p.HTTP)
	https := string(p.HTTPS)

	return common.Proxies{
		HTTPProxy:  &http,
		HTTPSProxy: &https,
	}
}
