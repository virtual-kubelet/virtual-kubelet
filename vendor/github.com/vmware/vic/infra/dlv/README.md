# DLV Debugging of VCH servers

## Overview

This notes describes how to set up dlv remote debugging of VCH servers

## Building debug enabled binaries (non stripped)

Set the following environment variable
``` shell
export VIC_DEBUG_BUILD=true
```
this tells the makefile to build non stripped binaries

## Preparing the VCH for debugging

Ssh must be enabled on the VCH. To enabled it run the following command:
``` shell
vic-machine-linux debug --target <TARGET> --thumbprint <THUMBPRINT> --name <vch-name> --enable-ssh --password <PASSWORD>
```

The script **dlv-setup.sh**
must be used to set up the VCH to run dlv. It performs several tasks:
* opens the necessary ports in the iptables,
* copies the GO environment necessary to run dlv (from $GOROOT and $GOPATH),
* creates the attach and detach scripts that reside in /usr/local/bin in the VCH
* if requested it copies the ssh public keys into an authorized_hosts on the VCH (for using attach/detach without specifying the password)

The command needs the user to be authorized on the VCH, alternatively sshpass must installed. To install sshpass on ubuntu:
```shell
sudo apt-get install sshpass
```

The command also requires the address (or FQDN) of the VCH and the root password specified in vic-machine debug. The environment variables:
``` shell
DLV_TARGET_HOST and SSHPASS
```
can be used to pass that information to **dlv-setup.sh**. Alternatively the options: __-h__ and __-d__ can be used on the command line.
For instance:
``` shell
dlv-setup.sh -h <target IP address/FQDN> -p <root password>
```
The command allows the user to specify __-s__ to require the automatic copying of the public keys in $HOME/.ssh into 
the **authorized-hosts** .ssh file for root on the VCH. This option must be used carefully since it modifies access to the VCH.
The command also allows the user to specify __-a__ to skip authentication. This can be used when the user has ssh access to the
target VCH but does not know the root password.

## Launching dlv on the target host

To launch dlv and attach it to one of the VCH server run the command **dlv-ctl.sh**. The following target servers are supported:
* vic-init
* vic-admin
* port-layer
* docker-engine
* vic-machine
* virtual-kubelet

The scripts needs the IP address (or the FQDN) of the target VCH host and the root password. The same environment variables
and command line options as **dlv-setup.sh** are accepted. The password can be skipped if the __-s__ option was successfully used during
setup. The script takes two arguments:
* action: this can be either attach or detach
* target: this can be one of the VCH services listed above
For example:
``` shell
dlv-ctl.sh -h <target IP address/FQDN> -p <root password> attach vic-admin
```
launches dlv in headless mode and attaches it to vic-admin and prints out the port number on which dlv listens.
To detach you can use:
``` shell
dlv-ctl.sh -h <target IP address/FQDN> -p <root password> detach vic-admin
```
The script allows specifying the action through with a couple of additional options __-a__ (for attach) and __-d__ for detach.
For example:
``` shell
dlv-ctl.sh -h <target IP address/FQDN> -p <root password> -a vic-admin
```
Performs an attach. While
``` shell
dlv-ctl.sh -h <target IP address/FQDN> -p <root password> -d vic-admin
```
performs a detach

## Using Goland to perform remote debugging
After dlv is attached to the appropriate server, you can configure Goland to start debugging that process.
On the drop down list with the debugger configurations select: __Edit Configurations__. In the configuration tab 
click on the __+__ button to add a new configuration. Select __Go Remote__. Type in the the VCH IP address (or FQDN) and
the port number returned by the **dlv-ctl** attach command. The debugger should be able to connect to the server.

## Timeout issues while debugging
Consider for example the case in which a request is sent to the **port-layer** from  **docker-engine**. When the request
is received by the **port-layer** a breakpoint is hit. The developer next steps through the code in the **port-layer**
while the **docker-engine** is waiting for a response. This may cause the **docker-engine** to timeout and abort
or retry the request. Ideally when debugging is enabled all the timeouts should be increased to allow slower
response times. This has not yet been implemented. The current idea is to connect the extension of timeout duration
with the debug level specified at the time of VCH creation.

## Debugging vic-machine
The target __vic-machine__ has been added to debug the vic-machine remotely, in this case everything above applies with
the exception that the __vic-machine__ does not usually run on a VCH host.
