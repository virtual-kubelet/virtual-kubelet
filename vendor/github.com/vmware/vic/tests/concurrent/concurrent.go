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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sethgrid/multibar"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	// concurrency
	DefaultConcurrency = 16

	// busybox leaf image name
	parent = "c33a7c3535692e8ca015bebc3a01f7a72b14cc013918c09c40d808efe1505c62"

	DefaultService = "root:password@somehost"

	DefaultResourcePool = "/dc1/host/cluster1/Resources"
	DefaultCluster      = "/dc1/host/cluster1"
	DefaultDatacenter   = "/dc1"

	DefaultDatastore = "vsanDatastore"

	DefaultVCH = "ZzZ"

	DefaultMemoryMB = 512
)

type Config struct {
	Service string

	Concurrency int

	Start bool
	RP    bool

	Datacenter   string
	Cluster      string
	ResourcePool string

	Datastore string

	MemoryMB int64

	VCH string
}

var (
	config = Config{}
)

func init() {
	flag.StringVar(&config.Service, "service", DefaultService, "Service")

	flag.IntVar(&config.Concurrency, "concurrency", DefaultConcurrency, "Concurrency")

	flag.Int64Var(&config.MemoryMB, "memory-mb", DefaultMemoryMB, "Memory")

	flag.StringVar(&config.Datacenter, "datacenter", DefaultDatacenter, "DataCenter")
	flag.StringVar(&config.Cluster, "cluster", DefaultCluster, "Cluster")
	flag.StringVar(&config.ResourcePool, "resource-pool", DefaultResourcePool, "ResourcePool")

	flag.StringVar(&config.Datastore, "datastore", DefaultDatastore, "Datastore")

	flag.StringVar(&config.VCH, "vch", DefaultVCH, "VCH")

	flag.BoolVar(&config.Start, "start", false, "Start/Stop")
	flag.BoolVar(&config.RP, "rp", false, "force 2 Resource Pool")

	flag.Parse()

	rand.Seed(time.Now().UnixNano())
}

