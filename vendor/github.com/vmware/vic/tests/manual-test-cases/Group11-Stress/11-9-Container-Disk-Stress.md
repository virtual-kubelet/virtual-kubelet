Test 11-9 - Container Disk Stress
=======

# Purpose:
To verify the VIC appliance works when stressing the appliance with a lot of disk operations

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. Create a container with a 10GB volume attached
3. Within the container, execute bonnie++ disk stress test
4. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The bonnie++ command should return without error and at the end, the variety of docker commands run should work without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
