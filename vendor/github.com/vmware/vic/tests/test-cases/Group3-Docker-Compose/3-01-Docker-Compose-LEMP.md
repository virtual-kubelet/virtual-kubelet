Test 3-01 - Docker Compose LEMP Server
=======

# Purpose:
To verify that VIC appliance can work when deploying a LEMP (Linux, Nginx, MySQL, PHP) server

# References:
[1 - Docker Compose Overview](https://docs.docker.com/compose/overview/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Create a compose file that includes an Nginx, MySQL and PHP server with network connections between them
2. Deploy VIC appliance to the vSphere server
3. Issue:  
```DOCKER_HOST=<VCH IP> docker-compose up```
4. wget the index file of the site and verify that it contains the default Nginx message

# Expected Outcome:
* Docker compose should return with success and the Nginx server should be running.
* The wget command should successfully return the default Nginx index file.

# Possible Problems:
None
