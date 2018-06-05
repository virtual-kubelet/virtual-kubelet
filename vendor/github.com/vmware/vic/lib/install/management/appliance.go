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

package management

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/diag"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	portLayerPort = constants.SerialOverLANPort

	// this is generated in the crypto/tls.alert code
	badTLSCertificate = "tls: bad certificate"

	// This is a constant also used in the lib/apiservers/engine/backends/system.go to assign custom info the docker types.info struct
	volumeStoresID = "VolumeStores"
)

var (
	lastSeenProgressMessage string
	unitNumber              int32
)

func (d *Dispatcher) isVCH(vm *vm.VirtualMachine) (bool, error) {
	if vm == nil {
		return false, errors.New("nil parameter")
	}
	defer trace.End(trace.Begin(vm.InventoryPath))

	info, err := vm.FetchExtraConfig(d.op)
	if err != nil {
		err = errors.Errorf("Failed to fetch guest info of appliance vm: %s", err)
		return false, err
	}

	var remoteConf config.VirtualContainerHostConfigSpec
	extraconfig.Decode(extraconfig.MapSource(info), &remoteConf)

	// if the moref of the target matches where we expect to find it for a VCH, run with it
	if remoteConf.ExecutorConfig.ID == vm.Reference().String() || remoteConf.IsCreating() {
		return true, nil
	}

	return false, nil
}

func (d *Dispatcher) isContainerVM(vm *vm.VirtualMachine) (bool, error) {
	if vm == nil {
		return false, errors.New("nil parameter")
	}
	defer trace.End(trace.Begin(vm.InventoryPath))
	var cspec executor.ExecutorConfig
	info, err := vm.FetchExtraConfig(d.op)
	if err != nil {
		err = errors.Errorf("Failed to fetch guest info of appliance vm: %s", err)
		return false, err
	}
	extraconfig.Decode(extraconfig.MapSource(info), &cspec)
	if cspec.Version == nil {
		return false, nil
	}
	return true, nil
}

func (d *Dispatcher) checkExistence(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) error {
	defer trace.End(trace.Begin(""))
	var err error
	var orp *object.ResourcePool
	if orp, err = d.findResourcePool(d.vchPoolPath); err != nil {
		return err
	}
	if orp == nil {
		return nil
	}

	rp := compute.NewResourcePool(d.op, d.session, orp.Reference())
	vm, err := rp.GetChildVM(d.op, conf.Name)
	if err != nil {
		return err
	}
	if vm == nil {
		return nil
	}

	d.op.Debug("Appliance is found")
	if ok, verr := d.isVCH(vm); !ok {
		verr = errors.Errorf("Found virtual machine %q, but it is not a VCH. Please choose a different virtual app.", conf.Name)
		return verr
	}
	err = errors.Errorf("A VCH with the name %q already exists. Please choose a different name before attempting another install", conf.Name)
	return err
}

func (d *Dispatcher) deleteVM(vm *vm.VirtualMachine, force bool) error {
	defer trace.End(trace.Begin(fmt.Sprintf("vm %q, force %t", vm.String(), force)))

	var err error
	power, err := vm.PowerState(d.op)
	if err != nil || power != types.VirtualMachinePowerStatePoweredOff {
		if err != nil {
			d.op.Warnf("Failed to get vm power status %q: %s", vm.Reference(), err)
		}
		if !force {
			if err != nil {
				return err
			}
			name, _ := vm.ObjectName(d.op)
			if name != "" {
				err = errors.Errorf("VM %q is powered on", name)
			} else {
				err = errors.Errorf("VM %q is powered on", vm.Reference())
			}
			return err
		}
		if _, err = vm.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
			return vm.PowerOff(ctx)
		}); err != nil {
			d.op.Debugf("Failed to power off existing appliance for %s, try to remove anyway", err)
		}
	}

	// get the actual folder name before we delete it
	folder, err := vm.DatastoreFolderName(d.op)
	if err != nil {
		// failed to get folder name, might not be able to remove files for this VM
		name, _ := vm.ObjectName(d.op)
		if name == "" {
			d.op.Errorf("Unable to automatically remove all files in datastore for VM %q", vm.Reference())
		} else {
			// try to use the vm name in place of folder
			d.op.Infof("Delete will attempt to remove datastore files for VM %q", name)
			folder = name
		}
	}

	// Power off the VM if necessary
	retryErrHandler := func(err error) bool {
		if vm.IsInvalidPowerStateError(err) {
			_, terr := vm.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
				return vm.PowerOff(ctx)
			})

			if terr == nil || tasks.IsTransientError(d.op, terr) || vm.IsInvalidPowerStateError(terr) {
				return true
			}
		}

		return tasks.IsTransientError(d.op, err) || tasks.IsConcurrentAccessError(err)
	}

	// Only retry VM destroy on ConcurrentAccess error
	err = retry.Do(func() error {
		_, err := vm.DeleteExceptDisks(d.op)
		return err
	}, retryErrHandler)

	if err != nil {
		d.op.Warnf("Destroy VM %s failed with %s, unregister the VM instead", vm.Reference(), err)

		err = retry.Do(func() error {
			return vm.Unregister(d.op)
		}, retryErrHandler)

		if err != nil {
			d.op.Errorf("Unregister the VM failed: %s", err)
			return err
		}
	}

	if _, err = d.deleteDatastoreFiles(d.session.Datastore, folder, true); err != nil {
		d.op.Warnf("Failed to remove datastore files for VM %s with folder path %s: %s", vm.Reference(), folder, err)
	}

	return nil
}

