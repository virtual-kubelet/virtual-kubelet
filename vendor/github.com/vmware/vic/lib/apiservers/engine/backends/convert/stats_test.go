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

package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/vsphere/performance"
)

const (
	vcpuMhz        = 3300
	vcpuCount      = 1
	vchMhzTotal    = 3300
	memConsumed    = 1024 * 1024 * 500
	memProvisioned = 1024 * 1024 * 1024
)

func TestContainerConverter(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)

	// grab a config object
	config := ccConfig(plumb)

	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	// returned writer is given to PL
	writer := cStats.Listen()
	assert.NotNil(t, writer)
	// second call should result in nil writer as
	// we are already listening
	w2 := cStats.Listen()
	assert.Nil(t, w2)

	// // ensure stop closes reader / writer
	cStats.Stop()
	// verify we stopped listening
	assert.False(t, cStats.IsListening())
}

func TestToContainerStats(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)

	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	initCPU := 1000
	vmBefore := vmMetrics(vcpuCount, initCPU)
	vmm := vmMetrics(vcpuCount, initCPU)
	// ensure we are after the initial metric
	vmm.SampleTime.Add(time.Second * 1)

	// first metric sent, should return nil
	js, err := cStats.ToContainerStats(vmm)
	assert.NoError(t, err)
	assert.Nil(t, js)

	// send the same stat should return nil
	js, err = cStats.ToContainerStats(vmm)
	assert.Nil(t, err)
	assert.Nil(t, js)

	// send out of order stat
	js, err = cStats.ToContainerStats(vmBefore)
	assert.NotNil(t, err)
	assert.Nil(t, js)

	secondCPU := 250
	// create a new metric
	vmmm := vmMetrics(vcpuCount, secondCPU)
	// sample will be 20 seconds apart..
	vmmm.SampleTime = vmm.SampleTime.Add(time.Second * 20)

	js, err = cStats.ToContainerStats(vmmm)
	assert.NoError(t, err)
	assert.NotZero(t, js.Read, js.PreRead)
	assert.Equal(t, uint64(vchMhzTotal*2), js.CPUStats.SystemUsage)
	assert.Equal(t, uint64(secondCPU+initCPU), js.CPUStats.CPUUsage.TotalUsage)
	assert.Equal(t, uint64(initCPU), js.PreCPUStats.CPUUsage.TotalUsage)
	assert.Equal(t, uint64(vchMhzTotal), js.PreCPUStats.SystemUsage)

	// this reading should show 250mhz of 3300mhz used -- 7.58%
	cpuPercent := fmt.Sprintf("%2.2f", calculateCPUPercentUnix(js.PreCPUStats.CPUUsage.TotalUsage, js.PreCPUStats.SystemUsage, js))
	assert.Equal(t, "7.58", cpuPercent)

	config.Cancel()
	<-config.Ctx.Done()
	// verify we stopped listening
	assert.True(t, success(cStats))
}

func TestContainerStatsListener(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	// start the listener
	writer := cStats.Listen()
	assert.NotNil(t, writer)

	// create an initial metric
	initCPU := 1000
	vm := vmMetrics(vcpuCount, initCPU)
	err := plumb.mockPLMetrics(vm, writer)
	assert.NoError(t, err)

	// send second metric
	vmm := vmMetrics(vcpuCount, initCPU+100)
	vmm.SampleTime = vm.SampleTime.Add(time.Second * 20)
	err = plumb.mockPLMetrics(vmm, writer)
	assert.NoError(t, err)

	// did client receive metric??
	ds, err := plumb.mockDockerClient()
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Equal(t, uint64((initCPU*2+100)/vcpuCount), ds.CPUStats.CPUUsage.TotalUsage)

	// docker expects data quicker than vSphere can produce -- sleep for just over 1 sec
	// and ensure the previous docker stat is returned to client
	time.Sleep(time.Millisecond * 1100)
	same, err := plumb.mockDockerClient()
	assert.NoError(t, err)
	assert.NotNil(t, same)
	assert.Equal(t, ds.CPUStats.CPUUsage.TotalUsage, same.CPUStats.CPUUsage.TotalUsage)

	config.Cancel()
	<-config.Ctx.Done()

	// verify we stopped listening
	assert.True(t, success(cStats))
}

