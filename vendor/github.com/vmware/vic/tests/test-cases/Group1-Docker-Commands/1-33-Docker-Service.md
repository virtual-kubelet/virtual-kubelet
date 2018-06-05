Test 1-33 - Docker Service
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker service APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/service/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker service create
3. Issue docker service inspect
4. Issue docker service ls
5. Issue docker service ps
6. Issue docker service rm
7. Issue docker service scale
8. Issue docker service update
9. Issue docker service logs

# Expected Outcome:
* Step 2-8 should result in an error that contains Docker Swarm is not yet supported
* Step 9 should result in an error that contains only supported with experimental daemon

# Possible Problems:
None