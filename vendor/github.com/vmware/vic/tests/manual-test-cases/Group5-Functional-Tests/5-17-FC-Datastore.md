Test 5-17 - FC Datastore
=======

# Purpose:
To verify that VIC works properly when a VCH is installed on an Fibre Channel based datastore

# References:
[1 - Add Fibre Channel Storage](https://pubs.vmware.com/vsphere-4-esx-vcenter/index.jsp?topic=/com.vmware.vsphere.server_configclassic.doc_41/esx_server_config/configuring_storage/t_add_fibre_channel_storage.html)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Deploy an FC server
3. Create a new datastore out of an FC lun on the FC server
4. Install the VIC appliance into the cluster using the new FC based datastore
5. Run a variety of docker operation on the VCH

# Expected Outcome:
All test steps should complete without error on the FC based datastore

# Possible Problems:
None
