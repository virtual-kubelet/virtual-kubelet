Test 1-28 - Docker Secret
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker secrets APIs.

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/secret/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker secret ls
3. Issue docker secret create
4. Issue docker secret inspect
5. Issue docker secret rm

# Expected Outcome:
* Step 2-5 should result in an error of not supported

# Possible Problems:
None