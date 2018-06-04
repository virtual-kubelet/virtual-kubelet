Test 8-02 - OOB VM Register
=======

# Purpose:
Verify that when a VM is registered OOB, the VIC continues to work

# References:

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install new VCH appliance
2. Create a VM out of band
3. Unregister the created VM
4. Register the created VM
5. Issue docker ps -a

# Expected Outcome:
* The VCH should continue to function properly and docker ps -a should return proper output without error
