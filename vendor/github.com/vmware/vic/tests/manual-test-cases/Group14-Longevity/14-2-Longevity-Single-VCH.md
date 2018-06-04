Test 14-2 - Longevity - Single - VCH
=======

# Purpose:
To verify that a single VIC appliance can run for prolonged periods of time without leaking or causing system instability

# References:
None

# Environment:
This test requires that a vSphere server is running, isolated, and available for long periods of time

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Execute a loop of the regression suite for up to 2 days
3. Issue vic-machine delete to cleanup the VCH
4. Install another VCH
5. Execute a single iteration of the regression suite
6. Issue vic-machine delete to cleanup the VCH

# Expected Outcome:
* VIC appliance should install correctly and cleanup after itself each time
* Each iteration of the regression suite should pass without error
* The VCH installed after the long loop should install correctly, function as expected and cleanup after itself

# Possible Problems:
None
