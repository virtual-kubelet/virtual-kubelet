Test 1-11 - Docker RM
=======

# Purpose:
To verify that docker rm command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/rm/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker create busybox dmesg to the VIC appliance
3. Issue docker rm <containerID> to the VIC appliance
4. Issue docker create busybox ls to the VIC appliance
5. Issue docker start <containerID> to the VIC appliance
6. Issue docker rm <containerID> to the VIC appliance
7. Issue docker create busybox /bin/top to the VIC appliance
8. Issue docker start <containerID> to the VIC appliance
9. Issue docker rm <containerID> to the VIC appliance
10. Issue docker rm -f <containerID> to the VIC appliance
11. Issue docker rm fakeContainer to the VIC appliance
12. Issue docker create --name test busybox to the VIC appliance
13. Remove the containerVM out-of-band using govc
14. Issue docker rm test to the VIC appliance
15. Issue docker rm to container created with an unknown executable
16. Create a container with an anonymous and a named volume
17. Issue docker rm -v to the container from Step 16
18. Issue volume ls to the VIC appliance
19. Create a container with an anonymous volume
20. Create a container with Step 19's anonymous volume as a named volume
21. Issue docker rm -v to the container from Step 19
22. Issue volume ls to the VIC appliance
23. Issue docker rm -v to the container from Step 20
24. Issue volume ls to the VIC appliance
25. Run a container with the volume from Step 19's volume
26. Issue docker rm -f to the container from Step 25
27. Create a new named volume
28. Create a mongo container with the above named volume (mapped to an image volume path) and an anonymous volume
29. Run docker volume ls
30. Run docker rm -v for the container created in Step 28
31. Run docker volume ls

# Expected Outcome:
* Steps 2-8,12,15-31 should complete without error
* Step 3,6,10 should result in the container being removed from the VIC appliance
* Step 9 should result in the following error:  
```
Error response from daemon: Conflict, You cannot remove a running container. Stop the container before attempting removal or use -f
```
* Step 11 should result in the following error:  
```
Error response from daemon: No such container: fakeContainer
```
* Step 13 should succeed on ESXi and fail on vCenter with the following error:
```
govc: ServerFaultCode: The method is disabled by 'VIC'
```
* When run on standalone ESXi, step 14 should result in the following error:  
```
Error response from daemon: No such container: test
```
* Step 17's output should contain the named volume but not the anonymous volume from Step 16
* Step 22's output should contain the volume used in steps 19 and 20
* Step 24's output should contain the volume used in steps 19 and 20
* Step 29's and 31's output should contain the named volume used in step 28

# Possible Problems:
None
