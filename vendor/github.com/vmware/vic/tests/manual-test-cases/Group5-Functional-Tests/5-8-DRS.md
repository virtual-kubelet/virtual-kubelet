Test 5-8 - DRS
=======

# Purpose:
To verify the VIC appliance detects when DRS should be enabled and fuctions properly when used with DRS

# References:
[1 - Managing DRS Clusters](https://pubs.vmware.com/vsphere-50/index.jsp?topic=%2Fcom.vmware.wssdk.pg.doc_50%2FPG_Ch13_Resources.15.8.html)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with 3 ESXi hosts in a cluster but with DRS disabled
2. Attempt to install a VCH appliance into the cluster
3. Enable DRS on the cluster
4. Re-attempt to install a VCH appliance into the cluster
5. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The first VCH appliance install should provide an error indicating that DRS must be enabled, the second VCH appliance install should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
None