func TestContainerConvertCtxCancel(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	// start the listener
	writer := cStats.Listen()
	assert.NotNil(t, writer)

	// cancel the context
	config.Cancel()
	<-config.Ctx.Done()
	// verify we stopped listening
	assert.True(t, success(cStats))
}

func TestContainerConvertNoStream(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	config.Stream = false
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	// start the listener
	writer := cStats.Listen()
	assert.NotNil(t, writer)

	// create an initial metric
	initCPU := 1000
	vm := vmMetrics(vcpuCount, initCPU)
	err := plumb.mockPLMetrics(vm, writer)
	assert.NoError(t, err)

	// send second metric
	vmm := vmMetrics(vcpuCount, initCPU+100)
	vmm.SampleTime = vm.SampleTime.Add(time.Second * 20)
	err = plumb.mockPLMetrics(vmm, writer)
	assert.NoError(t, err)

	ds, err := plumb.mockDockerClient()
	assert.NoError(t, err)
	assert.NotNil(t, ds)

	// converter canceled the context
	<-config.Ctx.Done()
	// verify we stopped listening
	assert.True(t, success(cStats))
}

func TestContainerNotRunningNoStream(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	config.Stream = false
	config.ContainerState.Running = false
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)

	// start the listener
	writer := cStats.Listen()
	assert.NotNil(t, writer)

	ds, err := plumb.mockDockerClient()
	assert.NoError(t, err)
	assert.NotNil(t, ds)

	// converter canceled the context
	<-config.Ctx.Done()
	// verify we stopped listening
	assert.True(t, success(cStats))
}

func TestDiskMinor(t *testing.T) {
	containerID := "12345"
	for i := 0; i <= 15; i++ {
		name := fmt.Sprintf("scsi0:%d", i)
		assert.Equal(t, uint64(i*16), diskMinor(containerID, name))
	}

	minor := uint64(0)
	// test with invalid disk names to ensure no panic, etc
	assert.Equal(t, minor, diskMinor(containerID, "foo:bar:0"))
	assert.Equal(t, minor, diskMinor(containerID, "foo"))
	assert.Equal(t, minor, diskMinor(containerID, "foo:"))
}

func TestCreateBlkioStatsEntry(t *testing.T) {
	minor := uint64(0)
	val := uint64(12)
	maj := uint64(8)
	entry := createBlkioStatsEntry(minor, "Read", val)
	assert.Equal(t, "Read", entry.Op)
	assert.Equal(t, val, entry.Value)
	assert.Equal(t, minor, entry.Minor)
	assert.Equal(t, maj, entry.Major)
}

func TestDiskStats(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)
	// create metric
	initCPU := 1000
	vm := vmMetrics(vcpuCount, initCPU)
	cStats.currentMetric = vm

	// update disk
	cStats.disk()

	assert.Equal(t, 3, len(cStats.curDockerStat.BlkioStats.IoServiceBytesRecursive))
	assert.Equal(t, 1, len(cStats.diskStats))

	// update again -- this should accumulate the totals
	cStats.disk()
	assert.Equal(t, 3, len(cStats.curDockerStat.BlkioStats.IoServiceBytesRecursive))
	assert.Equal(t, 1, len(cStats.diskStats))

	for _, disk := range cStats.curDockerStat.BlkioStats.IoServiceBytesRecursive {
		switch disk.Op {
		case "Write":
			assert.Equal(t, uint64(vm.Disks[0].Write.Bytes*2), disk.Value)
		}
	}
}

func TestNetworkStats(t *testing.T) {
	plumb := setup()
	defer teardown(plumb)
	// grab a config object
	config := ccConfig(plumb)
	cStats := NewContainerStats(config)
	assert.NotNil(t, cStats)
	// create metric
	initCPU := 1000
	vm := vmMetrics(vcpuCount, initCPU)
	cStats.currentMetric = vm

	// update network
	cStats.network()

	assert.Equal(t, 1, len(cStats.curDockerStat.Networks))
	assert.Equal(t, 1, len(cStats.netStats))

	// update again -- this should accumulate the totals
	cStats.network()
	assert.Equal(t, 1, len(cStats.curDockerStat.Networks))
	assert.Equal(t, 1, len(cStats.netStats))

	for network, usage := range cStats.curDockerStat.Networks {
		switch network {
		case "eth0":
			assert.Equal(t, uint64(200), usage.RxBytes)
		}
	}
}

