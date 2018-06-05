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

package network

import (
	"context"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/portlayer/event"
	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/store"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
)

var (
	DefaultContext *Context

	initializer struct {
		err  error
		once sync.Once
	}
)

type DuplicateResourceError struct {
	resID string
}

type ResourceNotFoundError struct {
	error
}

func (e DuplicateResourceError) Error() string {
	return fmt.Sprintf("%s already exists", e.resID)
}

func Init(ctx context.Context, sess *session.Session, source extraconfig.DataSource, sink extraconfig.DataSink) error {
	trace.End(trace.Begin(""))

	initializer.once.Do(func() {
		var err error
		defer func() {
			initializer.err = err
		}()

		f := find.NewFinder(sess.Vim25(), false)

		var config Configuration
		config.sink = sink
		config.source = source
		config.Decode()
		config.PortGroups = make(map[string]object.NetworkReference)

		log.Debugf("Decoded VCH config for network: %#v", config)
		for nn, n := range config.ContainerNetworks {
			pgref := new(types.ManagedObjectReference)
			if !pgref.FromString(n.ID) {
				log.Warnf("Could not reacquire object reference from id for network %s: %s", nn, n.ID)
			}

			var r object.Reference
			if r, err = f.ObjectReference(ctx, *pgref); err != nil {
				log.Warnf("could not get network reference for %s network: %s", nn, err)
				err = nil
				continue
			}

			config.PortGroups[nn] = r.(object.NetworkReference)
		}

		// make sure a NIC attached to the bridge network exists
		config.BridgeLink, err = getBridgeLink(&config)
		if err != nil {
			return
		}

		var kv kvstore.KeyValueStore
		kv, err = store.NewDatastoreKeyValue(ctx, sess, "network.contexts.default")
		if err != nil {
			return
		}

		var netctx *Context
		if netctx, err = NewContext(&config, kv); err != nil {
			return
		}

		if err = engageContext(ctx, netctx, exec.Config.EventManager); err == nil {
			DefaultContext = netctx
			log.Infof("Default network context allocated")
		}
	})

	return initializer.err
}

// handleEvent processes events
func handleEvent(netctx *Context, ie events.Event) {
	switch ie.String() {
	case events.ContainerPoweredOff:
		op := trace.NewOperation(context.Background(), fmt.Sprintf("handleEvent(%s)", ie.EventID()))
		op.Infof("Handling Event: %s", ie.EventID())
		// grab the operation from the event
		handle := exec.GetContainer(op, uid.Parse(ie.Reference()))
		if handle == nil {
			_, err := netctx.RemoveIDFromScopes(op, ie.Reference())
			if err != nil {
				op.Errorf("Failed to remove container %s scope: %s", ie.Reference(), err)
			}
			return
		}
		defer handle.Close()

		if handle.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
			op.Warnf("Live power state check on power off event shows %s: not unbinding network", ie.Reference(), handle.Runtime.PowerState)
			return
		}

		if _, err := netctx.UnbindContainer(op, handle); err != nil {
			op.Warnf("Failed to unbind container %s: %s", ie.Reference(), err)
			return
		}

		if err := handle.Commit(op, nil, nil); err != nil {
			op.Warnf("Failed to commit handle after network unbind for container %s: %s", ie.Reference(), err)
		}

	}
	return
}

// engageContext connects the given network context into a vsphere environment
// using an event manager, and a container cache. This hooks up a callback to
// react to vsphere events, as well as populate the context with any containers
// that are present.
func engageContext(ctx context.Context, netctx *Context, em event.EventManager) error {
	var err error

	// grab the context lock so that we do not unbind any containers
	// that stop out of band. this could cause, for example, for us
	// to bind a container when it has already been unbound by an
	// event callback
	netctx.Lock()
	defer netctx.Unlock()

	// subscribe to the event stream for Vm events
	if em == nil {
		return fmt.Errorf("event manager is required for default network context")
	}

	sub := fmt.Sprintf("%s(%p)", "netCtx", netctx)
	topic := events.NewEventType(events.ContainerEvent{}).Topic()
	s := em.Subscribe(topic, sub, func(ie events.Event) {
		handleEvent(netctx, ie)
	})

	defer func() {
		if err != nil {
			em.Unsubscribe(topic, sub)
		}
	}()

	op := trace.NewOperation(context.Background(), "engageContext")
	s.Suspend(true)
	defer s.Resume()
	for _, c := range exec.Containers.Containers(nil) {
		log.Debugf("adding container %s", c)
		h := c.NewHandle(ctx)
		defer h.Close()

		// add any user created networks that show up in container's config
		for n, ne := range h.ExecConfig.Networks {
			var s []*Scope
			s, err = netctx.findScopes(&n)
			if err != nil {
				if _, ok := err.(ResourceNotFoundError); !ok {
					return err
				}
			}

			if len(s) > 0 {
				continue
			}

			pools := make([]string, len(ne.Network.Pools))
			for i := range ne.Network.Pools {
				pools[i] = ne.Network.Pools[i].String()
			}

			log.Debugf("adding scope %s", n)

			scopeData := &ScopeData{
				ScopeType: ne.Network.Type,
				Name:      n,
				Subnet:    &ne.Network.Gateway,
				Gateway:   ne.Network.Gateway.IP,
				DNS:       ne.Network.Nameservers,
				Pools:     pools,
			}
			if _, err = netctx.newScope(scopeData); err != nil {
				return err
			}
		}

		if c.CurrentState() == exec.StateRunning {
			if _, err = netctx.bindContainer(op, h); err != nil {
				return err
			}
		}
	}

	return nil
}

func getBridgeLink(config *Configuration) (Link, error) {
	// add the gateway address to the bridge interface
	link, err := LinkByName(config.BridgeNetwork)
	if err != nil {
		// lookup by alias
		return LinkByAlias(config.BridgeNetwork)
	}

	return link, nil
}
