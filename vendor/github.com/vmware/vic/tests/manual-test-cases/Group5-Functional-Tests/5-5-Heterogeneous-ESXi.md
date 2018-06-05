Test 5-5 - Heterogeneous ESXi
=======

# Purpose:
To verify the VIC appliance works when the vCenter appliance is using multiple different ESXi versions

# References:
[1 - VMware vCenter Server Availability Guide](http://www.vmware.com/files/pdf/techpaper/vmware-vcenter-server-availability-guide.pdf)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a cluster in Nimbus
2. Deploy three different ESXi hosts with build numbers(6.0.0u2, 5.5u3, 6.5RC1):
```3620759``` and ```3029944``` and ```4240417```
3. Add each host to the cluster
4. Deploy a VCH appliance to the cluster allowing DRS to manage placement
5. Run a variety of docker commands on each of the VCH appliances.

# Expected Outcome:
The VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
None
