Test 4-01 - Docker Integration
=======

# Purpose:
To verify that the VIC appliance passes the docker integration tests or at least any failure are characterized and well documented.

# References:
[1 - Docker Integration Tests](https://docs.docker.com/opensource/project/test-and-docs/)

# Environment:
This test requires that a VIC appliance is available to execute the docker integration tests against

# Test Steps:
1. Deploy VIC appliance to a test server
2. Execute the docker integration tests:  
```
DOCKER_HOST=tcp://<VIP IP> go test
```

# Expected Outcome:
VIC appliance needs to pass enough of the tests and for any failures, they need to be characterized, agreed that they are acceptable, and well documented.

# Possible Problems:
None
