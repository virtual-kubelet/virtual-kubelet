Test 22-05 - mongo
=======

# Purpose:
To verify that the mongo application on docker hub works as expected on VIC

# References:
[1 - Docker Hub mongo Official Repository](https://hub.docker.com/_/mongo/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a mongo container in the background and verify that it is working:  
`docker run --name mongo1 -d mongo`

# Expected Outcome:
* Each step should succeed, mongo should be running without error in each case

# Possible Problems:
None
