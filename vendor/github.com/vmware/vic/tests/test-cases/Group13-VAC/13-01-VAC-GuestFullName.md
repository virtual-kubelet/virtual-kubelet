Test 13-01 - GuestFullName
=======

#Purpose:
To verify that VIC-Machine and VIC creates VMs with a custom guest name for OS.

#Environment:
This test requires that a vSphere server is running and available.

#Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Issue a GOVC command to get the guest name for the VIC appliance VM
3. Issue a docker create busybox
4. Issue a GOVC command to get the guest name for the container VM

#Expected Outcome:
* Step 2 should result in an output where 'Guest name' contains 'Photon - VCH'
* Step 4 should result in an output where 'Guest name' contains 'Photon - Container'

#Possible Problems:
None