func (d *Dispatcher) addNetworkDevices(conf *config.VirtualContainerHostConfigSpec, cspec *spec.VirtualMachineConfigSpec, devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	defer trace.End(trace.Begin(""))

	// network name:alias, to avoid create multiple devices for same network
	slots := make(map[int32]bool)
	nets := make(map[string]*executor.NetworkEndpoint)

	for name, endpoint := range conf.ExecutorConfig.Networks {
		if pnic, ok := nets[endpoint.Network.Common.ID]; ok {
			// there's already a NIC on this network
			endpoint.Common.ID = pnic.Common.ID
			d.op.Infof("Network role %q is sharing NIC with %q", name, pnic.Network.Common.Name)
			continue
		}

		moref := new(types.ManagedObjectReference)
		if ok := moref.FromString(endpoint.Network.ID); !ok {
			return nil, fmt.Errorf("serialized managed object reference in unexpected format: %q", endpoint.Network.ID)
		}
		obj, err := d.session.Finder.ObjectReference(d.op, *moref)
		if err != nil {
			return nil, fmt.Errorf("unable to reacquire reference for network %q from serialized form: %q", endpoint.Network.Name, endpoint.Network.ID)
		}
		network, ok := obj.(object.NetworkReference)
		if !ok {
			return nil, fmt.Errorf("reacquired reference for network %q, from serialized form %q, was not a network: %T", endpoint.Network.Name, endpoint.Network.ID, obj)
		}

		backing, err := network.EthernetCardBackingInfo(d.op)
		if err != nil {
			err = errors.Errorf("Failed to get network backing info for %q: %s", network, err)
			return nil, err
		}

		nic, err := devices.CreateEthernetCard("vmxnet3", backing)
		if err != nil {
			err = errors.Errorf("Failed to create Ethernet Card spec for %s", err)
			return nil, err
		}

		slot := cspec.AssignSlotNumber(nic, slots)
		if slot == constants.NilSlot {
			err = errors.Errorf("Failed to assign stable PCI slot for %q network card", name)
		}

		endpoint.Common.ID = strconv.Itoa(int(slot))
		slots[slot] = true
		d.op.Debugf("Setting %q to slot %d", name, slot)

		devices = append(devices, nic)

		nets[endpoint.Network.Common.ID] = endpoint
	}
	return devices, nil
}

func (d *Dispatcher) addIDEController(devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	defer trace.End(trace.Begin(""))

	// IDE controller
	scsi, err := devices.CreateIDEController()
	if err != nil {
		return nil, err
	}
	devices = append(devices, scsi)
	return devices, nil
}

func (d *Dispatcher) addParaVirtualSCSIController(devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	defer trace.End(trace.Begin(""))

	// para virtual SCSI controller
	scsi, err := devices.CreateSCSIController("pvscsi")
	if err != nil {
		return nil, err
	}
	devices = append(devices, scsi)
	return devices, nil
}

func (d *Dispatcher) createApplianceSpec(conf *config.VirtualContainerHostConfigSpec, vConf *data.InstallerData) (*types.VirtualMachineConfigSpec, error) {
	defer trace.End(trace.Begin(""))

	var devices object.VirtualDeviceList
	var err error
	var cpus int32   // appliance number of CPUs
	var memory int64 // appliance memory in MB

	// set to creating VCH
	conf.SetIsCreating(true)

	cfg, err := d.encodeConfig(conf)
	if err != nil {
		return nil, err
	}

	if vConf.ApplianceSize.CPU.Limit != nil {
		cpus = int32(*vConf.ApplianceSize.CPU.Limit)
	}
	if vConf.ApplianceSize.Memory.Limit != nil {
		memory = *vConf.ApplianceSize.Memory.Limit
	}

	spec := &spec.VirtualMachineConfigSpec{
		VirtualMachineConfigSpec: &types.VirtualMachineConfigSpec{
			Name:               conf.Name,
			GuestId:            string(types.VirtualMachineGuestOsIdentifierOtherGuest64),
			AlternateGuestName: constants.DefaultAltVCHGuestName(),
			Files:              &types.VirtualMachineFileInfo{VmPathName: fmt.Sprintf("[%s]", conf.ImageStores[0].Host)},
			NumCPUs:            cpus,
			MemoryMB:           memory,
			// Encode the config both here and after the VMs created so that it can be identified as a VCH appliance as soon as
			// creation is complete.
			ExtraConfig: append(vmomi.OptionValueFromMap(cfg, true), &types.OptionValue{Key: "answer.msg.serial.file.open", Value: "Append"}),
		},
	}

	if devices, err = d.addIDEController(devices); err != nil {
		return nil, err
	}

	if devices, err = d.addParaVirtualSCSIController(devices); err != nil {
		return nil, err
	}

	if devices, err = d.addNetworkDevices(conf, spec, devices); err != nil {
		return nil, err
	}

	deviceChange, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return nil, err
	}

	spec.DeviceChange = deviceChange
	return spec.VirtualMachineConfigSpec, nil
}

