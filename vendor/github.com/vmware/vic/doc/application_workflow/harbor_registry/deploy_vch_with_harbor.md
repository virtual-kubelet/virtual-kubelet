# Deploying vSphere Integrated Container Engine with Harbor

## Prerequisite

Harbor requires 60GB or more free space on your datastore.

## Workflow

We will use VIC Engine 0.8.0 and Harbor 0.5.0 for this example.  We will use Ubuntu as OS on our user machine.

If no server certificate and private key are provided during installation, Harbor will self generate these.  It will also provide a self-generated CA (certificate authority) certificate if no server certificate and private key are provided during installation.  The OVA installation guide for Harbor can be found in the [Harbor docs](https://github.com/vmware/harbor/blob/master/docs/installation_guide_ova.md).  Harbor requires both an IP address and FQDN (fully qualified domain name) for the the server.  There is also a DHCP install method available for debugging purposes, but it is not a recommended production deployment model.

We will assume a Harbor instance has been installed without server certificate and private key.  We will also assume we have downloaded the CA cert using the Harbor instuctions.  The last steps left to get Harbor working with vSphere Integrated Container Engine is to update standard docker with the Harbor CA cert and deploy a new VCH with the CA cert.  The instructions are provided below.
<br><br>

## Update the user working machine with the CA.crt for standard docker

We must update the standard docker on our laptop so it knows of our CA certificate.  Docker can look for additional CA certificates outside of the OS's CA bundle folder if we put new CA certificates in the right location, documented [here](https://docs.docker.com/engine/security/certificates/).

We create the necessary folder, copy our CA cert file there, and restart docker.  This should be all that is necessary.  We take the additional steps to verify that we can log onto our Harbor server.

```
loc@Devbox:~/mycerts$ sudo su
[sudo] password for loc: 
root@Devbox:/home/loc/mycerts# mkdir -p /etc/docker/certs.d/<Harbor FQDN>
root@Devbox:/home/loc/mycerts# mkdir -p /etc/docker/certs.d/<Harbor IP>
root@Devbox:/home/loc/mycerts# cp ca.crt /etc/docker/certs.d/<Harbor FQDN>/
root@Devbox:/home/loc/mycerts# cp ca.crt /etc/docker/certs.d/<Harbor IP>/
root@Devbox:/home/loc/mycerts# exit
exit
loc@Devbox:~/mycerts$ sudo systemctl daemon-reload
loc@Devbox:~/mycerts$ sudo systemctl restart docker

loc@Devbox:~$ docker logout <Harbor FQDN>
Remove login credentials for <Harbor FQDN>

loc@Devbox:~$ docker logout <Harbor IP>
Remove login credentials for <Harbor IP>

loc@Devbox:~$ docker login <Harbor FQDN>
Username: loc
Password: 
Login Succeeded

loc@Devbox:~$ docker login <Harbor IP>
Username: loc
Password: 
Login Succeeded

loc@Devbox:~$ docker logout <Harbor FQDN>
Remove login credentials for <Harbor FQDN>

loc@Devbox:~$ docker logout <Harbor IP>
Remove login credentials for <Harbor IP>
```
Notice we create folders for both FQDN and IP in the docker cert folder and copy the CA cert to both.  This will allow us to log into the Harbor from Docker using both FQDN and IP address.
<br><br>

## Install a VCH with the new CA certificate

In this step, we deploy a VCH and specify our CA cert via a --registry-ca parameter in vic-machine.  This parameter is a list, meaning we can easily add multiple CA certs by specifying multiple --registry-ca parameters.

For simplicity, we will install a VCH with the --no-tls flag.  This indicates we will not need TLS from a docker CLI to the VCH.  This does NOT imply that access to Harbor will be performed without TLS.

```
root@Devbox:/home/loc/go/src/github.com/vmware/vic/bin# ./vic-machine-linux create --target=<vCenter_IP> --image-store="vsanDatastore" --name=vic-docker --user=root -password=<vCenter_password> --compute-resource="/dc1/host/cluster1/Resources" --bridge-network DPortGroup --force --no-tls --registry-ca=ca.crt

WARN[2016-11-11T11:46:37-08:00] Configuring without TLS - all communications will be insecure

...

INFO[2016-11-11T11:47:57-08:00] Installer completed successfully             
```
<br>

Proceed to [Post-Install Usage](post_install_usage.md) for examples of how to use this deployed VCH with Harbor.