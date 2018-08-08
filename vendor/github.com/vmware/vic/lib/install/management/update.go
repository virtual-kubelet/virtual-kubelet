// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package management

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/trace"
)

type State int

// ruleset ID from /etc/vmware/firewall/service.xml
const RulesetID string = "vSPC"
const (
	enable State = iota
	disable
)

func (s State) String() string {
	switch s {
	case enable:
		return "enable"
	case disable:
		return "disable"
	}
	return ""
}

// EnableFirewallRuleset enables the ruleset on the target, allowing VIC backchannel traffic
func (d *Dispatcher) EnableFirewallRuleset() error {
	defer trace.End(trace.Begin("", d.op))
	return d.modifyFirewall(enable)
}

// DisableFirewallRuleset disables the ruleset on the target, denying VIC backchannel traffic
func (d *Dispatcher) DisableFirewallRuleset() error {
	defer trace.End(trace.Begin("", d.op))
	return d.modifyFirewall(disable)
}

// modifyFirewall sets the state of the firewall ruleset specified by RulesetID
func (d *Dispatcher) modifyFirewall(state State) error {
	defer trace.End(trace.Begin("", d.op))
	var err error
	var hosts []*object.HostSystem

	d.op.Debugf("cluster: %s", d.session.Cluster)

	hosts, err = d.session.Cluster.Hosts(d.op)
	if err != nil {
		return err
	}

	if len(hosts) == 0 {
		d.op.Infof("No hosts to modify")
		return nil
	}

	for _, host := range hosts {
		fs, err := host.ConfigManager().FirewallSystem(d.op)
		if err != nil {
			d.op.Errorf("Failed to get firewall system for host %q: %s", host.Name(), err)
			return err
		}

		switch state {
		case enable:
			err = fs.EnableRuleset(d.op, RulesetID)
		case disable:
			err = fs.DisableRuleset(d.op, RulesetID)
		}
		if err != nil {
			d.op.Errorf("Failed to %s ruleset %q on host %q: %s", state.String(), RulesetID, host.Name(), err)
			return err
		}
		d.op.Infof("Ruleset %q %sd on host %q", RulesetID, state.String(), host)
	}
	return nil
}
