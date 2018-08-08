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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/test"
)

func TestAssembleMetrics(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestAssembleMetrics")

	model, server, sess := test.VpxModelSetup(op, t)
	defer func() {
		model.Remove()
		server.Close()
	}()

	hosts, err := sess.Cluster.Hosts(op)
	if err != nil {
		t.Fatal(err)
	}

	morefToHost := make(map[string]*object.HostSystem)
	for i := range hosts {
		moref := hosts[i].Reference()
		morefToHost[moref.String()] = hosts[i]
	}

	var results []performance.EntityMetric
	counters := []string{cpuUsage, memActive, memConsumed, memTotalCapacity, memOverhead}

	// Create a fake host and then populate metric results for it. This tests that assembleMetrics
	// rejects and does not assemble metrics for a host whose metrics were not requested
	// (i.e. part of morefToHost) but were still present in the metrics from govmomi.
	fakeHostMoref := types.ManagedObjectReference{
		Type:  "foo",
		Value: "bar",
	}
	fakeHost := object.NewHostSystem(nil, fakeHostMoref)
	hosts = append(hosts, fakeHost)

	// Populate metric results for all hosts, including fakeHost.
	for i := range hosts {
		var values []performance.MetricSeries
		for j := range counters {
			val := performance.MetricSeries{
				Name:  counters[j],
				Value: []int64{int64(i)},
			}
			values = append(values, val)
		}

		results = append(results, performance.EntityMetric{
			Entity: hosts[i].Reference(),
			Value:  values,
		})
	}

	// Insert a non-aggregrate CPU usage value for the first host in metrics results
	// to check that non-aggregrate values aren't processed in assembleMetrics.
	instanceValue := []performance.MetricSeries{{
		Name:     counters[0],
		Instance: "1",
		Value:    []int64{int64(9999)},
	}}
	results = append(results, performance.EntityMetric{
		Entity: hosts[0].Reference(),
		Value:  instanceValue,
	})

	// Insert a metric with no value to test that assembleMetrics skips it.
	emptyValue := []performance.MetricSeries{{
		Name:  counters[0],
		Value: []int64{},
	}}
	results = append(results, performance.EntityMetric{
		Entity: hosts[0].Reference(),
		Value:  emptyValue,
	})

	// Once fakeHost's metrics have been added, remove it from the hosts slice for upcoming checks.
	hosts = hosts[:len(hosts)-1]

	metrics := assembleMetrics(op, morefToHost, results)

	// Assembled metrics should not have an entry for fakeHost.
	assert.Equal(t, len(metrics), len(hosts))
	_, exists := metrics[fakeHost.Reference().String()]
	assert.False(t, exists, "fakeHost %s should not be present in result metrics", fakeHost.String())

	for i, host := range hosts {
		hostMetric, exists := metrics[host.Reference().String()]
		assert.True(t, exists, "host %s should be present in result metrics", host.String())

		i := int64(i)
		assert.Equal(t, hostMetric.CPU.UsagePercent, float64(i)/100.0)
		assert.Equal(t, hostMetric.Memory.ActiveKB, i)
		assert.Equal(t, hostMetric.Memory.ConsumedKB, i)
		assert.Equal(t, hostMetric.Memory.OverheadKB, i)
		// Test that the total memory value is converted from MB to KB.
		assert.Equal(t, hostMetric.Memory.TotalKB, i*1024)
	}

	// Test that when a host moref is present in the govmomi metrics request but its metric
	// results are missing, assembleMetrics creates an empty entry for the said host.
	morefToHost[fakeHost.Reference().String()] = fakeHost

	// Remove fakeHost's metrics from the results slice to feed into assembleMetrics.
	var fakeResults []performance.EntityMetric
	for i := range results {
		if results[i].Entity != fakeHost.Reference() {
			fakeResults = append(fakeResults, results[i])
		}
	}

	metrics = assembleMetrics(op, morefToHost, fakeResults)
	// Assembled metrics should now have an (empty) entry for fakeHost.
	assert.Equal(t, len(metrics), len(hosts)+1)
	fakeHostMetrics, exists := metrics[fakeHost.Reference().String()]
	assert.True(t, exists, "fakeHost %s should now be present in result metrics", fakeHost.String())
	expectedFakeMetrics := HostMetricsInfo{}
	assert.Equal(t, expectedFakeMetrics, *fakeHostMetrics)
}

func TestFilterHosts(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestFilterHosts")

	model, server, sess := test.VpxModelSetup(op, t)
	defer func() {
		model.Remove()
		server.Close()
	}()

	hosts, err := sess.Cluster.Hosts(op)
	if err != nil {
		t.Fatal(err)
	}

	filteredHosts, err := filterHosts(op, sess.Vim25(), hosts)
	assert.NoError(t, err)
	assert.Len(t, filteredHosts, len(hosts))

	h0 := hosts[0]
	spec := &types.HostMaintenanceSpec{}

	_, err = tasks.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
		return hosts[0].EnterMaintenanceMode(op, 30, false, spec)
	})
	assert.NoError(t, err)

	filteredHosts, err = filterHosts(op, sess.Vim25(), hosts)
	assert.NoError(t, err)
	assert.Len(t, filteredHosts, len(hosts)-1)
	assert.NotContains(t, filteredHosts, h0)

	// TODO(jzt): uncomment this when vcsim host_system.go supports DisconnectHost_Task
	// _, err = tasks.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
	// 	return hosts[0].Disconnect(op)
	// })
	// assert.NoError(t, err)

	// filteredHosts, err = filterHosts(op, sess, hosts)
	// assert.NoError(t, err)
	// assert.Len(t, filteredHosts, len(hosts)-1)
}

func TestFilterHostsEmptyList(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestFilterHostsEmptyList")

	empty := []*object.HostSystem{}
	h, err := filterHosts(op, nil, empty)
	assert.Error(t, err)
	assert.Len(t, h, 0)
}
