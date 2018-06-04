Test 5-26 - Static IP Address
=======

# Purpose:
To verify that VIC works properly when a VCH is installed with a static IP address

# References:
1 `vic-machine-linux create -x`

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Reserve a static IP address
3. Install the VIC appliance into the cluster with the new static IP address
4. Run a variety of docker operations on the VCH

# Expected Outcome:
All test steps should complete without error

# Possible Problems:
None
