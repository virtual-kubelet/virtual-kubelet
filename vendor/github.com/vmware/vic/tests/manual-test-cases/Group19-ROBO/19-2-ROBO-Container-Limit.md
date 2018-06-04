Test 19-2 - ROBO - Container VM Limit
=======

# Purpose:
To verify that the total container VM limit feature works as expected in a vSphere ROBO Advanced environment.

# References:
1. [vSphere Remote Office and Branch Office](http://www.vmware.com/products/vsphere/remote-office-branch-office.html)
2. [Limit total allowed containerVMs per VCH](https://github.com/vmware/vic/issues/7273)
3. [vic-machine inspect to report configured containerVM limit](https://github.com/vmware/vic/issues/7284)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation. This test should be executed in the following topologies and should have vSAN enabled.
* 1 vCenter host with 3 clusters, where 1 cluster has 1 ESXi host and the other 2 clusters have 3 ESXi hosts each
* 2 vCenter hosts connected with ELM, where each vCenter host has a cluster/host/datacenter topology that emulates a customer environment (exact topology TBD)

See https://confluence.eng.vmware.com/display/CNA/VIC+ROBO for more details.

# Test Steps:
1. Deploy a ROBO Advanced vCenter testbed for both environments above
2. Install a VCH on a particular cluster in vCenter with a container VM limit of `y`
3. Use vic-machine inspect to verify the set container VM limit
4. Visit the VCH Admin page and verify the container VM limit is displayed in the VCH Info section
5. Create and run `y` (long-running) containers with the VCH
6. Create another (long-running) container so as to have `y+1` total containers, but only `y` running containers
7. Attempt to run the container created in Step 6
8. Delete one of the containers created in Step 5
9. Start the container created in Step 6
10. Create (don't run) `x` (`x` < `y`) long-running containers to have a total of `y + x` containers
11. From the `y` already-running containers, assemble a list of `x` containers (using `docker ps -q` for example)
12. Concurrently start the containers in Step 10 and concurrently delete the containers in Step 11
13. Check the number of running containers with `docker ps -q`
14. Use vic-machine configure to increase the container VM limit (new limit = `z`)
15. Use vic-machine inspect to verify the new container VM limit
16. Visit the VCH Admin page and verify the container VM limit is displayed in the VCH Info section
17. Create and run more containers and verify that up to a total of `z` containers can be run
18. Use vic-machine configure to set the limit to lower than the current number of containers running
19. Attempt to run more containers
20. Delete/stop some containers so the current container VM count is lower than the set limit
21. Attempt to create/run more containers until the set limit
22. Delete the VCH

# Expected Outcome:
* Steps 1 and 2 should succeed
* Step 3's output should indicate the limit set in Step 2
* Steps 4 and 5 should succeed
* Step 6 should succeed since the container limit applies to running containers
* Step 7 should fail since the container limit applies to running containers
* Steps 8-11 should succeed
* In Step 12, depending on the order in which operations are processed, a container should fail to start if it breaches the running container limit
* In Step 13, the number of running containers should be `<= y`, the current running container limit
* Step 14 should succeed
* Step 15's output should indicate the limit set in Step 14
* Step 16 should show the new container VM limit
* Step 17 should succeed
* Step 18 should succeed - exact behavior of existing running containers is TBD
* Step 19 should fail and should receive an error upon attempting to start "surplus" container VMs (exact behavior of existing running containers TBD)
* Steps 20-22 should succeed

# Possible Problems:
None
