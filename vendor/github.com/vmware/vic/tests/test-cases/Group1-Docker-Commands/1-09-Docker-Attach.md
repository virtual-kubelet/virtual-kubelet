Test 1-09 - Docker Attach
=======

# Purpose:
To verify that docker attach command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/attach/)

# Environment:
This test requires that a vSphere server is running and available

# Test Cases

## Basic attach
1. Deploy VIC appliance to vSphere server
2. Issue docker create -it busybox /bin/top to the VIC appliance
3. Issue docker start <containerID> to the VIC appliance
4. Issue docker attach <containerID> to the VIC appliance
5. Issue ctrl-p then ctrl-q to the container
6. Issue docker create -it busybox /bin/top to the VIC appliance
7. Issue docker attach <containerID>
8. Issue docker start <containerID> to the VIC appliance
9. Issue docker attach --detach-keys="a" <containerID> to the VIC appliance
10. Issue 'a' to the container
11. Attempt to reattach to the the same container a second time
12. Issue docker attach fakeContainer to the VIC appliance

### Expected Outcome:
* Steps 1-6,8-11 should all return without error
* Step 7 should result in the following error message:
```
You cannot attach to a stopped container, start it first
```
* Step 5 and 10 should cause the client to detach from the container gracefully, with the container continuing to run
* Step 12 should result in the following message:
 ```
 Error: No such container: fakeContainer
 ```
