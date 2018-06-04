Test 6-18 - Verify vic-machine create --container-name-convention
=======

# Purpose:
Verify vic-machine create --container-name-convention functions

# References:
* vic-machine-linux create -x

# Environment:
This test requires that a vSphere server is running and available

# Test Steps
1. Create a new VCH using the --container-name-convention as 192.168.1.1-{id}
2. Create a container and verify that the container works and vSphere name is according to the convention
3. Run a variety of docker operations
4. Create a new VCH using the --container-name-convention as 192.168.1.1-{name}
5. Create a container and verify that the container works and vSphere name is according to the convention
6. Run a variety of docker operations
7. Create a new VCH using the --container-name-convention as 192.168.1.1-mycontainer

# Expected Results
* All steps should succeed as expected, except step should fail with an error indicating that {name} or {id} must be included