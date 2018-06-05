Test 1-35 - Docker Swarm
=======

# Purpose:
To verify that VIC appliance responds appropriately to docker swarm APIs

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/node/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker swarm init
3. Issue docker swarm join
4. Issue docker swarm join-token
5. Issue docker swarm leave
6. Issue docker swarm unlock
7. Issue docker swarm unlock-key
8. Issue docker swarm update

# Expected Outcome:
* Step 2-8 should result in an error that contains Docker Swarm is not yet supported

# Possible Problems:
None