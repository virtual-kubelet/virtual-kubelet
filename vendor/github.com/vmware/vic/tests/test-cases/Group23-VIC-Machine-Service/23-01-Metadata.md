Test 23-01 - Version
=======

# Purpose:
To verify vic-machine-server returns a valid version number

# References:
1. [The design document](../../../doc/design/vic-machine/service.md)

# Environment:
This test has no environmental requirements

# Test Steps:
1. Start the vic-machine-server
2. Retry 5 times using curl to issue a GET request for the version endpoint
3. Use curl to issue a GET request for the gretting hello message endpoint

# Expected Outcome:
* Step 2 should succeed with a 200 OK response containing a version number
* Step 3 should succeed with a 200 OK response containing a greeting message

# Possible Problems:
* Step 1 could take more than the expected retry time for the service to become available, causing the GET request sent in step 2 to fail with a return code of 7. (Other tests in this suite may wait on a response from the version endpoint to determine whether the service is available, but this test should not as it is the endpoint under test.)