// Test Helpers

type plumbing struct {
	r   *io.PipeReader
	w   *io.PipeWriter
	out io.Writer
	// mock portlayer
	mockPL *json.Encoder
	// mock docker client decoder
	mockDoc *json.Decoder
}

func setup() *plumbing {
	r, o := io.Pipe()
	out := io.Writer(o)

	return &plumbing{
		r:       r,
		w:       o,
		out:     out,
		mockDoc: json.NewDecoder(r),
	}
}

// success is a helper to check the listening status of the
// converter
func success(converter *ContainerStats) bool {
	op := func() error {
		listen := converter.IsListening()
		if listen {
			return fmt.Errorf("still listening: %t", listen)
		}
		return nil
	}
	wait := func(err error) bool {
		if err != nil {
			return true
		}
		return false
	}
	// use the retry package and keep retrying until we've hit the limit
	if err := retry.Do(op, wait); err != nil {
		return false
	}
	return true
}

func teardown(p *plumbing) {
	// close the reader / writer
	p.r.Close()
	p.w.Close()
}

func (p *plumbing) mockPLMetrics(metric *performance.VMMetrics, writer io.Writer) error {
	if p.mockPL == nil {
		p.mockPL = json.NewEncoder(writer)
	}
	return p.mockPL.Encode(metric)
}

func (p *plumbing) mockDockerClient() (*types.StatsJSON, error) {
	docStats := &types.StatsJSON{}

	err := p.mockDoc.Decode(docStats)
	if err != nil {
		return nil, err
	}

	return docStats, nil
}

func ccConfig(p *plumbing) *ContainerStatsConfig {
	// test config
	ctx, cancel := context.WithCancel(context.Background())
	config := &ContainerStatsConfig{
		VchMhz:      int64(vchMhzTotal),
		Ctx:         ctx,
		Cancel:      cancel,
		ContainerID: "1234",
		Out:         p.out,
		Stream:      true,
		Memory:      2048,
		ContainerState: &types.ContainerState{
			Running: true,
		},
	}
	return config
}

func vmMetrics(count int, vcpuMhz int) *performance.VMMetrics {
	vmm := &performance.VMMetrics{}
	vmm.SampleTime = time.Now()
	vmm.CPU = cpuUsageMetrics(count, vcpuMhz)
	vmm.Memory = performance.MemoryMetrics{
		Consumed:    int64(memConsumed),
		Provisioned: int64(memProvisioned),
	}
	disk := performance.VirtualDisk{
		Name: "scsi0:0",
		Write: performance.DiskUsage{
			Bytes: uint64(100),
			Kbps:  5,
			Op:    uint64(5),
			Ops:   5,
		},
		Read: performance.DiskUsage{
			Bytes: uint64(10),
			Kbps:  5,
			Op:    uint64(5),
			Ops:   5,
		},
	}
	vmm.Disks = append(vmm.Disks, disk)
	network := performance.Network{
		Name: "eth0",
		Rx: performance.NetworkUsage{
			Bytes:   uint64(100),
			Kbps:    5,
			Packets: 1,
		},
		Tx: performance.NetworkUsage{
			Bytes:   uint64(10),
			Packets: 1,
		},
	}
	vmm.Networks = append(vmm.Networks, network)
	return vmm
}

// cpuUsageMetrics will return a populated CPUMetrics struct
func cpuUsageMetrics(count int, cpuMhz int) performance.CPUMetrics {
	vmCPUs := make([]performance.CPUUsage, count, count)
	total := count * cpuMhz
	for i := range vmCPUs {
		vmCPUs[i] = performance.CPUUsage{
			ID:       i,
			MhzUsage: int64(cpuMhz),
		}
	}

	return performance.CPUMetrics{
		CPUs:  vmCPUs,
		Usage: calcVCPUUsage(total),
	}
}

// calcUsage is a helper function that will take the total provdied usage
// and convert to percentage of total vCPU usage
func calcVCPUUsage(total int) float32 {
	return float32(total) / (vcpuMhz * vcpuCount)
}

// calculateCPUPercentUnix is a copy from docker to test the percentage calculations
func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}