func isManagedObjectNotFoundError(err error) bool {
	if soap.IsSoapFault(err) {
		_, ok := soap.ToSoapFault(err).VimFault().(types.ManagedObjectNotFound)
		return ok
	}

	return false
}

func (d *Dispatcher) findApplianceByID(conf *config.VirtualContainerHostConfigSpec) (*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(""))

	var err error
	var vmm *vm.VirtualMachine

	moref := new(types.ManagedObjectReference)
	if ok := moref.FromString(conf.ID); !ok {
		message := "Failed to get appliance VM mob reference"
		d.op.Error(message)
		return nil, errors.New(message)
	}
	ref, err := d.session.Finder.ObjectReference(d.op, *moref)
	if err != nil {
		if !isManagedObjectNotFoundError(err) {
			err = errors.Errorf("Failed to query appliance (%q): %s", moref, err)
			return nil, err
		}
		d.op.Debug("Appliance is not found")
		return nil, nil

	}
	ovm, ok := ref.(*object.VirtualMachine)
	if !ok {
		d.op.Errorf("Failed to find VM %q: %s", moref, err)
		return nil, err
	}
	vmm = vm.NewVirtualMachine(d.op, d.session, ovm.Reference())

	element, err := d.session.Finder.Element(d.op, vmm.Reference())
	if err != nil {
		return nil, err
	}
	vmm.SetInventoryPath(element.Path)
	return vmm, nil
}

func (d *Dispatcher) configIso(conf *config.VirtualContainerHostConfigSpec, vm *vm.VirtualMachine, settings *data.InstallerData) (object.VirtualDeviceList, error) {
	defer trace.End(trace.Begin(""))

	var devices object.VirtualDeviceList
	var err error

	vmDevices, err := vm.Device(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm devices for appliance: %s", err)
		return nil, err
	}
	ide, err := vmDevices.FindIDEController("")
	if err != nil {
		d.op.Errorf("Failed to find IDE controller for appliance: %s", err)
		return nil, err
	}
	cdrom, err := devices.CreateCdrom(ide)
	if err != nil {
		d.op.Errorf("Failed to create Cdrom device for appliance: %s", err)
		return nil, err
	}
	cdrom = devices.InsertIso(cdrom, fmt.Sprintf("[%s] %s/%s", conf.ImageStores[0].Host, d.vmPathName, settings.ApplianceISO))
	devices = append(devices, cdrom)
	return devices, nil
}

func (d *Dispatcher) configLogging(conf *config.VirtualContainerHostConfigSpec, vm *vm.VirtualMachine, settings *data.InstallerData) (object.VirtualDeviceList, error) {
	defer trace.End(trace.Begin(""))

	devices, err := vm.Device(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm devices for appliance: %s", err)
		return nil, err
	}

	p, err := devices.CreateSerialPort()
	if err != nil {
		return nil, err
	}

	err = vm.AddDevice(d.op, p)
	if err != nil {
		return nil, err
	}

	devices, err = vm.Device(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm devices for appliance: %s", err)
		return nil, err
	}

	serial, err := devices.FindSerialPort("")
	if err != nil {
		d.op.Errorf("Failed to locate serial port for persistent log configuration: %s", err)
		return nil, err
	}

	// TODO: we need to add an accessor for generating paths within the VM directory
	vmx, err := vm.VMPathName(d.op)
	if err != nil {
		d.op.Errorf("Unable to determine path of appliance VM: %s", err)
		return nil, err
	}

	// TODO: move this construction into the spec package and update portlayer/logging to use it as well
	serial.Backing = &types.VirtualSerialPortFileBackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			// name consistency with containerVM
			FileName: fmt.Sprintf("%s/tether.debug", path.Dir(vmx)),
		},
	}

	return []types.BaseVirtualDevice{serial}, nil
}

func (d *Dispatcher) setDockerPort(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) {
	if conf.HostCertificate != nil {
		d.DockerPort = fmt.Sprintf("%d", opts.DefaultTLSHTTPPort)
	} else {
		d.DockerPort = fmt.Sprintf("%d", opts.DefaultHTTPPort)
	}
}

