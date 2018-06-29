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

package performance

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

const (
	// number of samples per collection
	sampleSize = int32(2)
	// number of seconds between sample collection
	sampleInterval = int32(20)
	// vSphere recommomends a maxiumum of 50 entities per query
	maxEntityQuery = 50
)

// CPUUsage provides individual CPU metrics
type CPUUsage struct {
	// processor id (0,1,2)
	ID int
	// MhzUsage is the MhZ consumed by a specific processor
	MhzUsage int64
}

// CPUMetrics encapsulates available vm CPU metrics
type CPUMetrics struct {
	// CPUs are the individual CPU metrics
	CPUs []CPUUsage
	// Usage is the percentage of total vm CPU usage
	Usage float32
}

// MemoryMetrics encapsulates available vm memory metrics
type MemoryMetrics struct {
	// Consumed memory of vm in bytes
	Consumed int64
	// Active memory of vm in bytes
	Active int64
	// Provisioned memory of vm in bytes
	Provisioned int64
}

// NetworkUsage provides detailed network stats
type NetworkUsage struct {
	Bytes   uint64 // total bytes
	Kbps    int64  // KiloBytesPerSecond
	Packets int64  // total packet count
	Errors  int64  // NOT CURRENTLY IMPLEMENTED
	Dropped int64  // total dropped packet count
}

// Network provides metrics for individual network devices
type Network struct {
	Name string
	Rx   NetworkUsage
	Tx   NetworkUsage
}

// DiskUsage provides detailed disk stats
type DiskUsage struct {
	Bytes uint64 // total bytes for interval
	Kbps  int64  // KiloBytesPerSecond for interval
	Op    uint64 // Operation count for interval
	Ops   int64  // Operations per second
}

// VirtualDisk provides metrics for individual disks
type VirtualDisk struct {
	Name  string
	Write DiskUsage
	Read  DiskUsage
}

// VMMetrics encapsulates the available metrics
type VMMetrics struct {
	CPU        CPUMetrics
	Memory     MemoryMetrics
	Networks   []Network
	Disks      []VirtualDisk
	SampleTime time.Time
	// interval of collection in seconds
	Interval int32
}

// VMCollector is the VM metrics collector
type VMCollector struct {
	perfMgr *performance.Manager
	session *session.Session

	timer   *time.Ticker
	stopper chan struct{}

	// subscribers to streaming
	mu   sync.RWMutex
	subs map[types.ManagedObjectReference]*vmSubscription
}

// newVMCollector will instantiate a new collector responsible for
// gathering VM metrics
func NewVMCollector(session *session.Session) *VMCollector {
	return &VMCollector{
		subs:    make(map[types.ManagedObjectReference]*vmSubscription),
		perfMgr: performance.NewManager(session.Vim25()),
		session: session,
	}
}

// Start will begin the collection polling process
func (vmc *VMCollector) Start() {
	// create timer with sampleInterval alignment
	vmc.timer = time.NewTicker(time.Duration(int64(sampleInterval)) * time.Second)
	//create the stopper channel
	vmc.stopper = make(chan struct{})
	// loop on the channel
	go func() {
		ctx := context.Background()
		collectorOp := trace.NewOperation(ctx, "VM metrics collector")
		go vmc.collect(collectorOp)
		for {
			select {
			case <-vmc.timer.C:
				// collect metrics for current subscribers
				vmc.collect(collectorOp)
			case <-vmc.stopper:
				// Ticker has been stopped, exit routine
				collectorOp.Debugf("VM metrics collector complete")
				return
			}
		}

	}()
}

// Stop will stop the collection polling process
func (vmc *VMCollector) Stop() {
	vmc.timer.Stop()
	close(vmc.stopper)
}

// collect will query vSphere for VM metrics and return to the subscribers
func (vmc *VMCollector) collect(op trace.Operation) {

	// gather the chunked morefs
	vmReferences := vmc.subscriberReferences(op)

	// iterate over the chunked references and sample vSphere
	for i := range vmReferences {
		// create a new operation so we can effectively log and measure the sample
		sop := trace.NewOperation(op.Context, "sample operation")
		sop.Debugf("parentOp[%s] sample(%d/%d) with %d morefs", op.ID(), i+1, len(vmReferences), len(vmReferences[i]))
		go vmc.sample(sop, vmReferences[i])
	}
}

