# Tether

The tether provides two distinct sets of function:

1. the channel necessary to present the process input/output as if it were bound directly to the users local console.
2. configuration of the operating system - where regular docker can manipulate elements such as network interfaces directly, or use bind mounting of files to inject data and configuration into a container, this approach is not available when the container is running as an isolated VM; the tether has to take on all responsibility for correct configuration of the underlying OS before launching the container process.

## Operating System configuration

### Hostname

### Name resolution

### Filesystem mounts

### Network interfaces


## Management behaviours
This is a somewhat arbitrary divide, but these are essentially ongoing concerns rather than one-offs on initial start of the containerVM

### Hotplug
It's not yet determined whether the containerVM bootstrap images will include udevd, systemd, or if tether will take on that responsibility directly; this could be use of libudev or direct handling of netlink messages given the very limited scope of hotplug devices we need support for (CPU, memory, scsi disks, NICs).

### Signal handling

### Process reaping
Tether runs as pid1 when in a containerVM and has to discharge the associated responsibilities. This primarily means reaping orphaned children so that the process list does not get cluttered with zombies. On a more prosaic note, it's been observed that a connection to a bash container will not exit fully after typing `exit` if reaping isn't performed.

### Secrets

### Forking
This relates specifically to VMfork, aka _Instant Clone_ - the ability to freeze a VM and spin off arbitrary numbers of children that inherit that parent VMs memory state and configuration. This requires cooperation between ESX, GuestOS, and the application processes, to handle changes to configuration such as time, MAC and IP addresses, ARP caches, open network connections, etc.
More directly, VMfork requires that the fork be triggered from within the GuestOS as a means of ensuring it is in a suitable state, meaning the tether has to handle triggering of forking and recovery post-fork


## External communication
The vast bulk of container operations can be performed without a synchronous connection to containerVM, however `attach` is a core element of the docker command set and the one that is probably most heavily used outside of production deployment. This requires that we be able to present the user with a console for the container process.

The initial communication will be via plain network serial port, configured in client mode:

Overall flow:

1. ContainerVM powered on
2. ContainerVM configured so com1 is a client network serial port targeting the appliance VM
2. ESX initiates a serial-over-TCP connection to the applianceVM - this link is treated as a reliable bytestream
3. ApplianceVM accepts the connection
4. ApplianceVM acts as an SSH client over the new socket - authentication is negligible for now
5. ApplianceVM uses a custom global request type to retrieve list of containers running in containerVM (exec presents as a separate container); containerVM replies with list
6. ApplianceVM opens a channel, requesting a specific container; containerVM acknowledges and copies I/O between process and channel


From the Personality to the Portlayer:
* request for container X from Personality to Interaction component - X is a container handle
* Interaction configures containerVM network serial port to point at it's IP and connects the serial port
* Interaction blocks waiting for SSH session with X, record request in connection map
* Interaction starts stream copying from HTTP websocket to SSH channel

Incoming TCP connection to Interaction component:
* request set of IDs on the executor - (5) in overall flow
* create entries for each of the IDs in connection map
* if there is a recorded request for channel to a container X
  * establish attach channel
  * notify waiters for X
