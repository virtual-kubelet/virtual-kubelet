Test 6-16 - Verify vic-machine configure
=======

# Purpose:
Verify vic-machine configure

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available

# Test Steps
1. Deploy VCH
2. Configure VCH
3. Check the debug state of the VCH
4. Check the debug state of an existing containerVM
5. Configure the VCH by setting the debug state to 0
6. Check the debug state of the VCH
7. Check the debug state of the existing containerVM
8. Create a new container and check the debug state of it
9. check whether the output of vic-machine inspect contains the desired debug state
10. Configure the VCH by adding a container network
11. Run docker network ls
12. Run vic-machine inspect config
13. Run a container with the new container network
14. Configure the VCH by adding a new container network without specifying the previous network
15. Configure the VCH by adding a new container network while specifying the previous network
16. Run docker network ls
17. Run vic-machine inspect config
18. Run a container with the new container network
19. Configure the VCH by attempting to change an existing container network
20. Configure VCH http proxy
21. Verify http proxy is set correctly through govc
22. Configure the VCH's operations user credentials
23. Run vic-machine inspect config
24. Reset VCH http proxy using VCH ID
25. Verify http proxy is reset correctly through govc
26. Run vic-machine inspect config
27. Configure VCH dns server to 10.118.81.1 and 10.118.81.2
28. Run vic-machine inspect config
29. Reset VCH dns server to default
30. Run vic-machine inspect config
31. Configure VCH resources
32. Verify VCH configuration through vic-machine inspect
33. Configure VCH resources with too small values
34. Verify VCH configuration is rollback to old value
35. Configure the VCH by adding a new volume store
36. Run vic-machine inspect config
37. Run docker info
38. Create a volume on the default volume store
39. Create a volume on the new volume store
40. Run docker volume ls
41. Configure the volume stores without specifying an existing volume store
42. Configure the volume stores by attempting to change an existing volume store
43. Configure the VCH by adding a new volume store with a URL scheme
44. Run vic-machine inspect config
45. Verify configure is in vic-machine dialog

# Expected Outcome
* Step 15 should fail with an error message saying that the existing container network must be specified
* Step 20 should fail with an error message saying that changes to existing container networks are not supported
* Step 24's output should contain the operations user's name and the host thumbprint
* Step 36 and 37's output should contain both volume stores
* Step 40's output should contain both volumes
* Step 41 should fail with an error message saying that existing volume stores must be specified
* Step 42 should fail with an error message saying that changes to existing volume stores are not supported
* Step 44's output should contain all three volume stores
* All other steps should succeed
