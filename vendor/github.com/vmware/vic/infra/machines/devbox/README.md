# Vagrant Dev Box

## Overview

This box is an Ubuntu 16.04 VM with the following setup by default:

* Docker daemon with port forwarded to the Fusion/Workstation host at localhost:12375

* Go toolchain

* Additional tools (lsof, strace, etc)

## Requirements

* Vagrant (https://www.vagrantup.com/downloads.html)

* VMware Fusion or Workstation

* Vagrant Fusion or Workstation license (https://www.vagrantup.com/vmware)

## Provisioning

All files matching _provision*.sh_ in this directory will be applied by the Vagrantfile, you can symlink custom scripts
if needed.  The scripts are not Vagrant specific and can be applied to a VM running on ESX for example.

## Fusion/Workstation host usage

The following commands can be used from your Fusion or Workstation host.

### Shared Folders

By default your *GOPATH* is shared with the same path as the host.  This is useful if your editor runs
on the host, then errors on the guest with filename:line info have the same path.  For example, when running the
following command within the top-level project directory:

``` shell
vagrant ssh -- make -C $PWD all
```

### Create the VM

``` shell
vagrant up
```

### SSH Access

``` shell
vagrant ssh
```

### Docker Access

``` shell
DOCKER_HOST=localhost:12375 docker ps
```

### Stop the VM

``` shell
vagrant halt
```

### Restart the VM

``` shell
vagrant reload
```

### Provision

After you've done a `vagrant up`, the provisioning can be applied without reloading via:

``` shell
vagrant provision
```

### Delete the VM

``` shell
vagrant destroy
```

## VM guest usage

To open a bash term in the VM, use `vagrant ssh`.

The following commands can be used from devbox VM guest.

``` shell
cd $GOPATH/src/github.com/vmware/vic
```

### Local Drone CI test

``` shell
drone exec
```

## Devbox on ESX

The devbox can be deployed to ESX, the same provisioning scripts are applied:

``` shell
./deploy-esx.sh
```

### SSH access

``` shell
ssh-add ~/.vagrant.d/insecure_private_key
vmip=$(govc vm.ip $USER-ubuntu-1604)
ssh vagrant@$vmip
```

### Shared folders

You can share your folder by first exporting via NFS:

```
echo "$HOME/vic $(govc vm.ip $USER-ubuntu-1604) -alldirs -mapall=$(id -u):$(id -g)" | sudo tee -a /etc/exports
sudo nfsd restart
```

Then mount within the ubuntu VM:

``` shell
ssh vagrant@$vmip sudo mkdir -p $HOME/vic
ssh vagrant@$vmip sudo mount $(ipconfig getifaddr en1):$HOME/vic $HOME/vic
```
Note that you may need to use enN depending on the type of connection you have - use ifconfig to verify.
Note also that nfs-common is not installed in the box by default.

You can also mount your folder within ESX:

``` shell
govc datastore.create -type nfs -name nfsDatastore -remote-host $(ipconfig getifaddr en1) -remote-path $HOME/vic
esxip=$(govc host.info -json | jq -r '.HostSystems[].Config.Network.Vnic[] | select(.Device == "vmk0") | .Spec.Ip.IpAddress')
ssh root@$esxip mkdir -p $HOME
ssh root@$esxip /vmfs/volumes/nfsDatastore $HOME/vic
```

Add `$esxip` to /etc/exports and restart nfsd again.
