## Installing Virtual Integrated Containers Engine

The intent is that vSphere Integrated Containers Engine (VIC Engine) should not _require_ an installation step - deploying a [Virtual Container Host](../design/arch/vic-container-abstraction.md#virtual-container-host) (VCH) directly without any prior steps should always be possible. At the current time this is the only approach available.

## Deploying a Virtual Container Host

### Requirements

- ESXi/vCenter - the target virtualization environment.
   - ESXi - Enterprise license
   - vCenter - Enterprise plus license, only very simple configurations have been tested.
- Bridge network - when installed in a vCenter environment vic-machine does not automatically create a bridge network. An existing vSwitch or Distributed Portgroup should be specified via the -bridge-network flag, should not be the same as the external network, and should not have DHCP.

### Privileges and credentials

There is an operations user mechanism provided to allow a VCH to operate with less privileged credentials than are required for deploying a new VCH. These options are:
* `--ops-user`
* `--ops-password`
If neither option is specified then the user supplied via `--target` or `--user` will be used and a warning will be output. If only the `--ops-user` option is provided there will be an interactive prompt for the password.
At this time vic-machine _does not create_ the operations user, nor does it configure it with minimum set of RBAC permissions necessary for operation - this is actively being worked on (#1689)

Deploying a VCH requires credentials able to:
* create and configure a resource pool (vApp on vCenter)
* create a vSwitch (ESX only)
* create and configure the endpointVM within that pool
* upload files to datastores
* inspect much of the vSphere inventory

However operation of a VCH requires only a subset of those privileges:
* create and configure containerVMs within the VCH resource pool/vApp
* create/delete/attach/detach disks on the endpointVM
* attach containerVMs to supplied networks
* upload to/download from datastore

### Example
Replace the `<fields>` in the following example with values specific to your environment - this will install VCH to the specified resource pool of ESXi or vCenter, and the container VMs will be created under that resource pool. This example will use DHCP for the API endpoint and will not configure client authentication.

- --target is the URL of the destination vSphere environment in the form `https://user:password@ip-or-fqdn/datacenter-name`. Protocol, user, and password are OPTIONAL. Datacenter is OPTIONAL if targeting ESXi or only one datacenter is configured.
- --compute-resource is the compute resource where VCH will be deployed to. This should be the name of a cluster or resource pool, e.g. `myCluster` (vCenter), or `myPool` .
- --thumbprint is the thumbprint of the server's certificate, required if the certificate cannot be validated with a trusted certificate authority (`--force` will accept whatever certificate is presented)
- Note: environment variables can be set instead of these options:
    - --target => VIC\_MACHINE\_TARGET
    - --user => VIC\_MACHINE\_USER
    - --password => VIC\_MACHINE\_PASSWORD
    - --thumbprint => VIC\_MACHINE_THUMBPRINT

```
vic-machine-linux create --target <target-host>[/datacenter] --user <root> --password <password> --thumbprint <certificate thumbprint> --compute-resource <cluster or resource pool name> --image-store <datastore name> --name <vch-name> --no-tlsverify
```

This will, if successful, produce output similar to the following when deploying VIC Engine onto an ESXi (output was generated with --client-network-ip specified instead of `--no-tlsverify` denoted above):
```
INFO[2016-11-07T22:01:22Z] Using client-network-ip as cname for server certificates - use --tls-cname to override: x.x.x.x
WARN[2016-11-07T22:01:22Z] Using administrative user for VCH operation - use --ops-user to improve security (see -x for advanced help)
INFO[2016-11-07T22:01:22Z] Generating CA certificate/key pair - private key in ./XXX/ca-key.pem
INFO[2016-11-07T22:01:22Z] Generating server certificate/key pair - private key in ./XXX/server-key.pem
INFO[2016-11-07T22:01:22Z] Generating client certificate/key pair - private key in ./XXX/key.pem
INFO[2016-11-07T22:01:22Z] Generated browser friendly PFX client certificate - certificate in ./XXX/cert.pfx
INFO[2016-11-07T22:01:22Z] ### Installing VCH ####
INFO[2016-11-07T22:01:22Z] Validating supplied configuration
INFO[2016-11-07T22:01:22Z] Configuring static IP for additional networks using port group "VM Network"
INFO[2016-11-07T22:01:23Z] Firewall status: DISABLED on "/ha-datacenter/host/esx.localdomain/esx.localdomain"
WARN[2016-11-07T22:01:23Z] Firewall configuration will be incorrect if firewall is reenabled on hosts:
WARN[2016-11-07T22:01:23Z]   "/ha-datacenter/host/esx.localdomain/esx.localdomain"
WARN[2016-11-07T22:01:23Z] Firewall must permit 2377/tcp outbound if firewall is reenabled
INFO[2016-11-07T22:01:23Z] License check OK
INFO[2016-11-07T22:01:23Z] DRS check SKIPPED - target is standalone host
INFO[2016-11-07T22:01:23Z]
INFO[2016-11-07T22:01:23Z] Creating Resource Pool "XXX"
INFO[2016-11-07T22:01:23Z] Creating directory [datastore1] volumes
INFO[2016-11-07T22:01:23Z] Datastore path is [datastore1] volumes
INFO[2016-11-07T22:01:23Z] Creating appliance on target
INFO[2016-11-07T22:01:23Z] Network role "management" is sharing NIC with "client"
INFO[2016-11-07T22:01:23Z] Network role "external" is sharing NIC with "client"
INFO[2016-11-07T22:01:23Z] Uploading images for container
INFO[2016-11-07T22:01:23Z]      "bootstrap.iso"
INFO[2016-11-07T22:01:23Z]      "appliance.iso"
INFO[2016-11-07T22:01:30Z] Waiting for IP information
INFO[2016-11-07T22:01:44Z] Waiting for major appliance components to launch
INFO[2016-11-07T22:01:54Z] Initialization of appliance successful
INFO[2016-11-07T22:01:54Z]
INFO[2016-11-07T22:01:54Z] vic-admin portal:
INFO[2016-11-07T22:01:54Z] https://x.x.x.x:2378
INFO[2016-11-07T22:01:54Z]
INFO[2016-11-07T22:01:54Z] Published ports can be reached at:
INFO[2016-11-07T22:01:54Z] x.x.x.x
INFO[2016-11-07T22:01:54Z]
INFO[2016-11-07T22:01:54Z] Docker environment variables:
INFO[2016-11-07T22:01:54Z] DOCKER_TLS_VERIFY=1 DOCKER_CERT_PATH=/home/vagrant/vicsmb/src/github.com/vmware/vic/bin/XXX DOCKER_HOST=x.x.x.x:2376
INFO[2016-11-07T22:01:54Z]
INFO[2016-11-07T22:01:54Z] Environment saved in XXX/XXX.env
INFO[2016-11-07T22:01:54Z]
INFO[2016-11-07T22:01:54Z] Connect to docker:
INFO[2016-11-07T22:01:54Z] docker -H x.x.x.x:2376 --tlsverify --tlscacert="./XXX/ca.pem" --tlscert="./XXX/cert.pem" --tlskey="./XXX/key.pem" info
INFO[2016-11-07T22:01:54Z] Installer completed successfully
```

## Deleting a Virtual Container Host

Specify the same resource pool and VCH name used to create a VCH, then the VCH will removed, together with the created containers, images, and volumes, if --force is provided. Here is an example command and output - replace the `<fields>` in the example with values specific to your environment.

```
vic-machine-linux delete --target <target-host>[/datacenter] --user <root> --password <password> --compute-resource <cluster or resource pool name> --name <vch-name>
INFO[2016-06-27T00:09:25Z] ### Removing VCH ####
INFO[2016-06-27T00:09:26Z] Removing VMs
INFO[2016-06-27T00:09:26Z] Removing images
INFO[2016-06-27T00:09:26Z] Removing volumes
INFO[2016-06-27T00:09:26Z] Removing appliance VM network devices
INFO[2016-06-27T00:09:27Z] Removing Portgroup XXX
INFO[2016-06-27T00:09:27Z] Removing VirtualSwitch XXX
INFO[2016-06-27T00:09:27Z] Removing Resource Pool XXX
INFO[2016-06-27T00:09:27Z] Completed successfully
```


## Inspecting a Virtual Container Host

Specify the same resource pool and VCH name used to create a VCH, vic-machine inspect can show the VCH information.

```
vic-machine-linux inspect --target <target-host>[/datacenter] --user <root> --password <password> --compute-resource <cluster or resource pool name> --name <vch-name>
INFO[2016-10-08T23:40:28Z] ### Inspecting VCH ####
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] VCH ID: VirtualMachine:286
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] Installer version: v0.6.0-0-0fac2c0
INFO[2016-10-08T23:40:29Z] VCH version: v0.6.0-0-0fac2c0
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] VCH upgrade status:
INFO[2016-10-08T23:40:29Z] Installer has same version as VCH
INFO[2016-10-08T23:40:29Z] No upgrade available with this installer version
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] vic-admin portal:
INFO[2016-10-08T23:40:29Z] https://x.x.x.x:2378
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] Docker environment variables:
INFO[2016-10-08T23:40:29Z]   DOCKER_HOST=x.x.x.x:2376
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z]
INFO[2016-10-08T23:40:29Z] Connect to docker:
INFO[2016-10-08T23:40:29Z] docker -H x.x.x.x:2376 --tls info
INFO[2016-10-08T23:40:29Z] Completed successfully
```


## Enabling SSH to Virtual Container Host appliance
Specify the same resource pool and VCH name used to create a VCH, vic-machine debug will enable SSH on the appliance VM and then display the VCH information, now with SSH entry.

```
vic-machine-linux debug --target <target-host>[/datacenter] --user <root> --password <password> --compute-resource <cluster or resource pool name> --name <vch-name> --enable-ssh --rootpw <other password> --authorized-key <keyfile>
INFO[2016-10-08T23:41:16Z] ### Configuring VCH for debug ####
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] VCH ID: VirtualMachine:286
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] Installer version: v0.6.0-0-0fac2c0
INFO[2016-10-08T23:41:16Z] VCH version: v0.6.0-0-0fac2c0
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] SSH to appliance
INFO[2016-10-08T23:41:16Z] ssh root@x.x.x.x
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] vic-admin portal:
INFO[2016-10-08T23:41:16Z] https://x.x.x.x:2378
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] Docker environment variables:
INFO[2016-10-08T23:41:16Z]   DOCKER_HOST=x.x.x.x:2376
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z]
INFO[2016-10-08T23:41:16Z] Connect to docker:
INFO[2016-10-08T23:41:16Z] docker -H x.x.x.x:2376 --tls info
INFO[2016-10-08T23:41:16Z] Completed successfully
```

## List Virtual Container Hosts

vic-machine ls can list all VCHs in your VC/ESXi, or list all VCHs under the provided resource pool by compute-resource parameter.
```
vic-machine-linux ls --target <target-host> --user <root> --password <password>
INFO[2016-08-08T16:21:57-05:00] ### Listing VCHs ####

ID                           PATH                                                   NAME
VirtualMachine:vm-189        /dc1/host/cluster1/Resources/test1/test1-2        test1-2-1
VirtualMachine:vm-189        /dc2/host/cluster2/Resources/test2/test2-2        test2-2-1
```

```
vic-machine-linux ls --target <target-host>/dc1 --user <root> --password <password> --compute-resource test1
INFO[2016-08-08T16:25:50-02:00] ### Listing VCHs ####

ID                           PATH                                                   NAME
VirtualMachine:vm-189        /dc1/host/cluster1/Resources/test1/test1-2        test1-2-1
```


## Configuring Volumes in a Virtual Container Host

Volumes are implemented as VMDKs and mounted as block devices on a containerVM. This means that they cannot be used concurrently by multiple running, containers. Attempting to start a container that has a volume attached that is in use by a running container will result in an error.

The location in which volumes are created can be specified at creation time via the `--volume-store` argument. This can be supplied multiple times to configure multiple datastores or paths:
```
vic-machine-linux create --volume-store=datastore1/some/path:default --volume-store=ssdDatastore/other/path:fast ...
```

The volume store to use is specified via driver optons when creating volumes (capacity is in MB):
```
docker volume create --name=reports --opts VolumeStore=fast --opt Capacity=1024
```

Providing a volume store named `default` allows the driver options to be omitted in the example above and enables anoymous volumes including those defined in Dockerfiles, e.g.:
```
docker run -v /var/lib/data -it busybox
```


## Exposing vSphere networks within a Virtual Container Host

vSphere networks can be directly mapped into the VCH for use by containers. This allows a container to expose services to the wider world without using port-forwarding:

```
vic-machine-linux create --container-network=vpshere-network:descriptive-name
```

A container can then be attached directly to this network and a bridge network via:
```
docker create --net=descriptive-name haproxy
docker network connect bridge <container-id>
```

Currently the container does **not** have a firewall configured in this circumstance ([#692](https://github.com/vmware/vic/issues/692)).


## TLS configuration

There are three TLS configurations available for the API endpoint - the default configuration is _mutual authentication_.
If there is insufficient information via the create options to use that configuration you will see the following help output:
```
ERRO[2016-11-07T19:53:44Z] Common Name must be provided when generating certificates for client authentication:
INFO[2016-11-07T19:53:44Z]   --tls-cname=<FQDN or static IP> # for the appliance VM
INFO[2016-11-07T19:53:44Z]   --tls-cname=<*.yourdomain.com>  # if DNS has entries in that form for DHCP addresses (less secure)
INFO[2016-11-07T19:53:44Z]   --no-tlsverify                  # disables client authentication (anyone can connect to the VCH)
INFO[2016-11-07T19:53:44Z]   --no-tls                        # disables TLS entirely
INFO[2016-11-07T19:53:44Z]
ERRO[2016-11-07T19:53:44Z] Create cannot continue: unable to generate certificates
ERRO[2016-11-07T19:53:44Z] --------------------
ERRO[2016-11-07T19:53:44Z] vic-machine-linux failed: provide Common Name for server certificate
```

The [`--tls-cert-path`](#certificate-names-and---tls-cert-path) option applies to all of the TLS configurations other than --no-tls.


#### Disabled, `--no-tls`
Disabling TLS completely is strongly discouraged as it allows trivial snooping of API traffic by entities on the same network. When using this option the API will be served over HTTP, not HTTPS.


#### Server authentication, `--no-tlsverify`
In this configuration the API endpoint has a certificate that clients will use to validate the identity of the server. This allows the client to trust the server, but the server does not require
authentication or authorization of the client. If no certificate is provided then a self-signed certificate will be generated.

If using a pre-created certificate, the following options are used:
- `--tls-server-key` - path to key file in PEM format
- `--tls-server-cert` - path to certificate file in PEM format


#### Mutual authentication (tlsverify)

Mutual authentication, also referred to as _tlsverify_, means that the client must authenticate by presenting a certificate to the server in addition to the server authentication with the client.
In this configuration the vicadmin server also requires authentication, which can be via client certificate.

If using pre-created certificates the following option must be provided in addition to the server authentication options above.
- `--tls-ca` - path to certificate authority to vet client certificates against in PEM format. May be specified multiple times.

As a convenience, vic-machine will generate authority, server, and client certificates if a Common Name is provided.
- `--tls-cname` - FQDN or static IP of the API endpoint. This can be an FQDN wildcard to allow use with DHCP if DNS has entries for the DHCP allocated addresses.

These are self-signed certificates and therefore clients will need to be explicit about the certificate authority in order to perform validation.
See [the docker documentation](https://docs.docker.com/engine/security/https/) about TLSVERIFY for details of docker client
configuration (note that the _DOCKER_TLS_VERIFY_ environment variable must be removed from the environment completely to prevent it taking effect)

The following can be used for minimal customization of the generated certificates:
- `--organisation` - shown to clients to help distinguish certificates
- `--certificate-key-size`

If using a static IP for the API endpoint via the following option, a server certificate will be generated using that IP address as the Common Name unless certificates are provided
or `--no-tlsverify` is specified.
- `--client-network-ip` - IP or FQDN to use for the API endpoint


#### Certificate names and `--tls-cert-path`

If using the `--tls-server-key` and `--tls-server-cert` options, any filename and path can be provided for the server certificate and key. However when generating certificates the following standard names are used:
* server-cert.pem
* server-key.pem
* cert.pem
* key.pem
* ca.pem

The default value of `--tls-cert-path` is that of the `--name` parameter, in the current working directory, and is used as:
1. the location to check for existing certificates, by the default names detailed above.
2. the location to save generated certificates, which will occur only if existing certificates are not found

The certificate authority (CA) in the certificate path will only be loaded if no CA is specified via the `--tls-ca` option.

If a warning in the form below is received during creation it means that client authentication was enabled (a certificate authority was provided), but neither that authority nor the ones configured
on the system were able to verify the provided server certificate. This can be a valid configuration, but should be checked:
```
Unable to verify server certificate with configured CAs: <additional detail>
```

_NOTE: while it is possible to mix generated and pre-created client and server certificates additional care must be taken to ensure a working setup_

Sample `vic-machine` output when loading existing certificates for a tlsverify configuration:
```
INFO[2016-11-11T23:58:02Z] Using client-network-ip as cname where needed - use --tls-cname to override: 192.168.78.127
INFO[2016-11-11T23:58:02Z] Loaded server certificate ./xxx/server-cert.pem
INFO[2016-11-11T23:58:02Z] Loaded CA with default name from certificate path xxx
INFO[2016-11-11T23:58:02Z] Loaded client certificate with default name from certificate path xxx
```

#### Using client certificates with wget or curl

To use client certificates with wget and curl requires adding the following options:
```
wget --certificate=/path/to/cert --private-key=/path/to/key
curl --cert=/path/to/cert --key=/path/to/key
```
[Issues relating to Virtual Container Host deployment](https://github.com/vmware/vic/labels/component%2Fvic-machine)
