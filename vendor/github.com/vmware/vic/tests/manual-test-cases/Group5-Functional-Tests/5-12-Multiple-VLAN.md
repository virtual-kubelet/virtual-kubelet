Test 5-12 - Multiple VLAN
=======

# Purpose:
To verify the VIC appliance works when the vCenter appliance has multiple portgroups on different VLANs within the datacenter

# References:
[1 - VMware vCenter Server Availability Guide](http://www.vmware.com/files/pdf/techpaper/vmware-vcenter-server-availability-guide.pdf)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a distributed virtual switch with 3 portgroups on all different VLANs
2. Install the VIC appliance into one of the clusters
3. Run a variety of docker commands on the VCH appliance
4. Uninstall the VIC appliance
5. Deploy a new vCenter with a distributed virtual switch with 3 portgroups two on the same VLAN and one on a different VLAN
6. Install the VIC appliance into one of the clusters
7. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
Each VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
None
