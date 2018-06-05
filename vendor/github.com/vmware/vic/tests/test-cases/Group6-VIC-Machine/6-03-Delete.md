Test 6-03 - Verify delete clean up all resources
=======

# Purpose:
Verify vic-machine delete can delete vch installed by vic-machine create

# References:
* vic-machine-linux delete -h
# Environment:
This test requires that a vSphere server is running and available

# Test Cases

## Delete VCH and verify
1. Create VCH through vic-machine create
2. Create container
3. Make sure container VM is powered on
4. Delete VCH without --force, to get delete failure
5. Delete VCH with --force, to get delete success
6. Check vSphere through govc to make sure all resources are deleted correctly

### Expected Outcome:
* Step 4 should fail for VM is powered on
* Step 5 should success
* Step 6 should not find any resources created by vic-machine and VCH


## Attach Disks and Delete VM
1. Create VCH normally
2. Pull a moderately sized image (ubuntu)
3. Check to see that disk images exist via govc
4. Use govc to attach disk images to appliance VM directly
5. Delete the VCH
6. Ensure that VCH folder is entirely gone after delete via govc

### Expected Outcome:
1. All steps should succeed
2. govc should return 1 when not finding images in step 6


## Delete VCH with non-cVM in same RP
1. Create VCH though vic-machine create
2. Create VM in the VCH's resource pool
3. Delete VCH
4. Check vSphere through govc to make sure the VM was not deleted
5. Delete the VM and RP to cleanup

### Expected Outcome:
1. All steps should succeed


## Delete VCH moved from its RP
1. Create VCH though vic-machine create
2. Create VM in the VCH's resource pool
3. Create another RP
4. Move VCH to the new RP
5. Delete VCH
6. Check vSphere through govc to make sure the VM and containerVM were not deleted
7. Delete the VM, containerVM, and RP to cleanup

### Expected Outcome:
1. All steps should succeed


## Delete VCH moved to root RP and original RP deleted
1. Create VCH through vic-machine create
2. Create VM in the root RP
3. Move VCH to root RP
4. Delete the VCH created RP
5. Delete VCH
6. Check vSphere through govc to make sure the VM in the root RP was not deleted
7. Delete the VM to cleanup

### Expected Outcome:
1. All steps should succeed


## Delete VCH with non-cVM in VCH folder

### Summary:
This verifies that when there is a non-containerVM inside the VCH folder, vic-machine delete cleans up the VCH and containerVMs
but leaves the non-containerVM and its parent folder (VCH folder) intact.

### Test Steps:
1. Create VCH through vic-machine create
2. Create a busybox container
3. Create a dummy VM inside the VCH's folder
4. Delete the VCH with vic-machine delete --force
5. Use govc to look for the VCH folder
6. Use govc to verify that the container has been deleted
7. Use govc to look for the non-containerVM
8. Use govc to cleanup the non-containerVM, the VCH folder and the resource pool

### Expected Outcome:
* All steps should succeed
