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

package encode

import (
	"net"
	"strings"

	"github.com/go-openapi/strfmt"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/ip"
)

func AsIPAddress(address net.IP) models.IPAddress {
	return models.IPAddress(address.String())
}

func AsIPAddresses(addresses *[]net.IP) *[]models.IPAddress {
	m := make([]models.IPAddress, 0, len(*addresses))
	for _, value := range *addresses {
		m = append(m, AsIPAddress(value))
	}

	return &m
}

func AsCIDR(network *net.IPNet) models.CIDR {
	if network == nil {
		return models.CIDR("")
	}

	return models.CIDR(network.String())
}

func AsCIDRs(networks *[]net.IPNet) *[]models.CIDR {
	m := make([]models.CIDR, 0, len(*networks))
	for _, value := range *networks {
		m = append(m, AsCIDR(&value))
	}

	return &m
}

func AsIPRange(network *net.IPNet) models.IPRange {
	if network == nil {
		return models.IPRange("")
	}

	return models.IPRange(models.CIDR(network.String()))
}

func AsIPRanges(networks *[]ip.Range) *[]models.IPRange {
	m := make([]models.IPRange, 0, len(*networks))
	for _, value := range *networks {
		m = append(m, AsIPRange(value.Network()))
	}

	return &m
}

func AsNetwork(network *executor.NetworkEndpoint) *models.Network {
	if network == nil {
		return nil
	}

	m := &models.Network{
		PortGroup: &models.ManagedObject{
			ID: AsManagedObjectID(network.Network.Common.ID),
		},
		Nameservers: *AsIPAddresses(&network.Network.Nameservers),
	}

	if network.Network.Gateway.IP != nil {
		m.Gateway = &models.Gateway{
			Address:             AsIPAddress(network.Network.Gateway.IP),
			RoutingDestinations: *AsCIDRs(&network.Network.Destinations),
		}
	}

	return m
}

func AsImageFetchProxy(sessionConfig *executor.SessionConfig, http, https string) *models.VCHRegistryImageFetchProxy {
	var httpProxy, httpsProxy strfmt.URI
	for _, env := range sessionConfig.Cmd.Env {
		if strings.HasPrefix(env, http+"=") {
			httpProxy = strfmt.URI(strings.SplitN(env, "=", 2)[1])
		}
		if strings.HasPrefix(env, https+"=") {
			httpsProxy = strfmt.URI(strings.SplitN(env, "=", 2)[1])
		}
	}

	if httpProxy == "" && httpsProxy == "" {
		return nil
	}

	return &models.VCHRegistryImageFetchProxy{HTTP: httpProxy, HTTPS: httpsProxy}
}
