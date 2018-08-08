Suite 25-01 - Basic
===================

# Purpose:
To verify basic VM-Host Affinity functionality

# References:
1. [The design document](../../../doc/design/host-affinity.md)

# Environment:
This suite requires a vCenter Server environment where VCHs can be deployed and container VMs created.

Note that because these basic tests do not test the behavior of DRS in the presence of rules, but just the management of
VM groups, these tests do not require an environment where DRS is enabled.


Positive Testing
----------------

### 1. Creating a VCH creates a VM group and container VMs get added to it

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Create a variety of containers.
5. Verify that the container VMs were added to the DRS VM Group.

#### Expected Outcome:
* The DRS VM Group is created and the VCH endpoint VM and all container VMs are added to it.


### 2. Deleting a VCH deletes its VM group

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Delete the VCH.
5. Verify that the DRS VM Group no longer exists.

#### Expected Outcome:
* The DRS VM Group is deleted when the VCH is deleted.


### 3. Deleting a container cleans up its VM group

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH.
3. Create a variety of containers.
4. Verify that a DRS VM Group was created and that the endpoint VM and containers were added to it.
5. Delete the containers.
6. Verify that the DRS VM Group still exists, but does not include the removed containers.

#### Expected Outcome:
* Container VMs are removed from the DRS VM Group when they are deleted.


### 4. Create a VCH without a VM group

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a DRS VM Group with the expected name.
3. Verify that the DRS VM Group is empty.
4. Create a VCH which does not use a DRS VM Group.
5. Verify that the DRS VM Group is empty.
6. Create a variety of containers.
7. Verify that the DRS VM Group is empty.

#### Expected Outcome:
* Neither the VCH Endpoint VM nor the Container VMs are added to the DRS VM Group the VCH is not configured to use.
* VCH creation succeeds even though a DRS VM Group with the same name exists, as use of a group is not configured.


### 5. Attempt to create a VCH when a VM group with the same name already exists

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a DRS VM Group with the expected name.
3. Verify that the DRS VM Group is empty.
4. Attempt to create a VCH which would use a DRS VM Group and expect an error.
5. Verify that the DRS VM Group is empty.

#### Expected Outcome:
* VCH creation fails if a DRS VM Group with the same name already exists, instead of silently using the existing group.


### 6. Deleting a VCH gracefully handles missing VM group

#### Test Steps:
1. Verify that no DRS VM Group exists by the expected name.
2. Create a VCH.
3. Verify that a DRS VM Group was created and that the endpoint VM was added to it.
4. Remove the DRS VM Group with an out-of-band operation.
5. Verify that the DRS VM Group no longer exists.
6. Delete the VCH.

#### Expected Outcome:
* The overall deletion operation succeeds even though the DRS VM Group has already been deleted.
