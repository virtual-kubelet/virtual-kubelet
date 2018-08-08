# Whitelist Registries

vSphere Integrated Containers 1.2 (VIC) added the ability to whitelist registry access in an installed VCH.  When one or more registries are whitelisted for the VCH at install time the VCH goes into 'whitelist mode'.  From this point on, the VCH will only allow access to registries in its list of whitelisted registries.  In this mode, users will not be able to access any non-whitelisted registries, public or private.

## Specifying Whitelist Registries at Installation

Whitelisted registries can be declared during a VCH installation with *vic-machine* parameter, `--whitelist-registry`.  Two other vic-machine parameters affect whitelist registries, `--registry-ca` and `--insecure-registry`.  When whitelisted registries are declared during installation, the latter two parameters acts as modifiers to the whitelisted registries.

Registry-ca declares additional certificates to verify access to registry servers secured with TLS.  If a registry is declared as a whitelist registry and not an insecure registry (discussed below), the VCH must have access to the server certificate to verify access.  The Photon-based VM that VIC uses has a base set of well-known certificates from public CAs.  If a whitelist registry uses a certificate that is not in that set of well-known certificates, the certificate must be uploaded to the VCH via vic-machine's `--registry-ca` parameter.

Insecure-registry declares a registry server that can be used without requiring TLS certificate verification.  This modifies the whitelist label and takes precedence.  For instance, if a registry is declared with `--whitelist-registry` and with `--insecure-registry`, the VCH will assume the registry is an insecure whitelisted registry.  If the registry is listed with only `--whitelist-registry`, then the VCH will attempt to verify access using certificates.

If a registry is declared with `--insecure-registry` but not with `--whitelist-registry`, vic-machine will add the insecure registries to the list of whitelist registries *IF* at least one whitelist registry was declared.

A note about certificates.  During installation, vic-machine will attempt to verify the registry server is actually a valid registry server.  It will also attempt to validate that the certificates declared in `--registry-ca` are valid for the secure whitelisted registries.  Vic-machine only performs best effort validation of registry servers.  It will not remove the server's access from the VCH if it cannot validate the server.

Acceptable values for whitelist registry values are numbered IP, FQDN, CIDR formatted range, and wildcard domains.  If a CIDR format is used, e.g. 192.168.1.1/24, then the VCH will whitelist any IP address within that subnet.  Vic-machine will not try to validate CIDR defined ranges.  If a wildcard domain is provided, e.g. *.company.com, the VCH will whitelist any IP address or FQDN address that it can validate against the domain provided during installation.  A numeric IP address will cause the VCH to perform a reverse DNS lookup to validate against that wild card domain.  As with CIDR values, vic-machine will not attempt to validate wildcard domains during installation.  Examples are provided below.

The parameter `--whitelist-registry` creates a list of registries.  If multiple whitelist registries need to be declared, repeat `--whitelist-registry` multiple times during installation for each registry.

### Example: vch installation with vic-machine

This example installs 2 whitelist registries and 1 insecure registry.

```
./vic-machine-linux create --target=10.2.2.5 --image-store=datastore1 --name=vic-docker --user=root --password=xxxxx --compute-resource="/ha-datacenter/host/office2-sfo2-dhcp121.mycompany.com/Resources" --bridge-network=vic-network --debug=0 --volume-store=datastore1/test:default --tls-cname=*.mycompany.com --whitelist-registry="10.2.40.40:443" --whitelist-registry=10.2.2.1/24 --whitelist-registry=*.mycompany.com --insecure-registry=192.168.100.207  --registry-ca=/home/admin/mycerts/ca.crt
```

### Example: vic-machine's output during installation

Below is a snippet from the vic-machine output for the above command.

```
May 15 2017 16:36:12.453-07:00 WARN  Unable to confirm insecure registry 192.168.100.207 is a valid registry at this time.
May 15 2017 16:36:12.505-07:00 INFO  Insecure registries = 192.168.100.207
May 15 2017 16:36:12.505-07:00 INFO  Whitelist registries = 10.2.40.40:443, 10.2.2.1/24, *.mycompany.com, 192.168.100.207
```

Had the above command also included --debug=1 (or higher), the following would be the output

```
May 15 2017 16:36:12.453-07:00 WARN  Unable to confirm insecure registry 192.168.100.207 is a valid registry at this time.
May 15 2017 16:36:12.505-07:00 DEBUG  Secure registry 10.2.40.40:443 confirmed.
May 15 2017 16:36:12.505-07:00 DEBUG  Skipping registry validation for 10.2.2.1/24
May 15 2017 16:36:12.505-07:00 DEBUG Skipping registry validation for *.eng.vmware.com
May 15 2017 16:36:12.505-07:00 INFO  Insecure registries = 192.168.100.207
May 15 2017 16:36:12.505-07:00 INFO  Whitelist registries = 10.2.40.40:443, 10.2.2.1/24, *.mycompany.com, 192.168.100.207
```

There are a few things to note from this snippet.

