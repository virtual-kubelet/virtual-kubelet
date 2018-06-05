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

package util

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/trace"
)

const (
	// XXX leaving this as http for now.  We probably want to make this unix://
	scheme = "http://"
)

var (
	DefaultHost           = Host()
	nameTemplateOnce      sync.Once
	nameTemplateInitial   string
	nameTemplate          string
	nameAvailableCapacity int
)

func Host() *url.URL {
	name, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	thisHost, err := url.Parse(scheme + name)
	if err != nil {
		log.Fatal(err)
	}

	return thisHost
}

// ServiceURL returns the URL for a given service relative to this host
func ServiceURL(serviceName string) *url.URL {
	s, err := DefaultHost.Parse(serviceName)
	if err != nil {
		log.Fatal(err)
	}

	return s
}

// prepTemplate takes a template string, determines it's suitability for use and adjusts it if necessary
// returns:
//   adjusted template
//   available length for insertions
func prepTemplate(op trace.Operation, template string) (string, int) {
	if template == "" {
		template = config.DefaultNamePattern
	}

	withoutName := strings.Replace(template, config.NameToken.String(), "", 1)
	withoutEither := strings.Replace(withoutName, config.IDToken.String(), "", 1)
	availableLen := constants.MaxVMNameLength - len(withoutEither)

	// TODO: initialization time check that template actually contains a token or we have a static string

	// TODO: initialization time check that template is of usable length

	// if there's zero space for replacement then there is no room for parameterization to avoid collisons
	if availableLen > 0 {
		return template, availableLen
	}

	op.Error("Falling back to default name convention as custom convention overflows name capacity")

	template, availableLen = prepTemplate(op, config.DefaultNamePattern)
	if availableLen > 0 {
		return template, availableLen
	}

	// return sane fallback - has bounded length and probably few collisions
	op.Error("Falling back to raw ID name convention as default convention overflows name capacity")
	return config.IDToken.String(), constants.MaxVMNameLength
}

// cachedPrepTemplate provides basic single value memoization caching wrapping prepTemplate
func cachedPrepTemplate(op trace.Operation, template string) (string, int) {
	// cache these values for the first template
	// can move to full memoization if/when we allow dynamic choice or dynamic config
	nameTemplateOnce.Do(func() {
		nameTemplateInitial = template
		nameTemplate, nameAvailableCapacity = prepTemplate(op, template)
		op.Infof("Cached processed name convention template %q with insertion capacity of %d", nameTemplate, nameAvailableCapacity)
	})

	if template == nameTemplateInitial {
		return nameTemplate, nameAvailableCapacity
	}

	return prepTemplate(op, template)
}

// replaceToken will replace the first occurrence only of the specific PatternToken in the template.
// Returns:
//	 updated template with value inserted
//   renaming available capacity for insertions
func replaceToken(template string, token config.PatternToken, value string, availableLen int) (string, int) {
	if strings.Contains(template, token.String()) {
		trunc := value
		if len(value) > availableLen {
			trunc = value[:availableLen]
		}

		return strings.Replace(template, token.String(), trunc, 1), availableLen - len(trunc)
	}

	return template, availableLen
}

// Update the VM display name on vSphere UI
func DisplayName(op trace.Operation, cfg *spec.VirtualMachineConfigSpecConfig, namingConvention string) string {
	shortID := cfg.ID[:constants.ShortIDLen]
	prettyName := cfg.Name

	// determine length of template without tokens
	name, availableLen := cachedPrepTemplate(op, namingConvention)
	name, availableLen = replaceToken(name, config.IDToken, shortID, availableLen)
	name, availableLen = replaceToken(name, config.NameToken, prettyName, availableLen)

	op.Infof("Applied naming convention: %s resulting %s", namingConvention, name)
	return name
}

func ClientIP() (net.IP, error) {
	ips, err := net.LookupIP(constants.ClientHostName)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("No IP found on %s", constants.ClientHostName)
	}

	if len(ips) > 1 {
		return nil, fmt.Errorf("Multiple IPs found on %s: %#v", constants.ClientHostName, ips)
	}
	return ips[0], nil
}
