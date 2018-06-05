Test 12-2 Docker Commands Maintenance Mode
=======

#Purpose:
To verify the VIC appliance provides reasonable behavior when the host it is running on has been placed in maintenance mode

#References:
[1- VMware Maintenance Mode](https://pubs.vmware.com/vsphere-4-esx-vcenter/index.jsp?topic=/com.vmware.vsphere.resourcemanagement.doc_41/using_drs_clusters_to_manage_resources/c_using_maintenance_mode.html)

#Environment:
This test requires that a vSphere server is running and available

#Test Steps:
1. Install a new VIC appliance on the vSphere server
2. After the install, place the host into maintenance mode
3. Issue a variety of docker commands to the VIC appliance while the host is in maintenance mode

#Expected Outcome:
The VCH appliance should deploy without error and the docker commands should return an error that is reasonable and indicates that it cannot complete due to the host being in maintenance mode

#Possible Problems:
None