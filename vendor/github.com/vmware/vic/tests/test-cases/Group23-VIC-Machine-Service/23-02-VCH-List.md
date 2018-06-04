Test 23-02 - VCH List
=======

# Purpose:
To verify vic-machine-server can return a list of VCHs including the expected information

# References:
1. [The design document](../../../doc/design/vic-machine/service.md)

# Environment:
This test requires a vSphere system where VCHs can be deployed 

# Test Steps:
1. Deploy a VCH
2. Get a list of all VCHs
3. Get a list of all VCHs in the test datacenter
4. Attempt to list all VCHs in an invalid datacenter
5. Attempt to list all VCHs in an invalid compute resource
6. Attempt to list all VCHs in an invalid datacenter and compute resource

# Expected Outcome:
* The results of 2-3 should contain the VCH created in 1, with the correct ID.
* The requests in 4-6 should result in an appropriate error message.

# Possible Problems:
None known
