Test 5-6-2 - VSAN-Complex
=======

# Purpose:
To verify the VIC appliance works with VMware Virtual SAN

# References:
[1 - VMware Virtual SAN](http://www.vmware.com/products/virtual-san.html)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a complex test bed in Nimbus:  
```--testbedName test-vpx-4esx-virtual-fullInstall-vcva-8gbmem```  
2. Deploy VCH Appliance to the new vCenter
3. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
* None
