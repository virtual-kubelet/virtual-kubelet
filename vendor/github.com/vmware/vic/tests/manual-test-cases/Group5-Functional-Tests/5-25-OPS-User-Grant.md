Test 5-25 - OPS User Grant
=======

# Purpose:
To verify that VIC works properly when a VCH is installed with the option to create the proper permissions for the OPS-user

# Environment:
This test requires access to VMware Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy a new vCenter with a simple cluster
2. Create Local OPS User on VC
3. Install the VIC appliance into the cluster with the --ops-grant-perms option
4. With the ops-user, use govc to attempt to change the DRS settings on the cluster
5. Run a variety of docker operations on the VCH
6. Run privilege-dependent docker operations against the VCH
7. Create a container
8. Use govc to attempt to out-of-band destroy the container from Step 7
9. Clean up the VCH
10. Install version v1.3.1 of the VIC appliance into the cluster with the --ops-grant-perms option
11. Perform a VCH upgrade to the current version
12. With the ops-user, use govc to attempt to create a resource pool
13. Run a variety of docker operations on the VCH
14. Run privilege-dependent docker operations against the VCH
15. Create a container
16. Use govc to attempt to out-of-band destroy the container from Step 15
17. Clean up the VCH
18. Install the VIC appliance into the cluster with the --ops-grant-perms and --affinity-vm-group options
19. With the ops-user, use govc to attempt to create a resource pool
20. Run a variety of docker operations on the VCH
21. Run privilege-dependent docker operations against the VCH
22. Create a container
23. Use govc to attempt to out-of-band destroy the container from Step 6
24. Clean up the VCH
25. Install the VIC appliance into the cluster without any ops user options
26. Reconfigure the VCH with the --ops-user, --ops-password, --ops-grant-perms options
27. With the ops-user, use govc to attempt to change the DRS settings on the cluster
28. Run a variety of docker operations on the VCH
29. Create a container
30. Use govc to attempt to out-of-band destroy the container from Step 6
31. Clean up the VCH

# Expected Outcome:
* Steps 1-3 should succeed
* Step 4 should fail since the ops-user does not have required permissions to execute the operation
* Steps 5-7 should succeed
* Step 8 should fail since the destroy method should be disabled by VIC
* Steps 9-11 should succeed
* Step 12 should fail since the ops-user does not have required permission to execute the operation
* steps 13-15 should succeed
* Step 16 should fail since the destroy method should be disabled by VIC
* Steps 17 and 18 should succeed
* Step 19 should fail since the ops-user does not have required permission to execute the operation
* Step 20-22 should succeed
* Step 23 should fail since the destroy method should be disabled by VIC
* Step 24-26 should succeed
* Step 27 should fail since the ops-user does not have required permission to execute the operation
* Step 28 and 29 should succeed
* Step 30 should fail since the destroy method should be disabled by VIC
* Step 31 should succeed


# Possible Problems:
None
