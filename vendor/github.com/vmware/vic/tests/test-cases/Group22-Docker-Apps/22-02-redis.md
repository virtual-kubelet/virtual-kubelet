Test 22-02 - redis
=======

# Purpose:
To verify that the redis application on docker hub works as expected on VIC

# References:
[1 - Docker Hub redis Official Repository](https://hub.docker.com/_/redis/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a redis container in the background and verify that it is working:  
`docker run --name some-redis -d redis`
3. Run a redis client container that connects to the redis server
4. Run a redis container in the background with appendonly option and verify that it is working:
`docker run --name some-redis -d redis redis-server --appendonly yes`

# Expected Outcome:
* Each step should succeed, redis should be running without error in each case

# Possible Problems:
None
