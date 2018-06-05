Test 5-24 - Non vSphere Local Cluster
=======

# Purpose:
To verify that installing a VCH into a VC cluster that does not have vsphere.local domain configured works as expected

# References:

# Environment:
This test requires access to VMware Nimbus for dynamic vSphere configuration and creation

# Test Steps:
1. Create a simple vCenter cluster in Nimbus with the domain configured to vic.test instead of vsphere.local
2. Install a VCH
3. Run a variety of docker commands

# Expected Outcome:
* Each step should result in success

# Possible Problems:
