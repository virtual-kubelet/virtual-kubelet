Test 22-15 - swarm
=======

# Purpose:
To verify that the swarm application on docker hub works as expected on VIC

# References:
[1 - Docker Hub swarm Official Repository](https://hub.docker.com/_/wordpress/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Create an initial swarm cluster:
`docker run --rm swarm create`
3. Create 2 swarm nodes:
`docker run -d swarm join --addr=<node_ip:2375> token://<cluster_id>`
4. Start the swarm manager:
`docker run -t -p <swarm_port>:2375 -t swarm manage token://<cluster_id>`

# Expected Outcome:
* Each step should succeed, swarm should be running without error in each case

# Possible Problems:
None
