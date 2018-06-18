Test 22-03 - registry
=======

# Purpose:
To verify that the registry application on docker hub works as expected on VIC

# References:
[1 - Docker Hub registry Official Repository](https://hub.docker.com/_/registry/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a registry container in the background and verify that it is working:  
`docker run -d -p 5000:5000 --restart always --name registry registry:2`

# Expected Outcome:
* Each step should succeed, registry should be running without error in each case

# Possible Problems:
None
