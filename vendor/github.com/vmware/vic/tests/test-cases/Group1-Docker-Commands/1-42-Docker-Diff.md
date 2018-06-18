Test 1-42 - Docker Diff
=======

# Purpose:
To verify that docker diff command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/diff/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker create busybox /bin/sh -c "touch a b c; rm -rf /tmp; adduser -D krusty"
3. Issue docker start <containerID> to the VIC appliance
4. Issue docker diff <containerID> to the VIC appliance
5. Verify a, b, c were created, /tmp was deleted and /etc/hosts was modified

# Expected Outcome:
* Step 2 should complete without error, and the response should be the containerID
* Step 3 should run to completion without output
* Step 4 should produce the expected output reflecting the container filesystem changes

# Possible Problems:
None
