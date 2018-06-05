Test 5-18 - Datastore Cluster SDRS
=======

# Purpose:
To verify that VIC works properly when a VCH is installed on a datastore cluster

# References:
[1 - Creating a Datastore Cluster](https://pubs.vmware.com/vsphere-51/index.jsp?topic=%2Fcom.vmware.vsphere.resmgmt.doc%2FGUID-598DF695-107E-406B-9C95-0AF961FC227A.html)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Create a storage pod or datastore cluster folder
3. Move several shared datastores into the datastore cluster
4. Install the VIC appliance into the cluster using the new datastore cluster
5. Run a variety of docker operation on the VCH

# Expected Outcome:
All test steps should complete without error on the datastore cluster

# Possible Problems:
None