func (d *Dispatcher) createAppliance(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) error {
	defer trace.End(trace.Begin(""))
	d.op.Info("Creating appliance on target")

	spec, err := d.createApplianceSpec(conf, settings)
	if err != nil {
		d.op.Errorf("Unable to create appliance spec: %s", err)
		return err
	}

	// Create the VCH inventory folder
	if d.isVC {
		d.op.Info("Creating the VCH folder")
		// update the session pointer with the VCH Folder
		d.session.VCHFolder, err = d.session.VMFolder.CreateFolder(d.op, spec.Name)
		if err != nil {
			if soap.IsSoapFault(err) {
				switch soap.ToSoapFault(err).VimFault().(type) {
				case types.DuplicateName:
					return fmt.Errorf("The specified VCH name (%s) is already in use", conf.Name)
				}
			}
			return fmt.Errorf("Unable to create the VCH Folder(%s): %s", conf.Name, err)
		}
	}

	d.op.Info("Creating the VCH VM")
	info, err := tasks.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return d.session.VCHFolder.CreateVM(ctx, *spec, d.vchPool, d.session.Host)
	})

	if err != nil {
		d.op.Errorf("Unable to create the appliance VM: %s", err)
		return err
	}
	if info.Error != nil || info.State != types.TaskInfoStateSuccess {
		d.op.Errorf("Create appliance reported: %s", info.Error.LocalizedMessage)
	}

	if err = d.createVolumeStores(conf); err != nil {
		return errors.Errorf("Exiting because we could not create volume stores due to error: %s", err)
	}

	// get VM reference and save it
	moref := info.Result.(types.ManagedObjectReference)
	conf.SetMoref(&moref)
	obj, err := d.session.Finder.ObjectReference(d.op, moref)
	if err != nil {
		d.op.Errorf("Failed to reacquire reference to appliance VM after creation: %s", err)
		return err
	}
	gvm, ok := obj.(*object.VirtualMachine)
	if !ok {
		return fmt.Errorf("Required reference after appliance creation was not for a VM: %T", obj)
	}
	vm2 := vm.NewVirtualMachineFromVM(d.op, d.session, gvm)

	vm2.DisableDestroy(d.op)

	// update the displayname to the actual folder name used
	if d.vmPathName, err = vm2.DatastoreFolderName(d.op); err != nil {
		d.op.Errorf("Failed to get canonical name for appliance: %s", err)
		return err
	}
	d.op.Debugf("vm folder name: %q", d.vmPathName)
	d.op.Debugf("vm inventory path: %q", vm2.InventoryPath)

	vicadmin := executor.Cmd{
		Path: "/sbin/vicadmin",
		Args: []string{
			"/sbin/vicadmin",
			"--dc=" + settings.DatacenterName,
			"--pool=" + settings.ResourcePoolPath,
			"--cluster=" + settings.ClusterPath,
		},
		Env: []string{
			"PATH=/sbin:/bin",
			"GOTRACEBACK=all",
		},
		Dir: "/home/vicadmin",
	}
	if settings.HTTPProxy != nil {
		vicadmin.Env = append(vicadmin.Env, fmt.Sprintf("%s=%s", config.VICAdminHTTPProxy, settings.HTTPProxy.String()))
	}
	if settings.HTTPSProxy != nil {
		vicadmin.Env = append(vicadmin.Env, fmt.Sprintf("%s=%s", config.VICAdminHTTPSProxy, settings.HTTPSProxy.String()))
	}

	conf.AddComponent(config.VicAdminService, &executor.SessionConfig{
		User:    "vicadmin",
		Group:   "vicadmin",
		Cmd:     vicadmin,
		Restart: true,
		Active:  true,
	},
	)

	d.setDockerPort(conf, settings)

	personality := executor.Cmd{
		Path: "/sbin/docker-engine-server",
		Args: []string{
			"/sbin/docker-engine-server",
			//FIXME: hack during config migration
			"-port=" + d.DockerPort,
			fmt.Sprintf("-port-layer-port=%d", portLayerPort),
		},
		Env: []string{
			"PATH=/sbin",
			"GOTRACEBACK=all",
		},
	}
	if settings.HTTPProxy != nil {
		personality.Env = append(personality.Env, fmt.Sprintf("%s=%s", config.GeneralHTTPProxy, settings.HTTPProxy.String()))
	}
	if settings.HTTPSProxy != nil {
		personality.Env = append(personality.Env, fmt.Sprintf("%s=%s", config.GeneralHTTPSProxy, settings.HTTPSProxy.String()))
	}

	conf.AddComponent(config.PersonaService, &executor.SessionConfig{
		// currently needed for iptables interaction
		// User:  "nobody",
		// Group: "nobody",
		Cmd:     personality,
		Restart: true,
		Active:  true,
	},
	)

	// Kubelet
	if conf.KubeletConfigFile != "" {
		kubeletName := vm2.Name()
		kubeletStarter := executor.Cmd{
			Path: "/sbin/kubelet-starter",
			Args: []string{
				"/sbin/kubelet-starter",
			},
		}

		kubeletStarter.Env = append(kubeletStarter.Env, fmt.Sprintf("%s=%s", "KUBELET_NAME", kubeletName))

		// Set up the persona and port layer
		kubeletStarter.Env = append(kubeletStarter.Env, fmt.Sprintf("%s=%s", "PERSONA_PORT", d.DockerPort))
		kubeletStarter.Env = append(kubeletStarter.Env, fmt.Sprintf("%s=%d", "PORTLAYER_PORT", portLayerPort))

		if settings.HTTPProxy != nil {
			kubeletStarter.Env = append(kubeletStarter.Env, fmt.Sprintf("%s=%s", config.GeneralHTTPProxy, settings.HTTPProxy.String()))
		}
		if settings.HTTPSProxy != nil {
			kubeletStarter.Env = append(kubeletStarter.Env, fmt.Sprintf("%s=%s", config.GeneralHTTPSProxy, settings.HTTPSProxy.String()))
		}

		conf.AddComponent(config.KubeletStarterService, &executor.SessionConfig{
			Cmd:     kubeletStarter,
			Restart: true,
			Active:  true,
		},
		)
	}

	cfg := &executor.SessionConfig{
		Cmd: executor.Cmd{
			Path: "/sbin/port-layer-server",
			Args: []string{
				"/sbin/port-layer-server",
				"--host=localhost",
				fmt.Sprintf("--port=%d", portLayerPort),
			},
			Env: []string{
				//FIXME: hack during config migration
				"VC_URL=" + conf.Target,
				"DC_PATH=" + settings.DatacenterName,
				"CS_PATH=" + settings.ClusterPath,
				"POOL_PATH=" + settings.ResourcePoolPath,
				"DS_PATH=" + conf.ImageStores[0].Host,
			},
		},
		Restart: true,
		Active:  true,
	}

	conf.AddComponent(config.PortLayerService, cfg)

	// fix up those parts of the config that depend on the final applianceVM folder name
	conf.BootstrapImagePath = fmt.Sprintf("[%s] %s/%s", conf.ImageStores[0].Host, d.vmPathName, settings.BootstrapISO)

	if len(conf.ImageStores[0].Path) == 0 {
		conf.ImageStores[0].Path = d.vmPathName
	}

	// apply the fixed-up configuration
	spec, err = d.reconfigureApplianceSpec(vm2, conf, settings)
	if err != nil {
		d.op.Errorf("Error while getting appliance reconfig spec: %s", err)
		return err
	}

	// reconfig
	info, err = vm2.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return vm2.Reconfigure(ctx, *spec)
	})

	if err != nil {
		d.op.Errorf("Error while setting component parameters to appliance: %s", err)
		return err
	}
	if info.State != types.TaskInfoStateSuccess {
		d.op.Errorf("Setting parameters to appliance reported: %s", info.Error.LocalizedMessage)
		return err
	}

	d.appliance = vm2
	return nil
}

