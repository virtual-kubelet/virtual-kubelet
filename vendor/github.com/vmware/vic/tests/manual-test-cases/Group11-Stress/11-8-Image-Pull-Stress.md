Test 11-8 - Image Pull Stress
=======

# Purpose:
To verify the VIC appliance works when stressing the appliance with a lot of image pull commands at once

# References:
None

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. Pull 100 images all at once, with at least 10 of the images pulled being the same image
3. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
Each of the image pull commands should return without error and at the end, the variety of docker commands run should work without error

# Possible Problems:
If you exhaust the resources of the vSphere server, it is not necessarily a failure as long as the VCH appliance continues to function and behave as expected
