Test 22-11 - memcached
=======

# Purpose:
To verify that the memcached application on docker hub works as expected on VIC

# References:
[1 - Docker Hub memcached Official Repository](https://hub.docker.com/_/memcached/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a standard memcached container in the background:  
`docker run --name my-memcache -d memcached`
3. Run a memcached container in the background with additional memory:
`docker run --name my-memcache2 -d memcached memcached -m 64`

# Expected Outcome:
* Each step should succeed, memcached should be running without error in each case

# Possible Problems:
None
