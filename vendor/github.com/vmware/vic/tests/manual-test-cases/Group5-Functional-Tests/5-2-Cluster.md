Test 5-2 - Cluster
=======

# Purpose:
To verify the VIC appliance works when the vCenter appliance is using a cluster

# References:
[1 - VMware vCenter Server Availability Guide](http://www.vmware.com/files/pdf/techpaper/vmware-vcenter-server-availability-guide.pdf)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with 3 ESXi hosts in a cluster:  
```govc cluster.create cls```
2. Install the VIC appliance into the cluster using the cluster path as the compute resource:  
```
--compute-resource /ha-datacenter/host/cls
```
3. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
None
