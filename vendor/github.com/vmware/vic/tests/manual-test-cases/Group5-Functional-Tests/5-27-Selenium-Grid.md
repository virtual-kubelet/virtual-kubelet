Test 5-27 - Selenium Grid
=======

# Purpose:
To verify that VIC works properly when a large grid of Selenium workers is deployed

# References:
[1 - Selenium Grid](https://github.com/SeleniumHQ/docker-selenium/blob/master/README.md)


# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Create a docker network for the selenium grid
3. Deploy a selenium grid hub
4. Deploy 30 selenium workers of various types
5. Verify each of the workers are deployed properly and connect to the hub

# Expected Outcome:
All test steps should complete without error

# Possible Problems:
None