// subscriberReferences will return a two dimensional array of VM managed object references.  The
// references are chunked based on the maxEntityQuery limit.
func (vmc *VMCollector) subscriberReferences(op trace.Operation) [][]types.ManagedObjectReference {
	var mos []types.ManagedObjectReference
	var chunked [][]types.ManagedObjectReference

	vmc.mu.Lock()
	defer vmc.mu.Unlock()
	op.Debugf("begin subscriberReferences: %d", len(vmc.subs))

	// populate a two dimensional array chunked by the maxEntityQueryLimit
	for mo := range vmc.subs {
		mos = append(mos, mo)
		if len(mos) == maxEntityQuery {
			chunked = append(chunked, mos)
			mos = make([]types.ManagedObjectReference, 0, maxEntityQuery)
		}
	}

	// the initial loop will potentially miss the "remainder" morefs
	if len(mos) > 0 && len(mos) < maxEntityQuery {
		chunked = append(chunked, mos)
	}

	op.Debugf("end subscriberReferences: %d", len(chunked))
	return chunked
}

// sample will query the vSphere performanceManager and publish the gather metrics
func (vmc *VMCollector) sample(op trace.Operation, mos []types.ManagedObjectReference) {
	op.Debugf("begin sample for %d morefs", len(mos))
	defer op.Debugf("end sample for %d morefs", len(mos))

	// vSphere counters we are currently interested in monitoring
	counters := []string{"cpu.usagemhz.average", "mem.active.average",
		"virtualDisk.write.average", "virtualDisk.read.average",
		"virtualDisk.numberReadAveraged.average", "virtualDisk.numberWriteAveraged.average",
		"net.bytesRx.average", "net.bytesTx.average",
		"net.droppedRx.summation", "net.droppedTx.summation",
		"net.packetsRx.summation", "net.packetsTx.summation"}

	// create the spec
	spec := types.PerfQuerySpec{
		Format:     string(types.PerfFormatNormal),
		MaxSample:  sampleSize,
		IntervalId: sampleInterval,
	}

	// retrieve sample based on counter names
	sample, err := vmc.perfMgr.SampleByName(op.Context, spec, counters, mos)
	if err != nil {
		op.Errorf("unable to get metric sample: %s", err)
		return
	}

	// convert to metrics
	result, err := vmc.perfMgr.ToMetricSeries(op.Context, sample)
	if err != nil {
		op.Errorf("unable to convert metric sample to metric series: %s", err)
		return
	}

	// iterate over results, convert to vic metrics and publish to subscribers
	for i := range result {
		met := result[i]
		sub, exists := vmc.subs[met.Entity]
		if !exists {
			// the subscription is no longer valid go to the next result
			continue
		}
		// convert the sample to a metric and publish
		for s := range met.SampleInfo {

			metric := &VMMetrics{
				CPU: CPUMetrics{
					CPUs: []CPUUsage{},
				},
				Memory:     MemoryMetrics{},
				SampleTime: met.SampleInfo[s].Timestamp,
				Interval:   sampleInterval,
			}

			// the series will have values for each sample
			for _, v := range met.Value {
				// skip the aggregate metric (empty string) and any negative values
				if v.Instance == "" && v.Name != "mem.active.average" || v.Value[s] < 0 {
					continue
				}

				switch v.Name {
				case "cpu.usagemhz.average":
					// convert cpu instance to int
					cpu, err := instanceID(op, sub.ID(), v.Instance)
					if err != nil {
						// skipping this instance
						continue
					}
					// specific vCPU metric
					vcpu := CPUUsage{
						ID:       cpu,
						MhzUsage: v.Value[s],
					}
					metric.CPU.CPUs = append(metric.CPU.CPUs, vcpu)
				case "mem.active.average":
					metric.Memory.Active = v.Value[s]
				// DISK
				case "virtualDisk.read.average":
					disk := findDisk(metric, v.Instance)
					// perfManager returns kbps -- convert to sum of bytes
					sum := summation(v.Value[s]) * 1024
					metric.Disks[disk].Read.Bytes = sum
					metric.Disks[disk].Read.Kbps = v.Value[s]
				case "virtualDisk.write.average":
					disk := findDisk(metric, v.Instance)
					// perfManager returns kbps -- convert to sum of bytes
					sum := summation(v.Value[s]) * 1024
					metric.Disks[disk].Write.Bytes = sum
					metric.Disks[disk].Write.Kbps = v.Value[s]
				case "virtualDisk.numberReadAveraged.average":
					disk := findDisk(metric, v.Instance)
					// sum of iop read average
					metric.Disks[disk].Read.Op = summation(v.Value[s])
					metric.Disks[disk].Read.Ops = v.Value[s]
				case "virtualDisk.numberWriteAveraged.average":
					disk := findDisk(metric, v.Instance)
					// sum of iop write average
					metric.Disks[disk].Write.Op = summation(v.Value[s])
					metric.Disks[disk].Write.Ops = v.Value[s]
				// NET
				default:
					name := sub.DeviceName(v.Instance)
					// if we have a name gather the stats
					if name != "" {
						// get the network to update
						net := findNetwork(metric, name)
						switch v.Name {
						case "net.bytesRx.average":
							// perfManager returns kbps -- convert to sum of bytes
							sum := summation(v.Value[s]) * 1024
							metric.Networks[net].Rx.Bytes = sum
							metric.Networks[net].Rx.Kbps = v.Value[s]
						case "net.bytesTx.average":
							// perfManager returns kbps -- convert to sum of bytes
							sum := summation(v.Value[s]) * 1024
							metric.Networks[net].Tx.Bytes = sum
							metric.Networks[net].Tx.Kbps = v.Value[s]
						case "net.droppedRx.summation":
							metric.Networks[net].Rx.Dropped = v.Value[s]
						case "net.droppedTx.summation":
							metric.Networks[net].Tx.Dropped = v.Value[s]
						case "net.packetsRx.summation":
							metric.Networks[net].Rx.Packets = v.Value[s]
						case "net.packetsTx.summation":
							metric.Networks[net].Tx.Packets = v.Value[s]
						}
					}
				}
			}
			sub.Publish(metric)
		}
	}
}

