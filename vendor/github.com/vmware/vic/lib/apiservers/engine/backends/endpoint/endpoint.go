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

package endpoint

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	apinet "github.com/docker/docker/api/types/network"
)

func Alias(endpointConfig *apinet.EndpointSettings) []string {
	var aliases []string

	log.Debugf("EndpointsConfig: %#v", endpointConfig)
	log.Debugf("Aliases: %s", endpointConfig.Aliases)
	log.Debugf("Links: %s", endpointConfig.Links)

	// Links are already in CONTAINERNAME:ALIAS format
	aliases = endpointConfig.Links
	// Converts aliases to ":ALIAS" format
	for i := range endpointConfig.Aliases {
		aliases = append(aliases, fmt.Sprintf(":%s", endpointConfig.Aliases[i]))
	}
	return aliases
}
