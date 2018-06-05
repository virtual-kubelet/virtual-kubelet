Test 22-01 - nginx
=======

# Purpose:
To verify that the nginx application on docker hub works as expected on VIC

# References:
[1 - Docker Hub nginx Official Repository](https://hub.docker.com/_/nginx/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run an nginx container in the background and verify the server is up and running:  
`docker run --name nginx1 -d nginx`
3. Run an nginx container in the background with a mapped port:  
`docker run --name nginx2 -d -p 8080:80 nginx`
4. Run an nginx container in the background with a mapped content folder from a volume:  
`docker run --name nginx3 -v /some/content:/usr/share/nginx/html:ro -d nginx`

# Expected Outcome:
* Each step should succeed, nginx should be running without error in each case

# Possible Problems:
None
