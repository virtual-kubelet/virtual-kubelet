Test 1-31 - Docker Node
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker node APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/node/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker node demote
3. Issue docker node ls
4. Issue docker node promote
5. Issue docker node rm
6. Issue docker node update
7. Issue docker node ps
8. Issue docker node inspect

# Expected Outcome:
* Step 2-6 should result in an error that contains Docker Swarm is not yet supported
* Step 7-8 should result in an error that contains No such node

# Possible Problems:
None
