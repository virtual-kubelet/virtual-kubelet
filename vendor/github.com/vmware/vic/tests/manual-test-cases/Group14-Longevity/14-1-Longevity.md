Test 14-1 - Longevity
=======

# Purpose:
To verify that VIC appliance can run for prolonged periods of time without leaking or causing system instability

# References:
None

# Environment:
This test requires that a vSphere server is running, isolated, and available for long periods of time

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Execute a random loop of 10-50 iterations of the regression suite
3. Issue vic-machine delete to cleanup the VCH
4. Repeat Steps 1-3 48 times alternating between using TLS certificates and not on each install.

# Expected Outcome:
* VIC appliance should install correctly and cleanup after itself each time
* Each iteration of the regression suite should pass without error

# Possible Problems:
None
