Test 1-26 - Docker Hello World
=======

# Purpose:
To verify that VIC appliance can work with the most basic docker demonstration

# References:
[1 - Docker Hello World](https://hub.docker.com/_/hello-world/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker run hello-world to the new VIC appliance

# Expected Outcome:
* The command should successfully return the hello world message from docker

# Possible Problems:
None