1. The confirmation of the insecure registry was not attempted.
2. The whitelist registry that is secured was confirmed.
3. Both CIDR and wildcard domain declared as whitelist were skipped during validation.
4. The final whitelist registry list contains all registries declared with both --whitelist-registry and --insecure-registry.
5. While not stated yet, the IP address above that contained :443 will not prevent users to use just the server address during docker commands.  This will be discussed below.

## Using Docker Commands Against a VCH in 'Whitelist mode'

VIC currently supports Docker commands that are most applicable for production deployment of containers.  The commands that are affected by whitelist mode are docker info, docker login, and docker pull.  Below are examples of docker commands issued for the VCH installed with the command above.  Let's assume vic-machine properly installs a VCH with the above command and it reports the VCH has an FQDN of myvch.mycompany.com.

### Example: docker -H myvch.mycompany.com info

```
devbox:~/$ docker -H myvch.mycompany.com:2376 --tlsverify --tlscacert="vic-docker/ca.pem" --tlscert="vic-docker/cert.pem" --tlskey="vic-docker/key.pem" info

Containers: 0
 Running: 0
 Paused: 0
 Stopped: 0
Images: 0
Server Version: v1.1.0-rc3-0-c913391
Storage Driver: vSphere Integrated Containers v1.1.0-rc3-0-c913391 Backend Engine
VolumeStores: default
vSphere Integrated Containers v1.1.0-rc3-0-c913391 Backend Engine: RUNNING
 VCH CPU limit: 10414 MHz
 VCH memory limit: 58.61 GiB
 VCH CPU usage: 3103 MHz
 VCH memory usage: 56.03 GiB
 VMware Product: VMware ESXi
 VMware OS: vmnix-x86
 VMware OS version: 6.0.0
 Insecure Registries: 192.168.100.207
 Registry Whitelist Mode: enabled
 Whitelisted Registries: 10.2.40.40:443, 10.2.2.1/24, *.mycompany.com, 192.168.100.207
Plugins: 
 Volume: vsphere
 Network: bridge
Swarm: inactive
Operating System: vmnix-x86
OSType: vmnix-x86
Architecture: x86_64
CPUs: 10414
Total Memory: 58.61 GiB
ID: vSphere Integrated Containers
Docker Root Dir: 
Debug Mode (client): false
Debug Mode (server): false
Registry: registry-1.docker.io
Experimental: false
Live Restore Enabled: false
```

There are a few things to note in the output of this docker info call.

1. Insecure Registry and whitelist registry lists are shown.
2. There is a message, 'Registry Whitelist Mode: enabled'.  If no whitelist registries are declared during installation, this message will not be shown.
3. 'Registry: registry-1.docker.io' is displayed even though that address was not whitelisted.  This is the address for docker hub.  It does not mean docker hub is accessible (shown in example below).  It is simply the default registry that is attempted when attempting to login or pull without a registry address.

### Example: docker -H myvch.mycompany.com login 10.2.40.40

```
devbox:~/$ docker -H myvch.mycompany.com:2376 --tlsverify --tlscacert="vic-docker/ca.pem" --tlscert="vic-docker/cert.pem" --tlskey="vic-docker/key.pem" login 10.2.40.40

Username: 
Password: 
Login Succeeded
```

In this example, a command was issued to log onto a registry that was declared during installation.  Note, :443 was included during installation but left off during docker login.  The VCH will accept either form of the address.

### Example: docker -H myvch.mycompany.com login

```
devbox:~/$ docker -H myvch.mycompany.com:2376 --tlsverify --tlscacert="vic-docker/ca.pem" --tlscert="vic-docker/cert.pem" --tlskey="vic-docker/key.pem" login

Login with your Docker ID to push and pull images from Docker Hub. If you don't have a Docker ID, head over to https://hub.docker.com to create one.
Username: user
Password: 
Error response from daemon: Access denied to unauthorized registry (registry-1.docker.io) while VCH is in whitelist mode
```

Notice when the registry address is left off, it attempts to access docker hub (which was indicated in the docker info output above), but the VCH denies access.

### Example: docker -H myvch.mycompany.com pull 10.2.40.40/test/busybox

```
devbox:~/$ docker -H myvch.mycompany.com:2376 --tlsverify --tlscacert="vic-docker/ca.pem" --tlscert="vic-docker/cert.pem" --tlskey="vic-docker/key.pem" pull 10.2.40.40/test/busybox

Using default tag: latest
Pulling from test/busybox
c05511d7505a: Pull complete 
a3ed95caeb02: Pull complete 
Digest: sha256:85f3a6aadbb0f25e148d9cfbcf23fbb206f7e6159ea168c33ac51e76fdff4b8e
Status: Downloaded newer image for test/busybox:latest
```

This succeeds as it should.

### Example: docker pull busybox

```
devbox:~/$ docker -H myvch.mycompany.com:2376 --tlsverify --tlscacert="vic-docker/ca.pem" --tlscert="vic-docker/cert.pem" --tlskey="vic-docker/key.pem" pull busybox

Using default tag: latest
Access denied to unauthorized registry (docker.io) while VCH is in whitelist mode
```

An attempt to pull from docker hub fails with a message that access was denied while the VCH is in whitelist mode.