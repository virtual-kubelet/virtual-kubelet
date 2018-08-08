Test 19-3 - DRS Disabled - VM Placement
=======

# Purpose:
To verify that the VM placement feature specified works as expected in a vSphere environment without DRS, such as a ROBO deployment.
The current placement strategy is to avoid bad host selection, instead of selecting the "best" possible host.

# References:
1. [vSphere Remote Office and Branch Office](http://www.vmware.com/products/vsphere/remote-office-branch-office.html)
2. [VM Placement without DRS](https://github.com/vmware/vic/issues/7282)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation. This test should be executed in the following topologies and should have vSAN enabled.
* 1 PSC linking 2 VCs, each containing a 3-host cluster with VSAN enabled and DRS disabled, and a single host cluster.

See https://confluence.eng.vmware.com/display/CNA/VIC+ROBO for more details.

# Test Steps:
1. Deploy a vCenter testbed with DRS disabled
2. Install the VIC VCH appliance with compute resource set to the cluster
3. Deploy 2 containers that will consume resources predictably (e.g. the `progrium/stress` image)
4. Measure cluster metrics and gather resource consumption
5. Create a normal container using the `busybox` image
6. Relocate the `busybox` container to the same host as the VCH
7. Start the `busybox` container
8. Delete the VCH

# Expected Outcome:
* Step 1 should succeed
* Step 2 should succeed and the VCH should be placed on a host that satisfies the license and feature checks. The VCH's host should also meet the criteria defined in point 2 of [References](#references)
* Step 3 should succeed, with each stress container being relocated to its own host by the placement logic, separate from one another, and separate from the host containing the VCH
* Steps 4-5 should succeed and containers should be placed on ESX hosts in the cluster according to the criteria defined in point 2 of [References](#references)
* Steps 5 and 6 should succeed
* Step 7 should succeed, with the `busybox` container having been relocated from the ESX host containing the VCH to an ESX host that is does not contain the VCH or either of the stress containers

# Possible Problems:
None
