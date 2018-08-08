Test 11-3 - Many Containers
=======

# Purpose:
To verify the VIC appliance works when stressing the appliance with a lot of containers

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. In a loop, create 1000 containers using docker run busybox date
3. After the last iteration, run a variety of docker commands on the VCH appliance

# Expected Outcome:
Each of the containers should start without error and at the end, the variety of docker commands run should work without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
