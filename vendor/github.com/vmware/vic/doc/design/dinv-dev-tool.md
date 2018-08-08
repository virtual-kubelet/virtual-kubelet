# Goal

The goal for this investigation, is to find a suitable tool to enable "easy-button" provisioning of Docker hosts on top of VIC, also referred to as DinV, or Docker-in-VIC.

The short term goal is to provide a tool for the user to provision a prebuilt docker image containing a full fledged docker host and make it available to the user, automating the provisioning and security part (deployment, creation of certificates, registry cache persistency) as much as possible, giving the developers a tool to self-provision docker hosts with an easy button.

# Options

We're currently evaluating two options: use Docker Machine, or build our own tool.

# Investigation: Why docker machine isn't a good choice.

Docker machine is an open source tool published by docker, helps users in getting started with creating docker hosts, it works on various platforms, including desktops (Virtualbox, HyperV, Fusion), mega clouds (aws, azure, google cloud) and private clouds (vsphere, openstack).

Docker machine has a modular plugin architecture for the infrastructure part, but lacks a modular provisioning system, which means we cannot provision anything outside the OS/binaries that are built into its core.

VMware has contributed drivers for its platforms (Fusion, vSphere and vCloud Air), the people at Docker started to modularize these drivers with a component called libmachine.

Libmachine, despite the name, is not entirely modular, its external interface only includes the infrastructure portion (which also requires ssh)[1]  and doesn't have an interface for OS provisioning we can plug into, the only way to have a new OS or provisioning system supported, is to have a PR accepted into docker machine core.

There is no other way around this, even crafting an infrastructure plugin to do both infra provisioning and the certificate automation part won't work, docker machine expects to rely on libmachine for all the day2 operations it handles (upgrade, regenerate-certificates and even create), this renders Docker machine unsuitable for our needs.

Same goes for the security aspect, certificate creation is performed on the local machine and copied over SSH and there are no hooks whatsoever in this process.

# Investigation: How we should build our tool.

Developers and Ops people are accustomed to the modus operandi of docker machine, our tool has a very similar purpose, but with a specific target which is the docker api exposed by VIC (or others in the future), modeling our own tool on the same flow that docker machine has will ensure an easy adoption curve with our users.

Building our own tool means also making ourselves future proof with regards to new personalities and use cases with VIC, two examples that come to mind: Kubernetes and Docker Datacenter.

Our tool will initially focus heavily on the DinV use case, meaning we'll be a consumer of the docker API that the VCH provides, plus helpers that will automate the provisioning part of the process (some of these parts can be borrowed from docker/machine packages, like for certificate creation).

The tool will be dependent on the VCH implementing the `docker cp` operation (https://github.com/vmware/vic/issues/769).

We will provide an official "DinV" image, based on photon, which we will support, as well as the source for the container that customers can use to "roll their own" DinV image, with their own customization (including things like running `sshd` inside the container). For customization purposes, it would be ideal to support LinuxKit, but that will require VIC to support running container processes as PID 1.

# Conclusions

Building our own tool is the most sensible option right now, modeling it after docker machine will ensure an easy migration path for people accustomed to the tool, and will make it future proof for new type of deployments to come.

[1]
```go
// Driver defines how a host is created and controlled. Different types of
// driver represent different ways hosts can be created (e.g. different
// hypervisors, different cloud providers)
type Driver interface {
	// Create a host using the driver's config
	Create() error

	// DriverName returns the name of the driver
	DriverName() string

	// GetCreateFlags returns the mcnflag.Flag slice representing the flags
	// that can be set, their descriptions and defaults.
	GetCreateFlags() []mcnflag.Flag

	// GetIP returns an IP or hostname that this host is available at
	// e.g. 1.2.3.4 or docker-host-d60b70a14d3a.cloudapp.net
	GetIP() (string, error)

	// GetMachineName returns the name of the machine
	GetMachineName() string

	// GetSSHHostname returns hostname for use with ssh
	GetSSHHostname() (string, error)

	// GetSSHKeyPath returns key path for use with ssh
	GetSSHKeyPath() string

	// GetSSHPort returns port for use with ssh
	GetSSHPort() (int, error)

	// GetSSHUsername returns username for use with ssh
	GetSSHUsername() string

	// GetURL returns a Docker compatible host URL for connecting to this host
	// e.g. tcp://1.2.3.4:2376
	GetURL() (string, error)

	// GetState returns the state that the host is in (running, stopped, etc)
	GetState() (state.State, error)

	// Kill stops a host forcefully
	Kill() error

	// PreCreateCheck allows for pre-create operations to make sure a driver is ready for creation
	PreCreateCheck() error

	// Remove a host
	Remove() error

	// Restart a host. This may just call Stop(); Start() if the provider does not
	// have any special restart behaviour.
	Restart() error

	// SetConfigFromFlags configures the driver with the object that was returned
	// by RegisterCreateFlags
	SetConfigFromFlags(opts DriverOptions) error

	// Start a host
	Start() error

	// Stop a host gracefully
	Stop() error
}
```
