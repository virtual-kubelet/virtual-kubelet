Test 1-01 - Docker Info
=======

# Purpose:
To verify that docker info command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/info/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Issue a docker info command to the new VIC appliance
3. Issue a docker -D info command to the new VIC appliance
4. Issue docker info command, docker create busybox, docker info, docker start <containerID>, docker info
5. Issue docker info command, grab the resource pool CPU/mem limits, change values with govc, docker info, check that values are updated, revert to the old values
6. Issue docker info command, grab the resource pool CPU/mem usage, add a running container to the resource pool, docker info, check the resource pool CPU/mem usage values are updated

# Expected Outcome:
* In Step 2, the VIC appliance should respond with a properly formatted info response without errors. Supported volume drivers should be present in the Plugins section.
* Step 3 should result in additional debug information being returned as well.
* Verify in step 4 that the correct number of containers is reported.
* Verify in step 5 that docker info reports the latest resource pool CPU and memory limits.
* Verify in step 6 that docker info reports the latest resource pool CPU and memory usages

# Possible Problems:
None
