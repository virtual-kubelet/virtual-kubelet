Test 11-2 - Container Boot Storm
=======

# Purpose:
To verify the VIC appliance works when stressing the container start component of the system

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. Create 100 container on the new VCH appliance
3. In parallel, attempt to start all 100 containers at once
4. After the boot storm, run a variety of docker commands on the VCH appliance

# Expected Outcome:
Each of the containers should start without error and at the end, the variety of docker commands run should work without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
