Test 11-1 - VIC Install Stress
=======

# Purpose:
To verify the VIC appliance works when stressing the install component of the system

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. In a loop, install and delete a VCH appliance as rapidly as possible 100 times
2. After the last install, run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error each time and each of the docker commands executed against the last install should return without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
