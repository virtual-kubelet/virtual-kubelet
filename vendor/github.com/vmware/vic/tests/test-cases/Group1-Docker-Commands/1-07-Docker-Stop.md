Test 1-07 - Docker Stop
=======

# Purpose:
To verify that docker stop command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/stop/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker create busybox sleep 30 to the VIC appliance
3. Issue docker stop <containerID> to the VIC appliance
4. Issue docker start <containerID> to the VIC appliance
5. Issue docker stop <containerID> to the VIC appliance
6. Issue docker start <containerID> to the VIC appliance
7. Issue docker stop -t 2 <containerID> to the VIC appliance
8. Issue docker stop fakeContainer to the VIC appliance
9. Create a new container, start the container using govc/UI, attempt to stop the container using docker stop
10. Start a new container, stop it, then attempt to restart it again
11. Start a new container, stop it with Docker 1.13 CLI

# Expected Outcome:
* Steps 2-8 should each complete without error, and the response should be the containerID
* Step 5 should take 10 seconds to complete
* Step 7 should take 2 seconds to complete
* Step 8 should respond with the following error message:
```
Failed to stop container (fakeContainer): Error response from daemon: No such container: fakeContainer
```
* Step 9 should result in the container stopping successfully
* Step 10 should result in the container starting without error the second time
* Step 11 should result in the container stopping successfully

# Possible Problems:
None