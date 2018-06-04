Test 22-07 - elasticsearch
=======

# Purpose:
To verify that the elasticsearch application on docker hub works as expected on VIC

# References:
[1 - Docker Hub elasticsearch Official Repository](https://hub.docker.com/_/elasticsearch/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run an elasticsearch container in the background and verify that it is working:  
`docker run -d elasticsearch`
3. Run an elasticsearch container in the background with additional flags passed to elasticsearch:  
`docker run -d elasticsearch -Des.node.name="TestNode"`
4. Run an elasticsearch container in the background with a custom config folder passed in:   
`docker run -d -v "$PWD/config":/usr/share/elasticsearch/config elasticsearch`
5. Run an elasticsearch container in the background with a persisted data volume:  
`docker run -d -v "$PWD/esdata":/usr/share/elasticsearch/data elasticsearch`

# Expected Outcome:
* Each step should succeed, elasticsearch should be running without error in each case

# Possible Problems:
None
