Test 6-06 - Verify vic-machine create datastore function
=======

# Purpose:
Verify vic-machine create image store, volume store and container store functions

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available

# Test Cases: - image store

# Test Steps
1. create with wrong image store format, e.g. wrong separator, wrong schema
2. Create with not existed image store name
3. Verify deployment failed with user-friendly error message

# Test Steps
1. Create with not existed image store path with format: <image store name>:/some/path
2. Create with existed image store path with formt: <image store name>:/ds://some/path
3. Create without image store configured
3. Regression test
4. Verify deployment and regression test passed
5. Verify docker images are persisted in the specified path or default path if image store is not provided

# Test Cases: - image store not found
# Test steps
1. Delete VCH created image store through govc
2. Delete VCH
3. Verify VCH delete succeeds

# Test Cases: - image store delete

# Test steps
1. Delete above VCH deployed with image store path
2. Verify image store is deleted successfully

# Test Cases: - volume store

# Test Steps
1. create with wrong volume store format, e.g. wrong separator, wrong schema
2. Create with not existed volume store name
3. Verify deployment failed with user-friendly error message

# Test Steps
1. Create with not existed voume store path with format: <volume store name>:/some/path
2. Create with existed image store path with formt: <volume store name>:/ds://some/path
3. Create with multiple volume store path
4. Create without volume store parameters
4. Regression test
5. Test docker volume commands
6. Verify deployment and regression test passed
7. Verify volumes are persisted in the specified path
8. If volume store is not provided, verify docker volume commands does not work

# Test Cases: - volume store delete

# Test steps
1. Delete above VCH deployed with volume store path without --force
2. Verify delete is successful but volume store is not deleted through govc
3. Verify the configured volume stores are correctly listed in warning message during deletion

# Test steps
1. Delete VCH deployed with volume store path with --force
2. Verify volume store is deleted successfully through govc

# Test Cases: - container store
# FIXME: container store is not implemeted
