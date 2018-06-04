Test 5-25 - OPS User Grant
=======

# Purpose:
To verify that VIC works properly when a VCH is installed with the option to create the proper permissions for the OPS-user

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Create Local OPS User on VC
3. Give the Local OPS User ReadOnly Role on /
3. Install the VIC appliance into the cluster with the --ops-grant-perms option
4. Run a variety of docker operation on the VCH

# Expected Outcome:
All test steps should complete without error

# Possible Problems:
None
