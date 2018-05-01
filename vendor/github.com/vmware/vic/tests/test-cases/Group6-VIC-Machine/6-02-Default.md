Test 6-02 - Verify default parameters
=======

# Purpose:
Verify vic-machine delete default parameters of compute-resource and name

# References:
* vic-machine-linux delete -h

# Environment:
This test requires that a vSphere server is running and available

# Test Cases

## Delete with defaults
1. Delete VCH without compute-resource and name specified

### Expected Outcome:
* Command should fail for resource pool /Resources/virtual-container-host is not found
