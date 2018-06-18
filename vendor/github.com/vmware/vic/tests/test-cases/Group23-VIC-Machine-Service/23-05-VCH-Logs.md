Test 23-05 - VCH Logs
=======

# Purpose:
To verify vic-machine-server can provide logs for a VCH host when available

# References:
[1 - VIC Machine Service API Design Doc - VCH Certificate](../../../doc/design/vic-machine/service.md)

# Environment:
This test requires that a vSphere server is running and available, where VCH can be deployed.

# Test Steps:
1. Deloy a VCH into the test environment
2. Verify that the creation log is available after the VCH is created using the vic-machine-service
3. Verify that the creation log is available for its particular datacenter using the vic-machine-service
4. Delete the log file from VCH datastore folder
5. Verify that creation log is unavailable (404) using the vic-machine service
6. Verify that creation log is unavailable (404) for its particular datacenter using the vic-machine-service

# Expected Outcome:
* Step 2-3 should succeed and output should contain log message that the creation is completed successfully
* Step 5-6 should error with a 404 (not found) as no log file exists

# Possible Problems:
None
