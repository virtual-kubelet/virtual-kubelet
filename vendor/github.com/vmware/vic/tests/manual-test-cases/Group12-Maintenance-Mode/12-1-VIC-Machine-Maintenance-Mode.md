Test 12-1 VIC Machine Maintenance Mode
=======

# Purpose:
To verify the VIC appliance provides a reasonable error message when installing or deleting it from a host in maintenance mode

# References:
[1- VMware Maintenance Mode](https://pubs.vmware.com/vsphere-4-esx-vcenter/index.jsp?topic=/com.vmware.vsphere.resourcemanagement.doc_41/using_drs_clusters_to_manage_resources/c_using_maintenance_mode.html)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Put the vSphere server into maintenance mode
2. Issue a vic-machine create command to attempt to install VIC into the server while it is in maintenance mode
3. Instruct the vSphere server to exit maintenance mode
4. Issue a vic-machine create command to install VIC into the server
5. Instruct the vSphere server to enter maintenance mode
6. Issue a vic-machine delete command to attempt to delete VIC from the server while it is in maintenance mode

# Expected Outcome:
* For Step 2, the VCH appliance should deploy with an error that indicates the reason why it cannot install into a server in maintenance mode
* For Step 6, the vic-machine delete command should return with an error indicating the reason why it cannot delete the VCH while the server is in maintenance mode

# Possible Problems:
None
