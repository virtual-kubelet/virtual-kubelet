Test 1-15 - Docker Network Create
=======

# Purpose:
To verify that docker network create command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/network_create/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker network create test-network to the VIC appliance
3. Issue docker network create --label=foo=bar label-network to the VIC appliance
4. Issue docker network inspect -f '{{.Labels}}' label-network to the VIC appliance
5. Issue docker network create test-network to the VIC appliance
6. Issue docker network create -d overlay test-network2 to the VIC appliance
7. Issue docker network create --internal internal-network
8. Issue docker network inspect -f '{{.Internal}}' internal-network

# Expected Outcome:
* Step 2 should complete successfully and a new network should be created named test-network
* Step 3 should complete successfully and a new network should be created named label-network
* Step 4 should complete successfully and the output should show label-network's label
* Step 5 should result in an error with the following message:
```
Error response from daemon: network with name test-network already exists
```
* Step 6 should result in an error with the following message:
```
Error response from daemon: failed to parse pool request for address space "GlobalDefault" pool "" subpool "": cannot find address space GlobalDefault (most likely the backing datastore is not configured)
```
* Step 7 should return without an error
* Step 8's output should be equal to 'true'

# Possible Problems:
None