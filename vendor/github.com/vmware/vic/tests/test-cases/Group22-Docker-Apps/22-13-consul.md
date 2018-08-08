Test 22-13 - consul
=======

# Purpose:
To verify that the consul application on docker hub works as expected on VIC

# References:
[1 - Docker Hub consul Official Repository](https://hub.docker.com/_/consul/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Start a basic consul application container:
`docker run -d --name=dev-consul consul`
3. Start two consul agent containers linked to the server:
`docker run -d consul agent -dev -join=172.17.0.2`
4. Query the server for the members and verify that the agents have joined successfully:
`docker exec -t dev-consul consul members`

# Expected Outcome:
* Each step should succeed, consul should be running without error in each case

# Possible Problems:
None
