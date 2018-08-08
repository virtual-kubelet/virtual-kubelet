Test 22-10 - logstash
=======

# Purpose:
To verify that the logstash application on docker hub works as expected on VIC

# References:
[1 - Docker Hub logstash Official Repository](https://hub.docker.com/_/logstash/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Run a logstash container in the background with input and output mapped to stdin/stdout:  
`docker run -dit logstash -e 'input { stdin { } } output { stdout { } }'`
3. Run a logstash container in the background with input mapped to a log file on a volume:
`docker run -dit -v vol1:/logs logstash -e 'input { file { path => "/logs/my.log" start_position => "beginning" } } output { stdout { } }'`

# Expected Outcome:
* Each step should succeed, logstash should be running without error in each case

# Possible Problems:
None
