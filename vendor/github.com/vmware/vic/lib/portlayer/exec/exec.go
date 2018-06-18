// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package exec

import (
	"fmt"
	"sync"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"

	"github.com/vmware/vic/lib/portlayer/event"
	"github.com/vmware/vic/lib/portlayer/event/collector/vsphere"
	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
)

var (
	initializer struct {
		err  error
		once sync.Once
	}
)

func Init(ctx context.Context, sess *session.Session, source extraconfig.DataSource, _ extraconfig.DataSink) error {
	log.Info("Beginning initialization of portlayer exec component")
	initializer.once.Do(func() {
		var err error
		defer func() {
			if err != nil {
				initializer.err = err
			}
		}()
		f := find.NewFinder(sess.Vim25(), false)

		extraconfig.Decode(source, &Config)

		log.Debugf("Decoded VCH config for execution: %#v", Config)
		ccount := len(Config.ComputeResources)
		if ccount != 1 {
			err = fmt.Errorf("expected singular compute resource element, found %d", ccount)
			log.Error(err)
			return
		}

		cr := Config.ComputeResources[0]
		var r object.Reference
		r, err = f.ObjectReference(ctx, cr)
		if err != nil {
			err = fmt.Errorf("could not get resource pool or virtual app reference from %q: %s", cr.String(), err)
			log.Error(err)
			return
		}
		switch o := r.(type) {
		case *object.VirtualApp:
			Config.VirtualApp = o
			Config.ResourcePool = o.ResourcePool
		case *object.ResourcePool:
			Config.ResourcePool = o
		default:
			err = fmt.Errorf("could not get resource pool or virtual app from reference %q: object type is wrong", cr.String())
			log.Error(err)
			return
		}

		// we want to monitor the cluster, so create a vSphere Event Collector
		// The cluster managed object will either be a proper vSphere Cluster or
		// a specific host when standalone mode
		ec := vsphere.NewCollector(sess.Vim25(), sess.Cluster.Reference().String())

		// start the collection of vsphere events
		err = ec.Start()
		if err != nil {
			err = fmt.Errorf("%s failed to start: %s", ec.Name(), err)
			log.Error(err)
			return
		}

		// instantiate the container cache now
		NewContainerCache()

		// create the event manager &  register the existing collector
		Config.EventManager = event.NewEventManager(ec)

		// subscribe the exec layer to the event stream for Vm events
		vmSub := Config.EventManager.Subscribe(events.NewEventType(vsphere.VMEvent{}).Topic(), "exec", func(e events.Event) {
			if c := Containers.Container(e.Reference()); c != nil {
				c.OnEvent(e)
			}
		})

		// Grab the AboutInfo about our host environment
		about := sess.Vim25().ServiceContent.About

		vch := GetVCHstats(ctx)
		Config.VCHMhz = vch.CPULimit
		Config.VCHMemoryLimit = vch.MemoryLimit

		Config.HostOS = about.OsType
		Config.HostOSVersion = about.Version
		Config.HostProductName = about.Name
		log.Debugf("Host - OS (%s), version (%s), name (%s)", about.OsType, about.Version, about.Name)
		log.Debugf("VCH limits - %d Mhz, %d MB", Config.VCHMhz, Config.VCHMemoryLimit)

		// sync container cache
		vmSub.Suspend(true)
		defer vmSub.Resume()
		log.Info("Syncing container cache")
		if err = Containers.sync(ctx, sess); err != nil {
			log.Errorf("Error encountered during container cache sync during init process: %s", err)
			return
		}
	})

	return initializer.err
}

// publishContainerEvent will publish a ContainerEvent to the vic event stream
func publishContainerEvent(op trace.Operation, id string, created time.Time, eventType string) {
	if Config.EventManager == nil || eventType == "" {
		return
	}

	ce := &events.ContainerEvent{
		BaseEvent: &events.BaseEvent{
			// containerEvents are a construct of vic, so lets set the
			// ID equal to the operation that created the event
			ID:          op.ID(),
			Ref:         id,
			CreatedTime: created,
			Event:       eventType,
			Detail:      fmt.Sprintf("Container %s %s", id, eventType),
		},
	}

	Config.EventManager.Publish(ce)
}
