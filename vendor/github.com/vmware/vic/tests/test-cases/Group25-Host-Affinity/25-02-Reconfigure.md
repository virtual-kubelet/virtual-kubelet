Suite 25-02 - Reconfigure
=========================

# Purpose:
To verify VM-Host Affinity functionality when reconfiguring Virtual Container Hosts

# References:
1. [The design document](../../../doc/design/host-affinity.md)

# Environment:
This suite requires a vCenter Server environment where VCHs can be deployed and container VMs created.


Positive Testing
----------------

### 1. Configuring a VCH does not affect affinity

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH using a DRS VM Group.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Reconfigure the VCH to make a minor change unrelated to VM-Host affinity.
5. Verify that the DRS VM Group still exists and the endpoint VM is still a member of it.
6. Create a variety of containers.
7. Verify that the container VMs were added to the DRS VM Group.

#### Expected Outcome:
* The VCH can be safely reconfigured without unintentionally affecting the use of a DRS VM Group.


### 2. Configuring a VCH without a VM group does not affect affinity

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH without using a DRS VM Group.
3. Verify that no DRS VM Group exists by the expected name.
4. Reconfigure the VCH to make a minor change unrelated to VM-Host affinity.
5. Verify that no DRS VM Group exists by the expected name.
6. Create a variety of containers.
7. Verify that no DRS VM Group exists by the expected name.

#### Expected Outcome:
* Reconfiguring a VCH without a DRS VM Group without specifying the --affinity-vm-group flag does not cause a group to be created.


### 3. Enabling affinity affects existing container VMs

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH without using a DRS VM Group.
3. Verify that no DRS VM Group exists by the expected name.
4. Create a variety of containers.
5. Verify that no DRS VM Group exists by the expected name.
6. Reconfigure the VCH to enable use of a DRS VM Group.
7. Verify that a DRS VM Group was created and that the endpoint VM and container VMs were added to it.

#### Expected Outcome:
* Reconfiguring a VCH to enable use of a DRS VM Group creates the group and adds the endpoint VM and container VMs.


### 4. Enabling affinity affects subsequent container VMs

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH without using a DRS VM Group.
3. Verify that no DRS VM Group exists by the expected name.
4. Reconfigure the VCH to enable use of a DRS VM Group.
5. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
6. Create a variety of containers.
7. Verify that the container VMs were added to the DRS VM Group.

#### Expected Outcome:
* Reconfiguring a VCH to enable use of a DRS VM Group affects subsequent container VMs operations.


### 5. Disabling affinity affects existing container VMs

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH using a DRS VM Group.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Create a variety of containers.
5. Verify that the container VMs were added to the DRS VM Group.
6. Reconfigure the VCH to disable use of a DRS VM Group.
7. Verify that the DRS VM Group no longer exists.

#### Expected Outcome:
* Reconfiguring a VCH to disable use of a DRS VM Group affects existing container VMs.


### 6. Disabling affinity affects subsequent container VMs

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH using a DRS VM Group.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Reconfigure the VCH to disable use of a DRS VM Group.
5. Verify that the DRS VM Group no longer exists.
6. Create a variety of containers.
7. Verify that no DRS VM Group exists by the expected name.

#### Expected Outcome:
* Reconfiguring a VCH to disable use of a DRS VM Group affects subsequent container VM operations.
