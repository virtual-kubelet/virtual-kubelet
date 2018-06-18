Test 22-06 - postgres
=======

# Purpose:
To verify that the postgres application on docker hub works as expected on VIC

# References:
[1 - Docker Hub postgres Official Repository](https://hub.docker.com/_/postgres/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a mysql container in the background and verify that it is working:  
`docker run --name postgres1 -e POSTGRES_PASSWORD=password1 -d postgres`

# Expected Outcome:
* Each step should succeed, postgres should be running without error in each case

# Possible Problems:
None