func (d *Dispatcher) encodeConfig(conf *config.VirtualContainerHostConfigSpec) (map[string]string, error) {
	d.op.Debug("generating new config secret key")

	s, err := extraconfig.NewSecretKey()
	if err != nil {
		return nil, err
	}
	d.secret = s

	cfg := make(map[string]string)
	extraconfig.Encode(d.secret.Sink(extraconfig.MapSink(cfg)), conf)
	return cfg, nil
}

func (d *Dispatcher) decryptVCHConfig(vm *vm.VirtualMachine, cfg map[string]string) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin(""))

	if d.secret == nil {
		name, err := vm.ObjectName(d.op)
		if err != nil {
			err = errors.Errorf("Failed to get vm name %q: %s", vm.Reference(), err)
			return nil, err
		}
		// set session datastore to where the VM is running
		ds, err := d.getImageDatastore(vm, nil, true)
		if err != nil {
			err = errors.Errorf("Failure finding image store from VCH VM %q: %s", name, err)
			return nil, err
		}
		path, err := vm.DatastoreFolderName(d.op)
		if err != nil {
			err = errors.Errorf("Failed to get VM %q datastore path: %s", name, err)
			return nil, err
		}
		s, err := d.GuestInfoSecret(name, path, ds)
		if err != nil {
			return nil, err
		}
		d.secret = s
	}

	conf := &config.VirtualContainerHostConfigSpec{}
	extraconfig.Decode(d.secret.Source(extraconfig.MapSource(cfg)), conf)
	return conf, nil
}

func (d *Dispatcher) reconfigureApplianceSpec(vm *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) (*types.VirtualMachineConfigSpec, error) {
	defer trace.End(trace.Begin(""))

	var devices object.VirtualDeviceList
	var err error

	spec := &types.VirtualMachineConfigSpec{}

	// create new devices
	if devices, err = d.configIso(conf, vm, settings); err != nil {
		return nil, err
	}

	newDevices, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		d.op.Errorf("Failed to create config spec for appliance: %s", err)
		return nil, err
	}

	spec.DeviceChange = newDevices

	// update existing devices
	if devices, err = d.configLogging(conf, vm, settings); err != nil {
		return nil, err
	}

	updateDevices, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationEdit)
	if err != nil {
		d.op.Errorf("Failed to create config spec for logging update: %s", err)
		return nil, err
	}

	spec.DeviceChange = append(spec.DeviceChange, updateDevices...)

	cfg, err := d.encodeConfig(conf)
	if err != nil {
		return nil, err
	}

	spec.ExtraConfig = append(spec.ExtraConfig, vmomi.OptionValueFromMap(cfg, true)...)
	return spec, nil
}

// applianceConfiguration updates the configuration passed in with the latest from the appliance VM.
// there's no guarantee of consistency within the configuration at this time
func (d *Dispatcher) applianceConfiguration(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(""))

	extraConfig, err := d.appliance.FetchExtraConfig(d.op)
	if err != nil {
		return err
	}

	extraconfig.Decode(extraconfig.MapSource(extraConfig), conf)
	return nil
}

// waitForKey squashes the return values and simpy blocks until the key is updated or there is an error
func (d *Dispatcher) waitForKey(key string) {
	defer trace.End(trace.Begin(key))

	d.appliance.WaitForKeyInExtraConfig(d.op, key)
	return
}

