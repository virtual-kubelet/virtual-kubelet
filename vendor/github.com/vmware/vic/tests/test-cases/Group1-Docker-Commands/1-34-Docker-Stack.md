Test 1-34 - Docker Node
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker stack APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/stack/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker stack deploy
3. Issue docker stack ls
4. Issue docker stack ps
5. Issue docker stack rm
6. Issue docker stack services

# Expected Outcome:
* Step 2-6 should result in an error that contains Docker Swarm is not yet supported

# Possible Problems:
None