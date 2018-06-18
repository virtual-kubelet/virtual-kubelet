# VIC Unified Management OVA specification

<!-- TOC -->

- [VIC Unified Management OVA specification](#vic-unified-management-ova-specification)
  - [Appliance Specification](#appliance-specification)
  - [Filesystem layout](#filesystem-layout)
  - [Network layout](#network-layout)
  - [Components logging](#components-logging)
  - [Components startup / shutdown](#components-startup--shutdown)
  - [OVF Properties list](#ovf-properties-list)
    - [Appliance level properties](#appliance-level-properties)
    - [Harbor](#harbor)
    - [Admiral](#admiral)
  - [Upgrade Strategy](#upgrade-strategy)
    - [Upgrade from previous Harbor-only OVA](#upgrade-from-previous-harbor-only-ova)
    - [Future version upgrade](#future-version-upgrade)

<!-- /TOC -->

**User Statement**: As a customer of vSphere Integrated Containers, I want an easy way to install all the product components on my vSphere infrastructure, with a single download, and using a familiar workflow that leverages my competencies on vSphere. When an upgrade or a patch is released, I want to follow a simple guide to perform the upgrade of one or more components of vSphere Integrated Containers.

**Details**: to accomplish these goals, we need to create a single OVA that installs all the VIC components and performs their configuration using OVF properties, resulting in a running service that can be accessed by the vSphere admin for further configuration.

**Acceptance Criteria**:

1. The vSphere administrator downloads the VIC Management OVA from a known location
2. The vSphere administrator deploys the VIC Management OVA from the vSphere client, provides values for Harbor and Admiral configuration options
3. The deployment should also tie the Harbor and Admiral instances together
4. Once deployed the vSphere administrator should be able to
    - Access the Admiral / Harbor integrated UI
    - SSH into the machine to run vic-machine commands
5. The vSphere administrator should also be able to download the VIC Engine tarball from the OVA.

## Appliance Specification

The Appliance is based on Photon OS, version 1.0 (build 62c543d), it's built using [packer](https://packer.io) and produces two main artifacts, one for local development (packaged as a Vagrant box) and a releasable OVA artifact.

The OVA appliance build process is codified in a JSON document that contains the packer specification for the build, the build will be conducted as part of the existing CI system in use by the VIC Engine project, it will also be possible to build locally on the developer workstation to aid with the development.

All the components integrated in the appliance (Harbor, Admiral, Lightwave) will be run as Docker containers, using Docker runtime version 1.12.6, Docker Compose will also be available to orchestrate discrete services inside the components if needed.

Use of non-contained processes inside the appliance is severely discouraged.

## Filesystem layout

The appliance will ship with two disks, internally named `system` and `data`, `system` only contains the operating system and the docker containers of the services, `data` will hold all the user-generated data and data that has to be persisted by the components.

The data disk will be initialized on first boot using a fixed size, the filesystem will be automatically expanded by a startup script if the underlying disk size has changed (e.g. the operator shuts down the VM and increase the size of the data disk from vSphere), this will allow the operator to extend the size of the components data store up to the limit of a single VMDK.

## Network layout

The appliance will have a single IP, which will be configured using OVF properties upon deployment, network changes will be automatically picked up by the appliance startup script if the operator changes OVF properties post-deployment.

In order to ease deployment and lower install complexity, all the services will be run on different ports, there will be no effort to tie them together using a reverse proxy.

## Components logging

All the docker containers will log locally to syslog, using the docker syslog driver, this will give a familiar environment for VMware admins to troubleshoot issues with the appliance.

## Components startup / shutdown

Startup and shutdown of the components will be performed with systemd units, each component will use a single systemd unit, components that require service orchestration must use docker-compose to specify relations and startup/shutdown order.

## OVF Properties list

The OVF properties list includes all the fields that the user has to input upon deployment for initial configuration, all the parameters (except when noted) are changeable post deployment by accessing the VM properties on vCenter, startup scripts will reconfigure the appliance with the new parameters.

### Appliance level properties

- Root Password - `appliance.root_pwd`: The initial password of the root user. Subsequent changes of password should be performed in operating system. (8-128 characters)
- Permit Root Login - `appliance.permit_root_login`: Specifies whether root user can log in using SSH. (disabled by default)
- Network IP Address - `network.ip0`: The IP address of this interface. Leave blank if DHCP is desired.
- Network Netmask - `network.netmask0`: The netmask or prefix for this interface. Leave blank if DHCP is desired.
- Default Gateway - `network.gateway`: The default gateway address for this VM. Leave blank if DHCP is desired.
- Domain Name Servers - `network.DNS`: The domain name server IP Address for this VM(comma separated). Leave blank if DHCP is desired.
- Domain Search Path - `network.searchpath`: The domain search path(comma or space separated domain names) for this VM. Leave blank if DHCP is desired.
- Domain Name - `network.domain`: The domain name of this VM. Run command man resolv.conf for more explanation. Leave blank if DHCP is desired or the domain name is not needed for static IP.
- Email Server - `appliance.email_server`: The mail server to send out emails.
- Email Server Port - `appliance.email_server_port`: The port of mail server.
- Email Username - `appliance.email_username`: The user from whom the password reset email is sent. Usually this is a system email address.
- Email Password - `appliance.email_password`: The password of the user from whom the password reset email is sent.
- Email From - `appliance.email_from`: The name of the email sender.
- Email SSL `appliance.email_ssl`: Whether to enable secure mail transmission.

### Harbor

- Deploy - `harbor.deploy`: Specifies whether Harbor is enabled on the appliance.
- Port - `harbor.port`: The port on which Harbor will bind the NGINX frontend, defaults to 443
- Harbor Admin Password - `harbor.admin_password`: The initial password of Harbor admin. It only works for the first time when Harbor starts. It has no effect after the first launch of Harbor. Change the admin password from UI after launching Harbor. (8-20 characters)
- Database Password - `harbor.db_password`: The initial password of the root user of MySQL database. Subsequent changes of password should be performed in operating system. (8-128 characters)
- Garbage Collection - `harbor.gc_enabled`: When setting this to true, Harbor performs garbage collection everytime it boots up.
- SSL Cert - `harbor.ssl_cert`: Paste in the content of a certificate file. Leave blank for a generated self-signed certificate.
- SSL Cert Key - `harbor.ssl_cert_key`: Paste in the content of a certificate key file. Leave blank for a generated key.

### Admiral

- Deploy - `admiral.deploy`: Specifies whether Admiral is enabled on the appliance.
- Port - `admiral.port`: The (secure) port on which Admiral will bind the HTTP service, defaults to 8443
- SSL Cert - `admiral.ssl_cert`: Paste in the content of a certificate file. Leave blank for a generated self-signed certificate.
- SSL Cert Key - `admiral.ssl_cert_key`: Paste in the content of a certificate key file. Leave blank for a generated key.

## Upgrade Strategy

### Upgrade from previous Harbor-only OVA

Upgrade from previous Harbor-only OVA will consist in attaching both the system and data disk from the old Harbor-only OVA as additional disks of the newly deployed VIC OVA.

The procedure will roughly look like this:

1. Shutdown of the old Harbor-only Appliance.
2. Deploy of the new VIC Appliance.
    - New VIC Appliance deployed with all the old credentials (DB Password, Admin Password, etc...)
3. User will attach both disks from the Harbor-only Appliance to the newly deployed VIC Appliance as SCSI 0:2 and SCSI 0:3 (order is not important).
4. During bootup, a systemd unit will recognize the external data and, if there is no new data existing in the VIC appliance, will proceed with the migration procedure.
5. The migration procedure will copy the data over to the new data disk.
6. A Migration script provided by the Harbor team will perform some data transformation on the migrated data and will proceed with the customization of the configuration (which is stored in the system disk of the old Harbor-only appliance).
7. The VI Admin will follow a progress bar on the console of the VM that will show progress of the procedure.
8. Once the procedure is done, a message will appear, asking the user to shutdown and detach the old Harbor-only disks from the new OVA.
9. The user will then start the new VIC Appliance again and the procedure will be complete.

### Future version upgrade

Upgrade strategy (to subsequent version of the VIC Appliance) will include moving the data disk to a newly deployed OVA, the startup scripts will recognize existing data on the data disk and new versions of the components will start up with the previous data attached and available, it will be up to the individual components to do perform any application-level upgrade procedure (i.e. database schema updates) going forward.