Test 22-04 - mysql
=======

# Purpose:
To verify that the mysql application on docker hub works as expected on VIC

# References:
[1 - Docker Hub mysql Official Repository](https://hub.docker.com/_/mysql/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a mysql container in the background and verify that it is working:  
`docker run --name some-mysql -e MYSQL_ROOT_PASSWORD=my-secret-pw -d mysql`

# Expected Outcome:
* Each step should succeed, mysql should be running without error in each case

# Possible Problems:
None
