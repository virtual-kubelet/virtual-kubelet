Test 8-01 - Verify VM guest tools integration
=======

# Purpose:
Verify VM guest tools integration for VCH and container VMs

# References:
* govc vm.ip

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Create VCH through vic-machine create
2. Create container
3. Check vSphere through govc to make sure guest IP addresses are set

# Expected Outcome:
* Step 3 should success
