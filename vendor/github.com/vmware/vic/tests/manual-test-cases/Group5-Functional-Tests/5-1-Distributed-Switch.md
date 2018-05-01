Test 5-1 - Distributed Switch
=======

# Purpose:
To verify the VIC appliance works in a variety of different vCenter networking configurations

# References:
[1 - VMware Distributed Switch Feature](https://www.vmware.com/products/vsphere/features/distributed-switch.html)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter in Nimbus
2. Deploy three new ESXi hosts with 2 NICs each in Nimbus:
```nimbus-esxdeploy --nics=2 esx-1 3620759```
```nimbus-esxdeploy --nics=2 esx-2 3620759```
```nimbus-esxdeploy --nics=2 esx-3 3620759```
3. After setting up your govc environment based on the new vCenter deployed, create a new datacenter:
```govc datacenter.create ha-datacenter```
4. Add each of the new hosts to the vCenter:
```govc host.add -hostname=<ESXi IP> -username=<USER> -dc=ha-datacenter -password=<PW> -noverify=true```
5. Create a new distributed switch:
```govc dvs.create -dc=ha-datacenter test-ds```
6. Create three new distributed switch port groups for management and vm network traffic:
```govc dvs.portgroup.add -nports 12 -dc=ha-datacenter -dvs=test-ds management```
```govc dvs.portgroup.add -nports 12 -dc=ha-datacenter -dvs=test-ds vm-network```
```govc dvs.portgroup.add -nports 12 -dc=ha-datacenter -dvs=test-ds bridge```
7. Add the three ESXi hosts to the portgroups:
```govc dvs.add -dvs=test-ds -pnic=vmnic1 <ESXi IP1>```
```govc dvs.add -dvs=test-ds -pnic=vmnic1 <ESXi IP2>```
```govc dvs.add -dvs=test-ds -pnic=vmnic1 <ESXi IP3>```
8. Deploy VCH Appliance to the new vCenter:
```bin/vic-machine-linux create --target=<VC IP> --user=Administrator@vsphere.local --image-store=datastore1 --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso --generate-cert=false --password=Admin\!23 --force=true --bridge-network=bridge --compute-resource=/ha-datacenter/host/<ESXi IP 1>/Resources --public-network=vm-network --name=VCH-test```
9. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
* When you add an ESXi host to the vCenter it will overwrite its datastore name from datastore1 to datastore1 (n)
* govc requires an actual password so you need to change the default ESXi password before Step 4
* govc doesn't seem to be able to force a host NIC over to the new distributed switch, thus you need to create the ESXi hosts with 2 NICs in order to use the 2nd NIC for the distributed switch