// isPortLayerRunning decodes the `docker info` response to check if the portlayer is running
func isPortLayerRunning(op trace.Operation, res *http.Response, conf *config.VirtualContainerHostConfigSpec) bool {
	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		op.Debugf("error while reading res body: %s", err.Error())
		return false
	}

	var sysInfo dockertypes.Info
	if err = json.Unmarshal(resBody, &sysInfo); err != nil {
		op.Debugf("error while unmarshalling res body: %s", err.Error())
		return false
	}
	// At this point the portlayer is up successfully. However, we need to report the Volume Stores that were not created successfully.
	volumeStoresLine := ""

	for _, value := range sysInfo.SystemStatus {
		if value[0] == volumeStoresID {
			op.Debugf("Portlayer has established volume stores (%s)", value[1])
			volumeStoresLine = value[1]
			break
		}
	}

	allVolumeStoresPresent := confirmVolumeStores(op, conf, volumeStoresLine)
	if !allVolumeStoresPresent {
		op.Error("Not all configured volume stores are online - check port layer log via vicadmin")
	}

	for _, status := range sysInfo.SystemStatus {
		if status[0] == sysInfo.Driver {
			return status[1] == "RUNNING"
		}
	}

	return false
}

// confirmVolumeStores is a helper function that will log and warn the vic-machine user if some of their volumestores did not present in the portlayer
func confirmVolumeStores(op trace.Operation, conf *config.VirtualContainerHostConfigSpec, rawVolumeStores string) bool {
	establishedVolumeStores := make(map[string]struct{})

	splitStores := strings.Split(rawVolumeStores, " ")
	for _, v := range splitStores {
		establishedVolumeStores[v] = struct{}{}
	}

	result := true
	for k := range conf.VolumeLocations {
		if _, ok := establishedVolumeStores[k]; !ok {
			op.Errorf("VolumeStore (%s) cannot be brought online - check network, nfs server, and --volume-store configurations", k)
			result = false
		}
	}
	return result
}

