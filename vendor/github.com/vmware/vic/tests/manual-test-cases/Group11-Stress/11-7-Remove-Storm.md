Test 11-7 - Remove Storm
=======

# Purpose:
To verify the VIC appliance works when stressing the appliance with a lot of rm commands at once

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. Create 100 containers
3. In parallel, attempt to rm each of the containers as quickly as possible
4. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
Each of the volume rm commands should return without error and at the end, the variety of docker commands run should work without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
