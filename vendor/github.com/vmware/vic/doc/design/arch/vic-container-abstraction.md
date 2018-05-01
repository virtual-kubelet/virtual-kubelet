### The VIC Container Abstraction

VIC provisions containers _as_ VMs, rather than _in_ VMs. In understanding the VIC container abstraction, it is helpful to compare and contrast against the virtualization of a traditional container host.

#### Traditional Container Host

Let's take a Linux VM running Docker as an example. The container host is a VM running a Linux OS with the necessary libraries, kernel version and daemon installed. The container host will have a fixed amount of memory and vCPU resource that can be used by the containers provisioned into it.

The container host operating system along with the Docker daemon have to provide the following:
* **The control plane** - an endpoint through which control operations are performed, executing in the same OS as its controlling
* **A container abstraction** - library extensions to the guest OS need to provide a private namespace and resource constraints
* **Network virtualization** - simple bridge networking or overlay networking
* **Layered filesystem** - not an absolute requirement for a container, but typically conflated in most implementations
* **OS Kernel** - a dependency for the container executable to execute on, typically shared between containers

The hypervisor in this mode provides hardware virtualization of the entire container host VM, one or more VMDKs providing local disk for the OS, one or more vNICs to provide network connectivity for the OS and possibly paravirtualization capabilities allowing the containers to directly access hypervisor infrastructure.

#### The VIC model

VIC containers operate quite differently. In the above model, it would be reasonable to describe a container as being run _in_ a VM. In the VIC model, a container is run _as_ a VM. For the purposes of this project, we will refer to this as a _containerVM_.

So what does this mean in practice? Well, firstly a container host isn't a VM, it's a resource pool - this is why we call it a _Virtual_ Container Host. It's an abstract dynamically-configurable resource boundary into which containers can be provisioned. As for the other functions highlighted above:
* **The control plane** - functionally the same endpoint as above, but controlling vSphere and running in its own OS
* **A container abstraction** - is a VM. A VM provides resource constraints and a private namespace, like a container
* **Network virtualization** - provided entirely by vSphere. NSX, distributed port-groups. Each container gets a vNIC
* **Layered filesystem** - provided entirely by vSphere. VMDK snapshots in the initial release
* **OS Kernel** - provided as a minimal ISO from which the containerVM is either booted or forked

In this mode, there is necessarily a 1:1 coupling between a container and a VM. A container image is attached to the VM as a disk, the VM is either booted or forked from the kernel ISO, then the containerVM chroots into the container filesystem effectively becoming the container.

#### Differences

This model leads to some very distinct differences between a VIC container and a traditional container, none of which impact the portability of the container abstraction between these systems, but which are important to understand.

##### Container

1. There is no default shared filesystem between the container and its host
  * Volumes are attached to the container as disks and are completely isolated from each other
  * A shared filesystem could be provided by something like an NFS volume driver
2. The way that you do low-level management and monitoring of a container is different. There is no VCH shell.
  * Any API-level control plane query, such as `docker ps`, works as expected
  * Low-level management and monitoring uses exactly the same tools and processes as for a VM
3. The kernel running in the container is not shared with any other container
  * This means that there is no such thing as an optional _privileged_ mode. Every container is privileged and fully isolated.
  * When a containerVM kernel is forked rather than booted, much of its immutable memory is shared with a parent _template_
4. There is no such thing as unspecified memory or CPU limits
  * A Linux container will have access to all of the CPU and memory resource available in its host if not specified
  * A containerVM must have memory and CPU limits defined, either derived from a default or specified explicitly

##### Virtual Container Host

A container host in VIC is a _Virtual_ Container Host (VCH). A VCH is not in itself a VM - it is an abstract dynamic resource boundary that is defined and controlled by vSphere into which containerVMs can be provisioned. As such, a VCH can be a subset of a physical host or a subset of a cluster of hosts.

However a container host also represents an API endpoint with an isolated namespace for accessing the control plane, so a functionally equivalent service must be provisioned to the vSphere infrastructure that provides the same endpoint for each VCH. There are various ways in which such an service could be deployed, but the simplest representation is to run it in a VM.

Given that a VCH in many cases will represent a subset of resource from a cluster of physical hosts, it is actually closer in concept to something like Docker Swarm than a traditional container host.

There are also necessarily implementation differences, transparent to the user, which are required to support this abstraction. For example, given that a container is entirely isolated from other containers and its host is just an esoteric resource boundary, any control operations performed within the container - launching processes, streaming stout/stderr, setting environment variables, network specialization - must be done either by modifying the container image disk before it is attached; or through a special control channel embedded in the container (see [Tether](vic-port-layer-overview.md#the-tether-process)).
