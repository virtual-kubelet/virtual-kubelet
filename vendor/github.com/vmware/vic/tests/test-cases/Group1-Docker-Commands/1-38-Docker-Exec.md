Test 1-38 - Docker Exec
=======

# Purpose:
To verify that docker exec command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/exec/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker run -d busybox /bin/top
3. Issue docker exec <containerID> /bin/echo ID - 5 times with incrementing ID
4. Issue docker exec -i <containerID> /bin/echo ID - 5 times with incrementing ID
5. Issue docker exec -t <containerID> /bin/echo ID - 5 times with incrementing ID
6. Issue docker exec -it <containerID> /bin/echo ID - 5 times with incrementing ID
7. Issue docker exec -it <containerID> NON_EXISTING_COMMAND

# Expected Outcome:
* Step 2-6 should echo the ID given
* Step 7 should return an error

# Possible Problems:
None
