Test 19-4 - DRS Disabled License Features
=======

# Purpose:
To verify that vic-machine create checks for the DRS setting and command flags specific to resource pools in a vSphere environment without DRS, such as a ROBO deployment. If a VCH is being installed in a DRS-disabled environment, vic-machine create should warn that DRS is disabled and mention that any resource pool options supplied in the command will be ignored as they are not applicable in this environment.

# References:
1. [vSphere Remote Office and Branch Office](http://www.vmware.com/products/vsphere/remote-office-branch-office.html)
2. [Provide License and Feature Check](https://github.com/vmware/vic/issues/7277)
3. [vic-machine to provide license and feature check](https://github.com/vmware/vic/issues/7275)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation. This test should be executed in a vCenter environment with a cluster that has DRS turned off.

See https://confluence.eng.vmware.com/display/CNA/VIC+ROBO for more details.

# Test Steps:
1. Deploy a vCenter testbed with DRS disabled
2. Using vic-machine create, install a VCH with compute resource set to the cluster where DRS is off. Also supply options that are specific to resource pools: cpu, cpu-shares, cpu-reservation, memory, memory-shares and memory-reservation.
3. Delete the VCH

# Expected Outcome:
* All test steps should complete without error
* Step 2's output should contain a message stating that DRS is disabed and that the provided resource pool options will be ignored.

# Possible Problems:
None
