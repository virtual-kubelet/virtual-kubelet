Test 5-13 - Invalid ESXi Install
=======

# Purpose:
To verify the VIC appliance provides a reasonable error message when you innapropriately target an ESXi inside a VC for the install

# References:
[1 - VMware vCenter Server Availability Guide](http://www.vmware.com/files/pdf/techpaper/vmware-vcenter-server-availability-guide.pdf)

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a distributed virtual switch
2. Attempt to install the VIC appliance into one of the ESXi directly

# Expected Outcome:
vic-machine create should fail and provide a useful error

# Possible Problems:
None
