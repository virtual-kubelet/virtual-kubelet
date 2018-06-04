Test 19-4 - ROBO License Features
=======

# Purpose:
To verify that the license and feature checks required for a ROBO Advanced environment are displayed and updated on VCH Admin.

# References:
1. [vSphere Remote Office and Branch Office](http://www.vmware.com/products/vsphere/remote-office-branch-office.html)
2. [Provide License and Feature Check](https://github.com/vmware/vic/issues/7277)
3. [vic-admin to report on license and feature compliance](https://github.com/vmware/vic/issues/7276)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation. This test should be executed in the following topologies and should have vSAN enabled.
* 1 vCenter host with 3 clusters, where 1 cluster has 1 ESXi host and the other 2 clusters have 3 ESXi hosts each
* 2 vCenter hosts connected with ELM, where each vCenter host has a cluster/host/datacenter topology that emulates a customer environment (exact topology TBD)

See https://confluence.eng.vmware.com/display/CNA/VIC+ROBO for more details.

# Test Steps:
1. Deploy a ROBO Advanced vCenter testbed for both environments above
2. Install a VCH on vCenter
3. Visit the VCH Admin page and verify that the License and Feature Status sections show that required license and features are present
4. Assign a more restrictive license such as ROBO Standard or Standard that does not have the required features (VDS, VSPC) to vCenter
5. Assign the above license to each of the hosts within the vCenter cluster
6. Refresh the VCH Admin page and verify that the License and Feature Status sections show that required license and features are not present
7. Delete the VCH

# Expected Outcome:
* All test steps should complete without error

# Possible Problems:
None