// CheckDockerAPI checks if the appliance components are initialized by issuing
// `docker info` to the appliance
func (d *Dispatcher) CheckDockerAPI(conf *config.VirtualContainerHostConfigSpec, clientCert *tls.Certificate) error {
	defer trace.End(trace.Begin(""))

	var (
		proto          string
		client         *http.Client
		res            *http.Response
		err            error
		req            *http.Request
		tlsErrExpected bool
	)

	if conf.HostCertificate.IsNil() {
		// TLS disabled
		proto = "http"
		client = &http.Client{}
	} else {
		// TLS enabled
		proto = "https"

		// #nosec: TLS InsecureSkipVerify set true
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		// appliance is configured for tlsverify, but we don't have a client certificate
		if len(conf.CertificateAuthorities) > 0 {
			// if tlsverify was configured at all then we must verify the remote
			tr.TLSClientConfig.InsecureSkipVerify = false

			func() {
				d.op.Debug("Loading CAs for client auth")
				pool, err := x509.SystemCertPool()
				if err != nil {
					d.op.Warn("Unable to load system root certificates - continuing with only the provided CA")
					pool = x509.NewCertPool()
				}

				if !pool.AppendCertsFromPEM(conf.CertificateAuthorities) {
					d.op.Warn("Unable add CAs from config to validation pool")
				}

				// tr.TLSClientConfig.ClientCAs = pool
				tr.TLSClientConfig.RootCAs = pool

				if clientCert == nil {
					// we know this will fail, but we can try to distinguish the expected error vs
					// unresponsive endpoint
					tlsErrExpected = true
					d.op.Debug("CA configured on appliance but no client certificate available")
					return
				}

				cert, err := conf.HostCertificate.X509Certificate()
				if err != nil {
					d.op.Debug("Unable to extract host certificate: %s", err)
					tlsErrExpected = true
					return
				}

				cip := net.ParseIP(d.HostIP)
				if err != nil {
					d.op.Debug("Unable to process Docker API host address from %q: %s", d.HostIP, err)
					tlsErrExpected = true
					return
				}

				// find the name to use and override the IP if found
				addr, err := addrToUse(d.op, []net.IP{cip}, cert, conf.CertificateAuthorities)
				if err != nil {
					d.op.Debug("Unable to determine address to use with remote certificate, checking SANs")
					// #nosec: Errors unhandled .
					addr, _ = viableHostAddress(d.op, []net.IP{cip}, cert, conf.CertificateAuthorities)
					d.op.Debugf("Using host address: %s", addr)
				}
				if addr != "" {
					d.HostIP = addr
				} else {
					d.op.Debug("Failed to find a viable address for Docker API from certificates")
					// Server certificate won't validate since we don't have a hostname
					tlsErrExpected = true
				}
				d.op.Debugf("Host address set to: %q", d.HostIP)
			}()
		}

		if clientCert != nil {
			d.op.Debug("Assigning certificates for client auth")
			tr.TLSClientConfig.Certificates = []tls.Certificate{*clientCert}
		}

		client = &http.Client{Transport: tr}
	}

	dockerInfoURL := fmt.Sprintf("%s://%s:%s/info", proto, d.HostIP, d.DockerPort)
	d.op.Debugf("Docker API endpoint: %s", dockerInfoURL)
	req, err = http.NewRequest("GET", dockerInfoURL, nil)
	if err != nil {
		return errors.New("invalid HTTP request for docker info")
	}
	req = req.WithContext(d.op)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		res, err = client.Do(req)
		if err == nil {
			if res.StatusCode == http.StatusOK {
				if isPortLayerRunning(d.op, res, conf) {
					d.op.Debug("Confirmed port layer is operational")
					break
				}
			}

			d.op.Debugf("Received HTTP status %d: %s", res.StatusCode, res.Status)
		} else {
			// DEBU[2016-10-11T22:22:38Z] Error received from endpoint: Get https://192.168.78.127:2376/info: dial tcp 192.168.78.127:2376: getsockopt: connection refused &{%!t(string=Get) %!t(string=https://192.168.78.127:2376/info) %!t(*net.OpError=&{dial tcp <nil> 0xc4204505a0 0xc4203a5e00})}
			// DEBU[2016-10-11T22:22:39Z] Components not yet initialized, retrying
			// ERR=&url.Error{
			//     Op:  "Get",
			//     URL: "https://192.168.78.127:2376/info",
			//     Err: &net.OpError{
			//         Op:     "dial",
			//         Net:    "tcp",
			//         Source: nil,
			//         Addr:   &net.TCPAddr{
			//             IP:   {0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xc0, 0xa8, 0x4e, 0x7f},
			//             Port: 2376,
			//             Zone: "",
			//         },
			//         Err: &os.SyscallError{
			//             Syscall: "getsockopt",
			//             Err:     syscall.Errno(0x6f),
			//         },
			//     },
			// }
			// DEBU[2016-10-11T22:22:41Z] Error received from endpoint: Get https://192.168.78.127:2376/info: remote error: tls: bad certificate &{%!t(string=Get) %!t(string=https://192.168.78.127:2376/info) %!t(*net.OpError=&{remote error  <nil> <nil> 42})}
			// DEBU[2016-10-11T22:22:42Z] Components not yet initialized, retrying
			// ERR=&url.Error{
			//     Op:  "Get",
			//     URL: "https://192.168.78.127:2376/info",
			//     Err: &net.OpError{
			//         Op:     "remote error",
			//         Net:    "",
			//         Source: nil,
			//         Addr:   nil,
			//         Err:    tls.alert(0x2a),
			//     },
			// }

			// ECONNREFUSED: 111, 0x6f

			uerr, ok := err.(*url.Error)
			if ok {
				switch neterr := uerr.Err.(type) {
				case *net.OpError:
					switch root := neterr.Err.(type) {
					case *os.SyscallError:
						if root.Err == syscall.Errno(syscall.ECONNREFUSED) {
							// waiting for API server to start
							d.op.Debug("connection refused")
						} else {
							d.op.Debugf("Error was expected to be ECONNREFUSED: %#v", root.Err)
						}
					default:
						errmsg := root.Error()

						if tlsErrExpected {
							d.op.Warnf("Expected TLS error without access to client certificate, received error: %s", errmsg)
							return nil
						}

						// the TLS package doesn't expose the raw reason codes
						// but we're actually looking for alertBadCertificate (42)
						if errmsg == badTLSCertificate {
							// TODO: programmatic check for clock skew on host
							d.op.Error("Connection failed with TLS error \"bad certificate\" - check for clock skew on the host")
						} else {
							d.op.Errorf("Connection failed with error: %s", root)
						}

						return fmt.Errorf("failed to connect to %s: %s", dockerInfoURL, root)
					}

				case x509.UnknownAuthorityError:
					// This will occur if the server certificate was signed by a CA that is not the one used for client authentication
					// and does not have a trusted root registered on the system running vic-machine
					msg := fmt.Sprintf("Unable to validate server certificate with configured CAs (unknown CA): %s", neterr.Error())
					if tlsErrExpected {
						// Legitimate deployment so no error, but definitely requires a warning.
						d.op.Warn(msg)
						return nil
					}
					// TLS error not expected, the validation failure is a problem
					d.op.Error(msg)
					return neterr

				case x509.HostnameError:
					// e.g. "doesn't contain any IP SANs"
					msg := fmt.Sprintf("Server certificate hostname doesn't match: %s", neterr.Error())
					if tlsErrExpected {
						d.op.Warn(msg)
						return nil
					}
					d.op.Error(msg)
					return neterr

				default:
					d.op.Debugf("Unhandled net error type: %#v", neterr)
					return neterr
				}
			} else {
				d.op.Debugf("Error type was expected to be url.Error: %#v", err)
			}
		}

		select {
		case <-ticker.C:
		case <-d.op.Done():
			return d.op.Err()
		}

		d.op.Debug("Components not yet initialized, retrying")
	}

	return nil
}

