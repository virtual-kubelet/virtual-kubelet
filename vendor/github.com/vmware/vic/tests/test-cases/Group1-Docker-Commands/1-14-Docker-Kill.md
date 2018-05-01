Test 1-14 - Docker Kill
=======

# Purpose:
To verify that docker kill command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/kill/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker create busybox sleep 300 to the VIC appliance
3. Issue docker start <containerID> to the VIC appliance
4. Issue docker kill <containerID> to the VIC appliance
5. Issue docker start <containerID> to the VIC appliance
6. Issue docker kill -s HUP <containerID> to the VIC appliance
7. Issue docker kill -s TERM <containerID> to the VIC appliance
8. Issue docker kill fakeContainer to the VIC appliance
9. Issue docker create nginx to the VIC appliance
10. Issue docker start <containerID to the VIC appliance
11. Issue docker kill <containerID> to the VIC appliance

# Expected Outcome:
* Steps 2-7 should all return without error and provide the container ID in the response
* Step 4 should result in the container stopping immediately
* Step 6 should result in the container continuing to run
* Step 7 should result in the container stopping immediately
* Step 8 should result in an error and the following message:
```
Failed to kill container (fakeContainer): Error response from daemon: Cannot kill container fakeContainer: No such container: fakeContainer
```
* Step 11 should result in the container stopped

# Possible Problems:
None
