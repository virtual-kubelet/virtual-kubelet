Test 1-05 - Docker Start
=======

# Purpose:
To verify that docker start command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/start/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker create -it busybox /bin/top to VIC appliance
3. Issue docker start <containerID>
4. Issue docker create vmware/photon
5. Issue docker start vmware/photon <containerID>
6. Issue docker start fakeContainer
7. Create a container, remove it's ethernet adapter, then start the container
8. Create and start 5 busybox containers running /bin/top serially
9. Create and start 5 ubuntu containers running /bin/top serially
10. Create and start 5 busybox containers running /bin/top all at once
11. Run a container with a test-network, stop the container, remove the test-network, then start the container again
12. Issue docker start -a <containerID> to a busybox ls container

# Expected Outcome:
* Commands 1-5 should all return without error and respond with the container ID
* After commands 3 and 5 verify that the containers are running
* Step 6 should result in the VIC appliance returning the following error:
```
Error response from daemon: No such container: fakeContainer
Error: failed to start containers: fakeContainer
```
* Step 7 should result in an error message stating unable to wait for process launch status
* Steps 8-11 should all result in all containers succeeding and not throwing any errors
* Step 11 should result in the VIC appliance returning the following error:
```
Error response from daemon: Server error from portlayer: network test-network not found
Error: failed to start containers: containerID
```
* Step 12 should succeed and provide the output from the ls command to the screen

# Possible Problems:
None
