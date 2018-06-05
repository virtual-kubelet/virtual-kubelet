Test 21-02 - Artifactory Support
=======

# Purpose:
To verify that VIC engine works properly with Artifactory

# References:

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Execute docker login to the artifactory server
3. Execute a docker pull to an image on the artifactory server

# Expected Outcome:
* Steps 1-3 should all result in success

# Possible Problems:
None
