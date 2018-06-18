Test 22-09 - httpd
=======

# Purpose:
To verify that the httpd application on docker hub works as expected on VIC

# References:
[1 - Docker Hub httpd Official Repository](https://hub.docker.com/_/httpd/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run an httpd container in the background and verify the server is up and running:  
`docker run -dit --name httpd1 -v "$PWD":/usr/local/apache2/htdocs/ httpd:2.4`

# Expected Outcome:
* Each step should succeed, httpd should be running without error in each case

# Possible Problems:
None