// Subscribe to a vm metric subscription
func (vmc *VMCollector) Subscribe(op trace.Operation, moref types.ManagedObjectReference, id string) (chan interface{}, error) {
	vmc.mu.Lock()
	defer vmc.mu.Unlock()

	// used at end of func
	subscriptionCount := len(vmc.subs)

	// do we already have this subscription?
	_, exists := vmc.subs[moref]
	if !exists {
		op.Debugf("Creating new subscription(%s)", id)
		sub, err := newVMSubscription(op, vmc.session, moref, id)
		if err != nil {
			return nil, err
		}
		vmc.subs[moref] = sub
	}

	// get a subscriber channel
	ch := vmc.subs[moref].Channel()

	// This is first subscription so start collection
	if subscriptionCount == 0 {
		vmc.Start()
	}

	return ch, nil
}

// Unsubscribe from a vm metric subscription.  The subscriber channel will
// be evicted and when no subscribers remain the subscription will be removed.
func (vmc *VMCollector) Unsubscribe(op trace.Operation, moref types.ManagedObjectReference, ch chan interface{}) {
	vmc.mu.Lock()
	defer vmc.mu.Unlock()

	sub, exists := vmc.subs[moref]
	if exists {
		// remove the communication channel
		sub.Evict(ch)
		// do we have any subscribers to this subscription
		if sub.Publishers() == 0 {
			op.Debugf("Deleting metric subscription(%s)", sub.ID())
			delete(vmc.subs, moref)
		}
		op.Debugf("Unsubscribed %s from metrics", sub.ID())
	}

	// no subscriptions, so stop the collection
	if len(vmc.subs) == 0 {
		vmc.Stop()
	}
}

// instanceID coverts the ID or Key of a metric from a string to int.
func instanceID(op trace.Operation, subscriptionID string, instance string) (int, error) {
	converted, err := strconv.Atoi(instance)
	if err != nil {
		// I don't expect this to ever happen, but if it does log and don't publish
		op.Errorf("metrics failed to convert the subscription(%s) device id to an int - value(%#v): %s", subscriptionID, instance, err)
		return converted, err
	}
	return converted, nil
}

// summation returns the product of the average * interval
func summation(avg int64) uint64 {
	return uint64(avg) * uint64(sampleInterval)
}

// findDisk will iterate over the Disks and return the DiskUsage ordinal position
func findDisk(metric *VMMetrics, name string) int {
	// find by name
	for i := range metric.Disks {
		if metric.Disks[i].Name == name {
			return i
		}
	}
	// Let's create the disk
	d := VirtualDisk{
		Name:  name,
		Read:  DiskUsage{},
		Write: DiskUsage{},
	}
	metric.Disks = append(metric.Disks, d)

	return len(metric.Disks) - 1
}

// findNetwork will iterate over the Networks and return the NetworkUsage ordinal position
func findNetwork(metric *VMMetrics, name string) int {
	// find by name
	for i := range metric.Networks {
		if metric.Networks[i].Name == name {
			return i
		}
	}
	// Let's create the disk
	net := Network{
		Name: name,
		Rx:   NetworkUsage{},
		Tx:   NetworkUsage{},
	}
	metric.Networks = append(metric.Networks, net)

	return len(metric.Networks) - 1
}
