Test 13-2 vMotion Container
=======

# Purpose:
To verify the VCH appliance continues to function properly after some or all of it's related containers are vMotioned

# References:
[1- vMotion A Powered On Virtual Machine](http://pubs.vmware.com/vsphere-4-esx-vcenter/index.jsp?topic=/com.vmware.vsphere.dcadmin.doc_41/vsp_dc_admin_guide/migrating_virtual_machines/t_migrate_a_powered-on_virtual_machine_with_vmotion.html)

# Environment:
This test requires that a vCenter server is running and available

# Test Steps:
1. Install a new VCH appliance onto one of the hosts within the vCenter server
2. Create several containers on the new VCH appliance that are in the following states: created but not started, started and running, started and stopped, stopped after running and being attached to, running after being attached to but currently not attached to, running and currently attached to
3. vMotion each of the containers to a new host within the vCenter server
4. Complete the life cycle of the containers created in Step 2, including getting docker logs and re-attaching to containers that are running

# Expected Outcome:
In each scenario, the VCH appliance should continue to work as expected after being vMotioned and all docker commands should return without error

# Possible Problems:
None
