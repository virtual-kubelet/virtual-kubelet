# Communications

This document discusses possible communication channels between components, but not what flows over them. This is essentially Layer2/3 with Layer7 discussed in the documents relating to specific components.


## ContainerVM to Appliance
There are two types of communication that occur between a containerVM and the [VCH appliance](components.md#appliance):

1. application level network traffic
2. VIC control channel, which includes user interaction with the container console

This section addresses the VIC control channel


### Async - guestinfo
The containerVM guestinfo is used as a persistent, asynch publishing mechanism for specific data related to container state, primarily:

1. assigned DHCP address
2. error reporting for [containerVM components](components.md#container)
3. exit status of the container process

Appliance to containerVM communication also uses guestinfo as a channel and is addressed in a little more depth under [configuration](configuration.md#appliance)


### Synchronous
The following are different approaches to handling the interactive communication path between containerVM and appliance, each with pros and cons.

#### Network serial port
The network serial port is a mechanism provided by vSphere that allows serial traffic to be forwarded over a TCP connection; it can operate in either client or server mode.

Pros:
* native to vSphere
* serial support can be assumed in most OSes

Cons:
* requires Enterprise license
* requires route from host to appliance VM
  * implies firewall config
  * generally means the appliance needs to be on the management network
* serial does not appear to be efficient for large volumes of data
* inhibits vMotion if not targeted at a Virtual Serial Port Concentrator (vSPC)
* serial port access and access semantics differ significantly between OSes


#### vSPC relay
A vSPC is explicitly to enable vMotion for VMs that have network serial connections. When connecting to a vSPC the connection is an extended TELNET protocol rather than raw TCP. In the case of a regular vSPC appliance, the vSPC has to bridge between the management network (network serial originates from the ESX host, not the VM) and a network the VIC appliance is attached to.
The vSPC relay is an agent on ESX that does dumb forwarding of the incoming Telnet connection via vSocket into the VCH appliance, which provides a very basic vSPC implementation. In this case the containerVM would be configured with a network serial port with a vSPC target that is the ESX host running the applianceVM.

Pros:
* appliance does not require connectivity with the management network (when combined with [vmomi agent](#components.md#vmomi-authenticating-agent) )
* containerVM vMotion

Cons:
* requires Enterprise license
* require knowledge of applianceVM host and reconfiguration if applianceVM vMotion occurs


### Pipe backed serial port with relay agent
The pipe backed serial port is a mechanism provided by vSphere that maps serial port IO to a named pipe on the ESX host. This allows an agent on the host to act as a relay for that data.

Pros:
* does not require Enterprise license
* serial support can be assumed in most OSes
* does not inhibit vMotion

Cons:
* requires agent on all hosts in the cluster that will run containerVMs
* requires route from host to appliance VM
* serial port access and access semantics differ significantly between OSes


#### vSocket relay
[vSocket relay](components.md#vsocket-relay-agent) makes use of the paravirtual vSocket mechanism for VM to Host communication. This model requires a agent running on all ESX hosts in the cluster that can receive incoming vSocket communications and forward them to the appliance.

Pros:
* does not require Enterprise license
* common access and usage semantics across OSes
* does not inhibit vMotion
* improved efficiency over serial (uses PIO ports)

Cons:
* requires driver in the OS (easily supplied in the bootstrap image if driver exists)
* requires agent on all hosts in the cluster that will run containerVMs
* requires route from host to appliance VM

#### vSocket peer-to-peer relay
An extension of this approach has the vSocket relay agent forwarding to another vSocket relay agent on the host running the applianceVM, with that connection then relayed via vSocket into the appliance. The pros and cons below are in addition to those without the peer-to-peer relay.

Pros:
* appliance does not require connectivity with the management network (when combined with [vmomi agent](#components.md#vmomi-authenticating-agent) )

Cons:
* require knowledge of applianceVM host and reconfiguration if applianceVM vMotion occurs

