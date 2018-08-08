Test 1-18 - Docker Network RM
=======

# Purpose:
To verify that docker network rm command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/network_rm/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker network create test-network to the VIC appliance
3. Issue docker network create test-network2 to the VIC appliance
4. Issue docker network create test-network3 to the VIC appliance
5. Issue docker rm test-network to the VIC appliance
6. Issue docker rm test-network2 <ID of test-network3> to the VIC appliance
7. Issue docker rm test-network to the VIC appliance
8. Issue docker network create test-network
9. Issue docker create busybox /bin/top
10. Issue docker network connect test-network <containerID>
11. Issue docker start <containerID>
12. Issue docker network rm test-network
13. Issue docker stop <containerID>
14. Issue docker rm <containerID>
15. Issue docker network rm test-network

# Expected Outcome:
* Steps 5 and 6 should completely successfully and all three network should be removed
* Step 7 should result in an error and show the following error message:  
```
Error response from daemon: network test-network not found
```
* Step 12 should result in an error with the following message:
```
Error response from daemon: network test-network has active endpoints
```
* Step 15 should result in success and the network should be removed

# Possible Problems:
None