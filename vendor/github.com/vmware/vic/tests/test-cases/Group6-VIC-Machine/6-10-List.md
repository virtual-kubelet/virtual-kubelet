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
2. Run ls to query all VCHs
3. Run ls to query VCH via Compute Resource
4. Run ls with trailing slash on target
5. Run ls with invalid compute resource, should suggest correct
6. Run ls with invalid datacenter, should suggest correct
7. Run ls with valid datacenter
8. Run ls with compute resource pointing at empty cluster

# Expected Results
* Steps 1-4 should succeed and correctly list any VCHs present in the system
* Step 5-6 should fail and suggest a valid alternatives
* Step 7 should succeed
* Step 8 should succeed by listing no VCHs
