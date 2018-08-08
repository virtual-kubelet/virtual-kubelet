Test 1-27 - Docker Login
=======

# Purpose:
To verify that VIC appliance can log into registries and pull private and public images

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/login/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker pull private image on docker.io
3. Issue docker pull public image on docker.io
4. Issue docker login on docker.io with invalid credentials 
5. Issue docker login on docker.io with valid credentials
6. Issue docker pull private image on docker.io
7. Issue docker logout on docker.io

# Expected Outcome:
* Step 2 should result in an error without login
* Step 4 should result in an error of invalid credentials
* Step 3, 5-7 should each succeed

# Possible Problems:
Test will fail if docker account victest is disabled, or if connection to docker.io cannot be 
established.
