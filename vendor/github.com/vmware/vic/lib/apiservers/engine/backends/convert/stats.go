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
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/docker/api/types"

	"github.com/vmware/vic/pkg/vsphere/performance"
)

// ContainerStats encapsulates the conversion of VMMetrics to
// docker specific metrics
type ContainerStats struct {
	config *ContainerStatsConfig

	totalVCHMhz uint64
	dblVCHMhz   uint64
	preTotalMhz uint64

	preDockerStat *types.StatsJSON
	curDockerStat *types.StatsJSON
	currentMetric *performance.VMMetrics

	// disk & net stats are accumulated during the life of the
	// subscription. These maps will assist in that accumulation.
	diskStats map[string]performance.VirtualDisk
	netStats  map[string]performance.Network

	mu        sync.Mutex
	reader    *io.PipeReader
	writer    *io.PipeWriter
	listening bool
}

type ContainerStatsConfig struct {
	Ctx            context.Context
	Cancel         context.CancelFunc
	Out            io.Writer
	ContainerID    string
	ContainerState *types.ContainerState
	Memory         int64
	Stream         bool
	VchMhz         int64
}

type InvalidOrderError struct {
	current  time.Time
	previous time.Time
}

func (iso InvalidOrderError) Error() string {
	return fmt.Sprintf("The current sample time (%s) is before the previous time (%s)", iso.current, iso.previous)
}

// NewContainerStats will return a new instance of ContainerStats
func NewContainerStats(config *ContainerStatsConfig) *ContainerStats {
	return &ContainerStats{
		config:        config,
		curDockerStat: &types.StatsJSON{},
		totalVCHMhz:   uint64(config.VchMhz),
		dblVCHMhz:     uint64(config.VchMhz * 2),
		diskStats:     make(map[string]performance.VirtualDisk),
		netStats:      make(map[string]performance.Network),
	}
}

// IsListening returns the listening flag
func (cs *ContainerStats) IsListening() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.listening
}

// Stop will clean up the pipe and flip listening flag
func (cs *ContainerStats) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.listening {
		// #nosec: Errors unhandled.
		cs.reader.Close()
		// #nosec: Errors unhandled.
		cs.writer.Close()
		cs.listening = false
	}
}

// newPipe will initialize the pipe for encoding / decoding and
// set the listening flag
func (cs *ContainerStats) newPipe() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// create a new reader / writer
	cs.reader, cs.writer = io.Pipe()
	cs.listening = true
}

// Listen for new metrics from the portLayer, convert to docker format
// and encode to the configured Writer.
func (cs *ContainerStats) Listen() *io.PipeWriter {
	// Are we already listening?
	if cs.IsListening() {
		return nil
	}

	// create pipe for encode/decode
	cs.newPipe()

	dec := json.NewDecoder(cs.reader)
	doc := json.NewEncoder(cs.config.Out)

	// channel to transfer metric from decoder to encoder
	metric := make(chan performance.VMMetrics)

	// if we aren't streaming and the container is not running, then create an empty
	// docker stat to return
	if !cs.config.Stream && !cs.config.ContainerState.Running {
		cs.preDockerStat = &types.StatsJSON{}
	}

	// go routine to stop on Context.Cancel
	go func() {
		<-cs.config.Ctx.Done()
		close(metric)
		cs.Stop()
	}()

	// go routine will decode metrics received from the portLayer and
	// send them to the encoding routine
	go func() {
		for {
			select {
			case <-cs.config.Ctx.Done():
				return
			default:
				for dec.More() {
					var vmm performance.VMMetrics
					err := dec.Decode(&vmm)
					if err != nil {
						log.Errorf("container metric decoding error for container(%s): %s", cs.config.ContainerID, err)
						cs.config.Cancel()
					}
					// send the decoded metric for transform and encoding
					if cs.IsListening() {
						metric <- vmm
					}
				}
			}
		}

	}()

	// go routine will convert incoming metrics to docker specific stats and encode for the docker client.
	go func() {
		// docker needs updates quicker than vSphere can produce metrics, so we'll send a minimum of 1 metric/sec
		ticker := time.NewTicker(time.Millisecond * 500)
		for range ticker.C {
			select {
			case <-cs.config.Ctx.Done():
				ticker.Stop()
				return
			case nm := <-metric:
				// convert the Stat to docker struct
				stat, err := cs.ToContainerStats(&nm)
				if err != nil {
					log.Errorf("container metric conversion error for container(%s): %s", cs.config.ContainerID, err)
					cs.config.Cancel()
				}
				if stat != nil {
					cs.preDockerStat = stat
				}
			default:
				if cs.IsListening() && cs.preDockerStat != nil {
					// send docker stat to client
					err := doc.Encode(cs.preDockerStat)
					if err != nil {
						log.Warnf("container metric encoding error for container(%s): %s", cs.config.ContainerID, err)
						cs.config.Cancel()
					}
					// if we aren't streaming then cancel
					if !cs.config.Stream {
						cs.config.Cancel()
					}
				}
			}
		}
	}()

	return cs.writer
}

