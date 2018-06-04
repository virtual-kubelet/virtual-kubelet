Test 5-3 - Enhanced Linked Mode
=======

# Purpose:
To verify the VIC appliance works in when the vCenter appliance is using enhanced linked mode

# References:
[1 - VMware vCenter Server Availability Guide](http://www.vmware.com/files/pdf/techpaper/vmware-vcenter-server-availability-guide.pdf)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy two new vCenters in Nimbus each with one ESXi host configured
2. Establish an enhanced link between the two vCenters
3. Deploy VCH Appliance to the first vCenter cluster, referencing the cluster's own compute resources
4. Run a variety of docker commands on the VCH appliance
5. Deploy VCH Appliance to the first vCenter cluster, but reference the second cluster's resources through the enhanced link
6. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error in both scenarios and each of the docker commands executed against it should return without error

# Possible Problems:
None
