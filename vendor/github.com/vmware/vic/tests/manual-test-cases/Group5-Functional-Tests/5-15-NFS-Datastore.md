Test 5-15 - NFS Datastore
=======

# Purpose:
To verify that VIC works properly when a VCH is installed on an NFS based datastore

# References:
[1 - Best practices for running VMware vSphere on NFS](http://www.vmware.com/content/dam/digitalmarketing/vmware/en/pdf/techpaper/vmware-nfs-bestpractices-white-paper-en.pdf)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Deploy an NFS server
3. Create a new datastore out of a NFS share on the NFS server
4. Install the VIC appliance into the cluster using the new NFS based datastore
5. Run a variety of docker operation on the VCH

# Expected Outcome:
All test steps should complete without error on the NFS based datastore

# Possible Problems:
None