// ToContainerStats will convert the vic VMMetrics to a docker stats struct -- a complete docker stats
// struct requires two samples.  Func will return nil until a complete stat is available
func (cs *ContainerStats) ToContainerStats(current *performance.VMMetrics) (*types.StatsJSON, error) {
	// if we have a current metric then validate and transform
	if cs.currentMetric != nil {
		// do we have the same metric or has the metric not been initialized?
		if cs.currentMetric.SampleTime.Equal(current.SampleTime) || current.SampleTime.IsZero() {
			return nil, nil
		}
		// we have new current stats so need to move the previous CPU
		err := cs.previousCPU(current)
		if err != nil {
			return nil, err
		}
	}
	cs.currentMetric = current

	// create the current CPU stats
	cs.currentCPU()

	// create memory stats
	cs.memory()

	// create network stats
	cs.network()

	// create storage stats
	cs.disk()

	// set sample time
	cs.curDockerStat.Read = cs.currentMetric.SampleTime

	// PreRead will be zero if we don't have two samples
	if cs.curDockerStat.PreRead.IsZero() {
		return nil, nil
	}
	return cs.curDockerStat, nil
}

// network will calculate stats by network device.  The stats presented will be the
// network stats accumulated during the stats subscription.  This differs from vanilla
// docker as it provides the network stats for the lifetime of the container.
//
// TODO: Errors from either Tx or Rx are not currently supported (July 9th 2017)
func (cs *ContainerStats) network() {
	cs.curDockerStat.Networks = make(map[string]types.NetworkStats)
	for _, net := range cs.currentMetric.Networks {

		// get the previous network stats
		if preNet, exists := cs.netStats[net.Name]; exists {
			net.Rx.Bytes += preNet.Rx.Bytes
			net.Rx.Packets += preNet.Rx.Packets
			net.Rx.Dropped += preNet.Rx.Dropped
			net.Tx.Bytes += preNet.Tx.Bytes
			net.Tx.Packets += preNet.Tx.Packets
			net.Tx.Dropped += preNet.Tx.Dropped
			cs.netStats[net.Name] = net
		} else {
			// initial iteration
			cs.netStats[net.Name] = net
		}

		cs.curDockerStat.Networks[net.Name] = types.NetworkStats{
			RxBytes:   net.Rx.Bytes,
			RxPackets: uint64(net.Rx.Packets),
			RxDropped: uint64(net.Rx.Dropped),
			TxBytes:   net.Tx.Bytes,
			TxPackets: uint64(net.Tx.Packets),
			TxDropped: uint64(net.Tx.Dropped),
		}

	}
}

// disk will calculate supported stats by disk device.  The stats presented will be the
// disk stats accumulated during the stats subscription.  This differs from vanilla
// docker as it provides the disk stats for the lifetime of the container.
//
// Supported stats are io_service_bytes_recursive and io_serviced_recursive, so bytes and iops
// during the stats subscription
//
// TODO: Currently disk assumes a single scsi controller.  Multiple scsi controllers will need
// to be supported in a future release (July 9th 2017)
func (cs *ContainerStats) disk() {
	// docker storage stats to populate
	storage := types.BlkioStats{
		IoServiceBytesRecursive: []types.BlkioStatEntry{},
		IoServicedRecursive:     []types.BlkioStatEntry{},
	}

	for _, disk := range cs.currentMetric.Disks {
		// disk stats accumulate for the life of subscription, so
		// either add previous stats or store initial stats
		if preDisk, exists := cs.diskStats[disk.Name]; exists {
			// add previous values to current value
			disk.Read.Bytes += preDisk.Read.Bytes
			disk.Read.Op += preDisk.Read.Op
			disk.Write.Bytes += preDisk.Write.Bytes
			disk.Write.Op += preDisk.Write.Op
			cs.diskStats[disk.Name] = disk
		} else {
			// initial iteration
			cs.diskStats[disk.Name] = disk
		}

		// get the minor number for the disk device
		deviceMinor := diskMinor(cs.config.ContainerID, disk.Name)

		// need to update read, write & total for supported stats (bytes & iops)
		storage.IoServiceBytesRecursive = append(storage.IoServiceBytesRecursive,
			createBlkioStatsEntry(deviceMinor, "Read", cs.diskStats[disk.Name].Read.Bytes))
		storage.IoServiceBytesRecursive = append(storage.IoServiceBytesRecursive,
			createBlkioStatsEntry(deviceMinor, "Write", cs.diskStats[disk.Name].Write.Bytes))
		storage.IoServiceBytesRecursive = append(storage.IoServiceBytesRecursive,
			createBlkioStatsEntry(deviceMinor, "Total", cs.diskStats[disk.Name].Read.Bytes+cs.diskStats[disk.Name].Write.Bytes))
		// Ops
		storage.IoServicedRecursive = append(storage.IoServicedRecursive,
			createBlkioStatsEntry(deviceMinor, "Read", cs.diskStats[disk.Name].Read.Op))
		storage.IoServicedRecursive = append(storage.IoServicedRecursive,
			createBlkioStatsEntry(deviceMinor, "Write", cs.diskStats[disk.Name].Write.Op))
		storage.IoServicedRecursive = append(storage.IoServicedRecursive,
			createBlkioStatsEntry(deviceMinor, "Total", cs.diskStats[disk.Name].Read.Op+cs.diskStats[disk.Name].Write.Op))

	}
	// add the block stats to the docker stat
	cs.curDockerStat.BlkioStats = storage
}