// ensureApplianceInitializes checks if the appliance component processes are launched correctly
func (d *Dispatcher) ensureApplianceInitializes(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(""))

	if d.appliance == nil {
		return errors.New("cannot validate appliance due to missing VM reference")
	}

	d.op.Info("Waiting for IP information")
	d.waitForKey(extraconfig.CalculateKeys(conf, "ExecutorConfig.Networks.client.Assigned.IP", "")[0])
	ctxerr := d.op.Err()

	if ctxerr == nil {
		d.op.Info("Waiting for major appliance components to launch")
		for _, k := range extraconfig.CalculateKeys(conf, "ExecutorConfig.Sessions.*.Started", "") {
			d.waitForKey(k)
		}
	}

	// at this point either everything has succeeded or we're going into diagnostics, ignore error
	// as we're only using it for IP in the success case
	updateErr := d.applianceConfiguration(conf)

	// confirm components launched correctly
	d.op.Debug("  State of components:")
	for name, session := range conf.ExecutorConfig.Sessions {
		status := "waiting to launch"
		if session.Started == "true" {
			status = "started successfully"
		} else if session.Started != "" {
			status = session.Started
			d.op.Errorf("  Component did not launch successfully - %s: %s", name, status)
		}

		d.op.Debugf("    %q: %q", name, status)
	}

	// TODO: we should call to the general vic-machine inspect implementation here for more detail
	// but instead...
	if !ip.IsUnspecifiedIP(conf.ExecutorConfig.Networks["client"].Assigned.IP) {
		d.HostIP = conf.ExecutorConfig.Networks["client"].Assigned.IP.String()
		d.op.Infof("Obtained IP address for client interface: %q", d.HostIP)
		return nil
	}

	// it's possible we timed out... get updated info having adjusted context to allow it
	// keeping it short
	ctxerr = d.op.Err()

	baseOp := trace.NewOperationWithLoggerFrom(context.Background(), d.op, "ensureApplianceInitializes")
	op, cancel := trace.WithTimeout(&baseOp, 10*time.Second, "ensureApplianceInitializes timeout")
	defer cancel()
	d.op = op
	err := d.applianceConfiguration(conf)
	if err != nil {
		return fmt.Errorf("unable to retrieve updated configuration from appliance for diagnostics: %s", err)
	}

	if ctxerr == context.DeadlineExceeded {
		d.op.Info("")
		d.op.Error("Failed to obtain IP address for client interface")
		d.op.Info("Use vic-machine inspect to see if VCH has received an IP address at a later time")
		d.op.Info("  State of all interfaces:")

		// if we timed out, then report status - if cancelled this doesn't need reporting
		for name, net := range conf.ExecutorConfig.Networks {
			addr := net.Assigned.String()
			if ip.IsUnspecifiedIP(net.Assigned.IP) {
				addr = "waiting for IP"
			}
			d.op.Infof("    %q IP: %q", name, addr)
		}

		// if we timed out, then report status - if cancelled this doesn't need reporting
		d.op.Info("  State of components:")
		for name, session := range conf.ExecutorConfig.Sessions {
			status := "waiting to launch"
			if session.Started == "true" {
				status = "started successfully"
			} else if session.Started != "" {
				status = session.Started
			}
			d.op.Infof("    %q: %q", name, status)
		}

		return errors.New("Failed to obtain IP address for client interface (timed out)")
	}

	return fmt.Errorf("Failed to get IP address information from appliance: %s", updateErr)
}

// CheckServiceReady checks if service is launched correctly, including ip address, service initialization, VC connection and Docker API
// Should expand this method for any more VCH service checking
func (d *Dispatcher) CheckServiceReady(ctx context.Context, conf *config.VirtualContainerHostConfigSpec, clientCert *tls.Certificate) error {
	defer func(oldOp trace.Operation) { d.op = oldOp }(d.op)
	d.op = trace.FromContext(ctx, "CheckServiceReady")

	if err := d.ensureApplianceInitializes(conf); err != nil {
		return err
	}

	// vic-init will try to reach out to the vSphere target.
	d.op.Info("Checking VCH connectivity with vSphere target")
	// Checking access to vSphere API
	if cd, err := d.CheckAccessToVCAPI(d.appliance, conf.Target); err == nil {
		code := int(cd)
		if code > 0 {
			d.op.Warnf("vSphere API Test: %s %s", conf.Target, diag.UserReadableVCAPITestDescription(code))
		} else {
			d.op.Infof("vSphere API Test: %s %s", conf.Target, diag.UserReadableVCAPITestDescription(code))
		}
	} else {
		d.op.Warnf("Could not run VCH vSphere API target check due to %v but the VCH may still function normally", err)
	}

	if err := d.CheckDockerAPI(conf, clientCert); err != nil {
		err = errors.Errorf("Docker API endpoint check failed: %s", err)
		// log with info because this might not be an error
		d.op.Info(err)
		return err
	}
	return nil
}

// deleteFolder deletes the supplied folder if it is empty.  During a VCH Delete there is a slight possibility vic
// could delete a folder it didn't create.  The only time a VCH would be in a folder vic didn't create
// would be outside of normal vic operations.  There is no risk of loss of data as it will this will only
// delete an empty folder.
func (d *Dispatcher) deleteFolder(folder *object.Folder) {
	// only continue if VC and the target Folder is NOT the datacenter wide VM Folder
	if d.isVC && folder != nil && folder.Reference() != d.session.VMFolder.Reference() {
		children, err := folder.Children(d.op)
		if err != nil {
			d.op.Errorf("Unable to retrieve Folder(%s) contents: %s", folder.InventoryPath, err)
			return
		}
		if len(children) > 0 {
			d.op.Warnf("Folder(%s) is not empty and will not be removed", folder.InventoryPath)
			return
		}
		d.op.Debugf("Destroying folder %s", folder.Name())
		_, err = tasks.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
			return folder.Destroy(d.op)
		})
		if err != nil {
			d.op.Errorf("Failed to remove Folder(%s) - manual removal may be needed: %s", folder.InventoryPath, err)
		}
	}
}