func main() {
	ctx := context.Background()

	// session
	c := &session.Config{
		Service:        config.Service,
		Insecure:       true,
		Keepalive:      30 * time.Minute,
		DatacenterPath: config.Datacenter,
		ClusterPath:    config.Cluster,
		PoolPath:       config.ResourcePool,
		DatastorePath:  config.Datastore,
		UserAgent:      version.UserAgent("vic-engine"),
	}

	s, err := session.NewSession(c).Connect(ctx)
	if err != nil {
		log.Panic(err)
	}
	defer s.Logout(ctx)

	if s, err = s.Populate(ctx); err != nil {
		log.Panic(err)
	}

	helper := datastore.NewHelperFromSession(ctx, s)
	p, err := datastore.PathFromString(fmt.Sprintf("[%s] %s/", config.Datastore, config.VCH))
	if err != nil {
		log.Panic(err)
	}
	helper.RootURL = *p

	var vapp *object.VirtualApp
	var pool *object.ResourcePool
	if s.IsVC() && !config.RP {
		// vapp
		vapp, err = s.Finder.VirtualApp(ctx, fmt.Sprintf("%s/%s", config.ResourcePool, config.VCH))
		if err != nil {
			log.Panic(err)
		}
	} else {
		// pool
		pool, err = s.Finder.ResourcePool(ctx, config.ResourcePool)
		if err != nil {
			log.Panic(err)
		}
	}

	// image store path
	image, err := url.Parse(config.VCH)
	if err != nil {
		log.Panic(err)
	}

	// VIC/<id>/images
	rres, err := helper.LsDirs(ctx, "VIC")
	if err != nil {
		log.Panic(err)
	}
	r := rres.HostDatastoreBrowserSearchResults

	STORE := ""
	for i := range r {
		if strings.Contains(r[i].File[0].GetFileInfo().Path, "-") {
			STORE = r[i].File[0].GetFileInfo().Path
			break
		}
	}

	// VCH/*-bootstrap.iso
	res, err := helper.Ls(ctx, "")
	if err != nil {
		log.Panic(err)
	}
	ISO := ""
	for i := range res.File {
		if strings.HasSuffix(res.File[i].GetFileInfo().Path, "-bootstrap.iso") {
			ISO = res.File[i].GetFileInfo().Path
			break
		}
	}
	if ISO == "" {
		log.Panic("Failed to find ISO file")
	}

	// bars
	progressBars, err := multibar.New()
	if err != nil {
		log.Panic(err)
	}
	progressBars.Printf("\nConcurrent testing...\n\n")

	create := progressBars.MakeBar(config.Concurrency-1, fmt.Sprintf("%16s", "Creating"))
	start := func(progress int) {}
	stop := func(progress int) {}
	if config.Start {
		start = progressBars.MakeBar(config.Concurrency-1, fmt.Sprintf("%16s", "Starting"))
		stop = progressBars.MakeBar(config.Concurrency-1, fmt.Sprintf("%16s", "Stopping"))
	}
	destroy := progressBars.MakeBar(config.Concurrency-1, fmt.Sprintf("%16s", "Destroying"))

	for i := range progressBars.Bars {
		progressBars.Bars[i].ShowTimeElapsed = false
	}

	go progressBars.Listen()

	var mu sync.Mutex
	var vms []*vm.VirtualMachine

	wrap := func(f func(i int) error, p multibar.ProgressFunc) {
		var wg sync.WaitGroup

		errs := make(chan error, config.Concurrency)
		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)

			go func(i int) {
				defer wg.Done()

				errs <- f(i)
			}(i)
		}

		go func() {
			wg.Wait()
			close(errs)
		}()

		idx := 0
		for err := range errs {
			if err != nil {
				progressBars.Printf("ERROR: %s", err)
			}
			p(idx)
			idx++
		}
	}

	createFunc := func(i int) error {
		name := fmt.Sprintf("%d-vm", i)

		specconfig := &spec.VirtualMachineConfigSpecConfig{
			NumCPUs:  1,
			MemoryMB: config.MemoryMB,

			ID:         name,
			Name:       name,
			VMFullName: name,

			ParentImageID: parent,
			BootMediaPath: fmt.Sprintf("[%s] %s/%s", config.Datastore, config.VCH, ISO),
			VMPathName:    fmt.Sprintf("[%s]", config.Datastore),

			ImageStoreName: STORE,
			ImageStorePath: image,
		}

		// Create a linux guest
		linux, err := guest.NewLinuxGuest(ctx, s, specconfig)
		if err != nil {
			return err
		}
		h := linux.Spec().Spec()

		var res *types.TaskInfo
		if s.IsVC() && !config.RP {
			res, err = tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
				return vapp.CreateChildVM(ctx, *h, nil)
			})
			if err != nil {
				return err

			}
		} else {
			res, err = tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
				return s.VMFolder.CreateVM(ctx, *h, pool, nil)
			})
			if err != nil {
				return err
			}
		}
		mu.Lock()
		vms = append(vms, vm.NewVirtualMachine(ctx, s, res.Result.(types.ManagedObjectReference)))
		mu.Unlock()

		return nil
	}
	wrap(createFunc, create)

	if config.Start {
		startFunc := func(i int) error {
			_, err := tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
				return vms[i].PowerOn(ctx)
			})
			return err
		}
		wrap(startFunc, start)

		stopFunc := func(i int) error {
			_, err := tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
				return vms[i].PowerOff(ctx)
			})
			return err
		}
		wrap(stopFunc, stop)
	}

	destroyFunc := func(i int) error {
		v, err := vms[i].VMPathName(ctx)
		if err != nil {
			return err
		}

		concurrent := false
		// if DeleteExceptDisks succeeds on VC, it leaves the VM orphan so we need to call Unregister
		// if DeleteExceptDisks succeeds on ESXi, no further action needed
		// if DeleteExceptDisks fails, we should call Unregister and only return an error if that fails too
		//		Unregister sometimes can fail with ManagedObjectNotFound so we ignore it
		_, err = vms[i].DeleteExceptDisks(ctx)
		if err != nil {
			switch f := err.(type) {
			case task.Error:
				switch f.Fault().(type) {
				case *types.ConcurrentAccess:
					log.Printf("DeleteExceptDisks failed for %d with ConcurrentAccess error. Ignoring it", i)
					concurrent = true
				}
				// err but not concurrent
				if !concurrent {
					return err
				}
			}
		}
		if concurrent && vms[i].IsVC() {
			if err := vms[i].Unregister(ctx); err != nil {
				if !tasks.IsNotFoundError(err) && !tasks.IsConcurrentAccessError(err) {
					return err
				}
			}
		}

		fm := s.Datastore.NewFileManager(s.Datacenter, true)
		// remove from datastore
		if err := fm.Delete(ctx, path.Dir(v)); err != nil {
			return err
		}

		return nil
	}
	wrap(destroyFunc, destroy)
}
