Test 1-12 - Docker RMI
=======

# Purpose:
To verify that docker rmi command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/rmi/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker pull busybox to the VIC appliance
3. Issue docker rmi busybox to the VIC appliance
4. Issue docker pull busybox to the VIC appliance
5. Issue docker create busybox /bin/top to the VIC appliance
6. Issue docker rm <containerID> to the VIC appliance
7. Issue docker rmi busybox to the VIC appliance
8. Issue docker pull busybox to the VIC appliance
9. Issue docker create busybox /bin/top to the VIC appliance
10. Issue docker rmi busybox to the VIC appliance
11. Issue docker rmi -f busybox to the VIC appliance
12. Issue docker rmi fakeImage to the VIC appliance
13. Issue a docker pull by digest
14. Issue a docker rmi by digest
15. Issue docker pull for busybox and alpine
16. Reboot the VCH and wait for it to initialize
17. Obtain the image short ID for busybox
18. Issue docker rmi using the short ID for busybox
19. Obtain the image long ID for alpine
20. Issue docker rmi using the long ID for alpine

# Expected Outcome:
* Step 3 and 7 should result in success and the image should be removed from inventory
* Step 10 should result in failure and the error message below:  
```
Failed to remove image (busybox): Error response from daemon: conflict: unable to remove repository reference "busybox" (must force) - container 6a26c7deb9b7 is using its referenced image 47bcc53f74dc
```
* Step 11 should result in success and the image should be removed from inventory
* Step 12 should result in an error with the following message:  
```
Failed to remove image (fakeImage): Error response from daemon: No such image: fakeImage:latest
```
* Steps 13-14 should result in success and the output of step 14 should contain the digest of the rmi'd image
* Steps 15-20 should succeed

# Possible Problems:
None
