Test 1-22 - Docker Volume RM
=======

# Purpose:
To verify that docker volume rm command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/volume_rm/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker volume create --name=test to the VIC appliance
3. Issue docker volume create --name=test2 to the VIC appliance
4. Issue docker volume rm test to the VIC appliance
5. Issue docker create -v test2:/test busybox to the VIC appliance
6. Issue docker volume rm test2 to the VIC appliance
7. Issue docker volume rm test3 to the VIC appliance
8. Issue docker rm <containerID from Step 5> to the VIC appliance
9. Issue docker volume rm test2 to the VIC appliance

# Expected Outcome:
* Step 4 should result in success and the volume should not be listed anymore
* Step 6 should result in error with the following message:  
```
Error response from daemon: Conflict: remove test2: volume is in use - [<containerID>]
```
* Step 7 should result in error with the following message:  
```
Error response from daemon: get test3: no such volume
```
* Step 9 should result in success and the volume should no longer be listed

# Possible Problems:
* VIC requires you to specify storage on creation of the VCH that volumes can be created from, so when installing the VCH make sure to specify this parameter: --volume-store=