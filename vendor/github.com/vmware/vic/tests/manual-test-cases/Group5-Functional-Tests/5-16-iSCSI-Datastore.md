Test 5-16 - iSCSI Datastore
=======

# Purpose:
To verify that VIC works properly when installed on an iSCSI based datastore

# References:
[1 - Configuring iSCSI Adapters and Storage](https://pubs.vmware.com/vsphere-55/index.jsp?topic=%2Fcom.vmware.vsphere.storage.doc%2FGUID-C476065E-C02F-47FA-A5F7-3B3F2FD40EA8.html)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster using iSCSI based datastores
2. Install the VIC appliance into the cluster specifying one of the iSCSI datastores as the image-store
3. Run a variety of docker commands on the VCH

# Expected Outcome:
Each step should result in success and each of the docker commands should not return an error

# Possible Problems:
None
