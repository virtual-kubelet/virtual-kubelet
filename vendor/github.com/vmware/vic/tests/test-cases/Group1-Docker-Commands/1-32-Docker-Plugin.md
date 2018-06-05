Test 1-32 - Docker Plugin
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker plugin APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/plugin_create/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker plugin install
3. Issue docker plugin create
4. Issue docker plugin enable
5. Issue docker plugin disable
6. Issue docker plugin inspect
7. Issue docker plugin ls
8. Issue docker plugin push
9. Issue docker plugin rm
10. Issue docker plugin set

# Expected Outcome:
* Step 2-10 should result in an error that contains does not yet support plugins

# Possible Problems:
None