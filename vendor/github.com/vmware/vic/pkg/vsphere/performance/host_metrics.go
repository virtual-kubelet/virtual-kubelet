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

package performance

import (
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
)

const (
	// cpuUsage measures the actively used CPU of the host, as a percentage of the total available CPU.
	cpuUsage = "cpu.usage.average"

	// memActive measures the sum of all active metrics for all powered-on VMs plus vSphere services on the host.
	memActive = "mem.active.average"

	// memConsumed measures the amount of machine memory used on the host, including vSphere services, VMkernel,
	// the service console and the total consumed memory metrics for all running VMs.
	memConsumed = "mem.consumed.average"

	// memTotalCapacity measures the total amount of memory reservation used by and available for powered-on VMs
	// and vSphere services on the host.
	memTotalCapacity = "mem.totalCapacity.average"

	// memOverhead measures the total of all overhead metrics for powered-on VMs, plus the overhead of running
	// vSphere services on the host.
	memOverhead = "mem.overhead.average"
)

// HostMemory stores an ESXi host's memory metrics.
type HostMemory struct {
	ActiveKB   int64
	ConsumedKB int64
	OverheadKB int64
	TotalKB    int64
}

// HostCPU stores an ESXi host's CPU metrics.
type HostCPU struct {
	UsagePercent float64
}

// HostMetricsInfo stores an ESXi host's memory and CPU metrics.
type HostMetricsInfo struct {
	Memory HostMemory
	CPU    HostCPU
}

// HostMetricsProvider returns CPU and memory metrics for all ESXi hosts in a cluster
// via implementation of the MetricsProvider interface.
type HostMetricsProvider struct {
	*vim25.Client
}

// NewHostMetricsProvider returns a new instance of HostMetricsProvider.
func NewHostMetricsProvider(c *vim25.Client) *HostMetricsProvider {
	return &HostMetricsProvider{Client: c}
}

// GetMetricsForComputeResource gathers host metrics from the supplied compute resource.
// Returned map is keyed on the host ManagedObjectReference in string form.
func (h *HostMetricsProvider) GetMetricsForComputeResource(op trace.Operation, cr *object.ComputeResource) (map[string]*HostMetricsInfo, error) {
	// Gather hosts from the session cluster and then obtain their morefs.
	hosts, err := cr.Hosts(op)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain host morefs from compute resource: %s", err)
	}

	if hosts == nil {
		return nil, fmt.Errorf("no hosts found in compute resource")
	}

	return h.GetMetricsForHosts(op, hosts)
}

// GetMetricsForHosts returns metrics pertaining to supplied ESX hosts.
// Returned map is keyed on the host ManagedObjectReference in string form.
func (h *HostMetricsProvider) GetMetricsForHosts(op trace.Operation, hosts []*object.HostSystem) (map[string]*HostMetricsInfo, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts provided")
	}

	if h.Client == nil {
		return nil, fmt.Errorf("client not set")
	}

	// filter out hosts that are in maintenance mode or disconnected
	hosts, err := filterHosts(op, h.Client, hosts)
	if err != nil {
		return nil, err
	}

	morefToHost := make(map[string]*object.HostSystem)
	morefs := make([]types.ManagedObjectReference, len(hosts))
	for i, host := range hosts {
		moref := host.Reference()
		morefToHost[moref.String()] = host
		morefs[i] = moref
	}

	// Query CPU and memory metrics for the morefs.
	spec := types.PerfQuerySpec{
		Format:     string(types.PerfFormatNormal),
		IntervalId: sampleInterval,
	}

	counters := []string{cpuUsage, memActive, memConsumed, memTotalCapacity, memOverhead}
	perfMgr := performance.NewManager(h.Client)
	sample, err := perfMgr.SampleByName(op.Context, spec, counters, morefs)
	if err != nil {
		errStr := "unable to get metric sample: %s"
		op.Errorf(errStr, err)
		return nil, fmt.Errorf(errStr, err)
	}

	results, err := perfMgr.ToMetricSeries(op.Context, sample)
	if err != nil {
		errStr := "unable to convert metric sample to metric series: %s"
		op.Errorf(errStr, err)
		return nil, fmt.Errorf(errStr, err)
	}

	metrics := assembleMetrics(op, morefToHost, results)
	return metrics, nil
}

// assembleMetrics processes the metric samples received from govmomi and returns a finalized metrics map
// keyed by the hosts.
func assembleMetrics(op trace.Operation, morefToHost map[string]*object.HostSystem,
	results []performance.EntityMetric) map[string]*HostMetricsInfo {
	metrics := make(map[string]*HostMetricsInfo)

	for _, host := range morefToHost {
		metrics[host.Reference().String()] = &HostMetricsInfo{}
	}

	for i := range results {
		res := results[i]
		host, exists := morefToHost[res.Entity.String()]
		if !exists {
			op.Warnf("moref %s does not exist in requested morefs, skipping", res.Entity.String())
			continue
		}

		ref := host.Reference().String()
		// Process each value and assign it directly to the corresponding metric field
		// since there is only one sample.
		for _, v := range res.Value {

			// We don't need to collect non-aggregate (non-empty Instance) metrics.
			if v.Instance != "" {
				continue
			}

			if len(v.Value) == 0 {
				op.Warnf("metric %s for moref %s has no value, skipping", v.Name, res.Entity.String())
				continue
			}

			switch v.Name {
			case cpuUsage:
				// Convert percent units from 1/100th of a percent (100 = 1%) to a human-readable percentage.
				metrics[ref].CPU.UsagePercent = float64(v.Value[0]) / 100.0
			case memActive:
				metrics[ref].Memory.ActiveKB = v.Value[0]
			case memConsumed:
				metrics[ref].Memory.ConsumedKB = v.Value[0]
			case memOverhead:
				metrics[ref].Memory.OverheadKB = v.Value[0]
			case memTotalCapacity:
				// Total capacity is in MB, convert to KB so as to have all memory values in KB.
				metrics[ref].Memory.TotalKB = v.Value[0] * 1024
			}
		}
	}

	return metrics
}

// filterHosts removes candidate hosts who are either disconnected or in maintenance mode.
func filterHosts(op trace.Operation, client *vim25.Client, hosts []*object.HostSystem) ([]*object.HostSystem, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no candidate hosts to filter check")
	}

	props := []string{"summary.runtime"}
	refs := make([]types.ManagedObjectReference, 0, len(hosts))
	for _, h := range hosts {
		refs = append(refs, h.Reference())
	}

	hs := make([]mo.HostSystem, 0, len(hosts))
	pc := property.DefaultCollector(client)
	err := pc.Retrieve(op, refs, props, &hs)
	if err != nil {
		return nil, err
	}

	result := hosts[:0]
	for i, h := range hs {
		if h.Summary.Runtime.ConnectionState == types.HostSystemConnectionStateConnected && !h.Summary.Runtime.InMaintenanceMode {
			result = append(result, hosts[i])
		}
	}

	return result, nil
}
