Test 22-12 - centos
=======

# Purpose:
To verify that the centos application on docker hub works as expected on VIC

# References:
[1 - Docker Hub centos Official Repository](https://hub.docker.com/_/centos/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a latest centos container:  
`docker run centos:latest yum update`
3. Run a centos:6 container:  
`docker run centos:6 yum update`

# Expected Outcome:
* Each step should succeed, centos should be running without error in each case

# Possible Problems:
None
