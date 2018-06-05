Test 22-14 - wordpress
=======

# Purpose:
To verify that the wordpress application on docker hub works as expected on VIC

# References:
[1 - Docker Hub wordpress Official Repository](https://hub.docker.com/_/wordpress/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Start a mysql db container:
`docker run --name mysql1 -e MYSQL_ROOT_PASSWORD=password1 -d mysql`
3. Start a wordpress container linked to the mysql container:
`docker run --name wordpress1 --link mysql1:mysql -d wordpress`
4. Start a wordpress container linked to the mysql container and with published ports:
`docker run --name wordpress2 --link mysql1:mysql -p 8080:80 -d wordpress`

# Expected Outcome:
* Each step should succeed, wordpress should be running without error in each case

# Possible Problems:
None
