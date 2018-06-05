# Configuration

Three primary elements make up a functioning VCH - these are referred to as components in the rest of this document, along with their subcomponents:

1. [vic-machine](vic-machine.md) - the mechanism by which the VCH is deployed, configured, and inspected
2. appliance - a VM that runs the VCH logic ([docker personality](components.md#docker-api-server), [log server](components.md#vicadmin), [port-layer](arch/vic-port-layer-overview.md), et al)
3. containerVMs - VMs that _are_ containers ([tether](tether.md), and container process)

This document discusses how configuration between components is done, and why it's done in that manner.

## Component interactions

For both the VCH and containerVMs the configuration is used for two way communication, with a component updating its own configuration as an asynch publishing mechanism for low-frequency, persistent information. The fields listed below are examples rather than comprehensive lists.

### vic-machine configuration of appliance


Configures VCH on initial deploy:
* _vSphere target and credentials_ (hidden)
* networks (external, client, management, bridge, mapped vsphere networks)
* vsphere compute path (e.g. resource pool)
* image store URI
* container store URI
* permissible URIs for volumes
* white/black list of registries
* _etc_

vic-machine also:
* updates VCH config as part of management operations
* inspects (read-only) containerVM configuration for diagnostics

### appliance

Configuration of containers and environment:
* Configures containerVMs with (example config elements below)
    * _appliance_ (hidden)
    * ID
    * command (executable, args, environment, working directory)
    * networks (MAC, CIDR, gateway, name, nameservers)
* configures logical networks (whether IPAM segregation or SDN logical networks)
* configures VMDKs (volumes, images, container read/write layers)

Publishes the following:
* DHCP supplied network configuration
* system & subcomponent status


### ContainerVM

Publishes the following:
* DHCP supplied network configuration
* container start status
* container process exit code


## Configuration persistence mechanism

VIC uses the vSphere `extraConfig` and `guestinfo` mechanisms for storing configuration, the former used for config associated with either appliance or contianerVM that should **not** be visible to the GuestOS, and the latter for config that should. In reality the mechanisms are the same, with guestinfo being a subset of extraConfig.

Access to guestinfo data within the GuestOS is done via the vmware-rpctool, or library providing the same capability e.g. [vmware/vmw-guestinfo](https://github.com/vmware/vmw-guestinfo/). The various subcomponents in the appliance and the tether in containerVMs will access this configuration directly via library calls - there is no intermediate step that presents this data via arguments or environment variables.

The reasons for taking this approach:

1. it keeps the appliance VM completely stateless - a reboot will return it to a known good state
2. no cross component dependency - containerVMs do not require an VCH for ongoing operations, including vSphere High Availability restart
3. the configuration is protected from tampering, even if superuser priviledges are obtained
4. configuration is visible both from within the VM and remotely via vSphere APIs

ExtraConfig data is used for data that should not be visible to the GuestOS - the best example of this are the vSphere target and credentials (ideally an SSO token), which are hidden from the appliance and only visible to the [vmomi agent](components.md#vmomi-authenticating-agent) that supplies the authenticated vSphere API connection. This means that even if superuser priviledges are obtained on the appliance, the risk is limited to a controlled connection rather than leaking any form of access token or even the URI of the vSphere endpoint.


## Accessing configuration - implementation

The defacto model for holding configuration in Go is a struct with Fields that contain the specifics. The idiomatic way of populating a struct from a serialized form is with a Decoder from the [Go encoding](https://golang.org/pkg/encoding/) package.

It is expected that use of the configuration be performed via two endcoder/decoder pairs - one that operates directly on guestinfo via the vmx-guestinfo library, the other that takes a [VirtualMachineConfigSpec](pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.vm.ConfigSpec.html) and operates on the ExtraConfig field of the spec.

The implementation will understand the following set of annotations and map the fields to appropriate extraConfig keys - in the case where the key describes a boolean state, omitting the annotation implies the opposite:
* `hidden` - hidden from GuestOS
* `read-only` - value can only be modified via vSphere APIs
* `non-persistent` - value will be lost on VM reboot
* `volatile` - field is not exported directly, but via a function that freshens the value each time)
* `atomic:<group>` - speculative to allow atomic updates to multiple fields