func (cs *ContainerStats) memory() {
	// given MB (i.e. 2048) convert to GB
	cs.curDockerStat.MemoryStats.Limit = uint64(cs.config.Memory * 1024 * 1024)
	// given KB (i.e. 384.5) convert to Bytes
	cs.curDockerStat.MemoryStats.Usage = uint64(cs.currentMetric.Memory.Active * 1024)
}

// previousCPU will move the current stats to the previous CPU location
func (cs *ContainerStats) previousCPU(current *performance.VMMetrics) error {
	// validate that the sampling is in the correct order
	if current.SampleTime.Before(cs.curDockerStat.Read) {
		err := InvalidOrderError{
			current:  current.SampleTime,
			previous: cs.curDockerStat.Read,
		}
		return err
	}

	// move the stats
	cs.curDockerStat.PreCPUStats = cs.curDockerStat.CPUStats

	// set the previousTotal -- this will be added to the current CPU
	cs.preTotalMhz = cs.curDockerStat.PreCPUStats.CPUUsage.TotalUsage

	cs.curDockerStat.PreRead = cs.curDockerStat.Read
	// previous systemUsage will always be the VCH total
	// see note in func currentCPU() for detail
	cs.curDockerStat.PreCPUStats.SystemUsage = cs.totalVCHMhz

	return nil
}

// currentCPU will convert the VM CPU metrics to docker CPU stats
func (cs *ContainerStats) currentCPU() {
	cpuCount := len(cs.currentMetric.CPU.CPUs)
	dockerCPU := types.CPUStats{
		CPUUsage: types.CPUUsage{
			PercpuUsage: make([]uint64, cpuCount, cpuCount),
		},
	}

	// collect the current CPU Metrics
	for ci, current := range cs.currentMetric.CPU.CPUs {
		dockerCPU.CPUUsage.PercpuUsage[ci] = uint64(current.MhzUsage)
		dockerCPU.CPUUsage.TotalUsage += uint64(current.MhzUsage)
	}

	// vSphere will report negative usage for a starting VM, lets
	// set to zero
	if dockerCPU.CPUUsage.TotalUsage < 0 {
		dockerCPU.CPUUsage.TotalUsage = 0
	}

	// The first stat available for a VM will be missing detail
	if cpuCount > 0 {
		// TotalUsage is the sum of the individual vCPUs Mhz
		// consumption this reading.  We must divide that by the
		// number of vCPUs to get the average across both, since
		// the cpuUsage calc (explained below) will multiply by
		// the number of CPUs to get the cpuUsage percent
		dockerCPU.CPUUsage.TotalUsage /= uint64(cpuCount)
	}

	// Set the current systemUsage to double the VCH as the
	// previous systemUsage is the VCH total.  The docker
	// client formula creates a SystemDelta which is the following:
	// systemDelta = currentSystemUsage - previousSystemUsage
	// We always need systemDelta to equal the total amount of
	// VCH Mhz thus the doubling here.
	dockerCPU.SystemUsage = cs.dblVCHMhz

	// Much like systemUsage (above) totalCPUUsage and previous
	// totalCPUUsage will be used to create a CPUUsage delta as such:
	// CPUDelta = currentTotalCPUUsage - previousTotalCPUUsage
	// This amount will then be divided by the systemDelta
	// (explained above) as part of the CPU % Usage calculation
	// cpuUsage = (CPUDelta / SystemDelta) * cpuCount * 100
	// This will require the addition of the previous total usage
	dockerCPU.CPUUsage.TotalUsage += cs.preTotalMhz
	cs.curDockerStat.CPUStats = dockerCPU
}

// diskMinor will parse the disk name and return the minor id of
// the disk device.  The func assumes that minor identifiers are multiples
// of 16 (0,16,32,48,etc).
func diskMinor(containerID string, name string) uint64 {
	// disks are named scsi0:0, scsi0:1
	// i.e. controller+controller number:device number
	device := strings.Split(name, ":")
	// convert to an int
	minor, err := strconv.Atoi(device[len(device)-1])
	if err != nil {
		// log error, but continue and return a minor number of zero
		// unlikely this would happen, but if it does and there is more than one disk on the vm
		// then it could go undetected
		log.Errorf("stats error generating container(%s) disk(%s) minor: %s", containerID, name, err)
	}
	// minor identifiers are multiples of 16
	minor *= 16
	return uint64(minor)
}

func createBlkioStatsEntry(minor uint64, op string, value uint64) types.BlkioStatEntry {
	return types.BlkioStatEntry{
		Major: 8,
		Minor: minor,
		Op:    op,
		Value: value,
	}
}
