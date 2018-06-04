Test 1-29 - Docker Checkpoint
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker checkpoint APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/checkpoint/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker checkpoint create
3. Issue docker checkpoint ls
4. Issue docker checkpoint rm

# Expected Outcome:
* Step 2-4 should result in an error of not supported

# Possible Problems:
None