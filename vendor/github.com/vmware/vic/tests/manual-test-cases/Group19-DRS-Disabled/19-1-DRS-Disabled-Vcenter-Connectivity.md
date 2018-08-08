Test 19-1 - DRS Disabled vCenter Connectivity
=======

# Purpose:
To verify that the applications deployed in containerVMs in a disabled DRS environment are functional when the ESXi(s) hosting the containerVMs are disconnected from the vSphere host. This test exercises the WAN connectivity and resiliency support for an environment that could represent a customer's cluster topology with remote elements, such as a ROBO deployement or an environment where one or more VC(s) and/or ESX hosts are in different locations.

# References:
1. [vSphere Remote Office and Branch Office](http://www.vmware.com/products/vsphere/remote-office-branch-office.html)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation. This test should be executed in the following topologies and should have vSAN enabled.
* 1 vCenter host with 3 clusters, where 1 cluster has 1 ESXi host and the other 2 clusters have 3 ESXi hosts each
* 2 vCenter hosts connected with ELM, where each vCenter host has a cluster/host/datacenter topology that emulates a customer environment (exact topology TBD)

See https://confluence.eng.vmware.com/display/CNA/VIC+ROBO for more details.

# Test Steps:
1. Deploy a ROBO Advanced vCenter testbed for both environments above
2. Deploy the VIC appliance OVA on vCenter for testing VIC Product as well
3. Once the OVA is powered on and initialized, populate Harbor with some images
4. Install a VCH on a cluster in vCenter
5. Log in to the Admiral UI
6. Add the VCH to the default project in Admiral
7. Using Admiral, deploy some containers through the VCH
8. Create and start some container services such as nginx, wordpress or a database
9. Run a multi-container application exercising network links with docker-compose
10. To simulate a WAN link outage, _abruptly_ disconnect each ESX host in the cluster from vCenter (possibly by changing firewall rules)
11. Verify that the containers/services/applications started in Steps 7-9 are still alive and responding
12. Pull an image from Harbor
13. Create/start a container
14. Re-connect all hosts in the cluster to vCenter
15. Create/start a container
16. Delete the VCH

# Expected Outcome:
* Steps 1-12 should succeed
* Step 13 should fail since the vCenter host is disconnected from the VCH's host
* Steps 14-16 should succeed

# Possible Problems:
None
