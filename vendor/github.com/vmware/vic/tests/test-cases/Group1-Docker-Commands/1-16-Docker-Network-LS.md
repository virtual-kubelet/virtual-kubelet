Test 1-16 - Docker Network LS
=======

# Purpose:
To verify that docker network ls command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/network_ls/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker network ls to the VIC appliance
3. Issue docker network ls -q to the VIC appliance
4. Issue docker network ls -f name=bridge to the VIC appliance
5. Issue docker network ls -f name=fakeName to the VIC appliance
6. Issue docker network create --label=foo foo-network to the VIC appliance
7. Issue docker network ls -f label=foo to the VIC appliance
8. Issue docker network ls --no-trunc to the VIC appliance

# Expected Outcome:
* Step 2 should return at the least the default networks
* Step 3 should return the networks ID only
* Step 4 should return only the bridge network
* Step 5 should return no networks listed
* Step 6 should return without errors
* Step 7 should return only the foo-network network
* Step 8 should return all of the networks with their full IDs

# Possible Problems:
None