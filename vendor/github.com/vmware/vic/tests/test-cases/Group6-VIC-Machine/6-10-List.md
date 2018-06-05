Test 6-10 - Verify vic-machine ls
=======

# Purpose:
Verify vic-machine ls functions

# References:
* vic-machine-linux ls -h

# Environment:
This test requires that a vSphere server is running and available

# Test Steps
1. Create VCH
3. Run ls to query out VCH
4. Run inspect to verify VCH id is correct
5. Run inspect to verify VCH compute path and name are correct
6. Run inspect to verify VCH with a trailing slash in the target
7. Run inspect with an invalid compute resource

# Expected Results
* Steps 1-6 should succeed and correctly list any VCHs present in the system
* Step 7 should fail and suggest a valid compute resource
