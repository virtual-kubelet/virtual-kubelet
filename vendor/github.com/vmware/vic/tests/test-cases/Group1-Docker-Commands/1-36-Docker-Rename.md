Test 1-36 - Docker Rename
=======

# Purpose:
To verify that the docker rename command is supported by VIC appliance.

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/rename)

# Environment:
This test requires that a vSphere server is running and available.

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker rename foo bar
3. Issue docker pull busybox
4. Issue docker create --name cont1-name1 busybox
5. Issue docker rename cont1-name1 cont1-name2
6. Verify that the container was renamed by checking ps, inspect and govc vm.info
7. Issue docker run -dit --name cont2-name1 busybox
8. Issue docker rename cont2-name1 cont2-name2
9. Verify that the container was renamed by checking ps, inspect and govc vm.info
10. Issue docker run -dit --name cont3-name1 busybox
11. Issue docker stop cont3-name1
12. Issue docker rename cont3-name1 cont3-name2
13. Issue docker start cont3-name2
14. Verify that the container was renamed by checking ps, inspect and govc vm.info
15. Issue docker create --name cont4 busybox
16. Issue docker rename cont4 ""
17. Issue docker create --name cont5 busybox
18. Issue docker create --name cont6 busybox
19. Issue docker rename cont5 cont5
20. Issue docker rename cont5 cont6
21. Issue docker create --name cont7-name1 busybox
22. Issue docker rename cont7-name1 cont7-name2
23. Issue docker start cont7-name1
24. Issue docker run --link cont7-name2:cont7alias busybox ping -c2 cont7alias
25. Issue docker run busybox ping -c2 cont7-name2
26. Issue docker run -dit --name cont8-name1 busybox
27. Issue docker rename cont8-name1 cont8-name2
28. Issue docker stop cont8-name2
29. Issue docker start cont7-name2
30. Issue docker run --link cont8-name2:cont8alias busybox ping -c2 cont8alias
31. Issue docker run busybox ping -c2 cont8-name2
32. Issue docker run -dit --name cont9-name1 busybox
33. Issue docker rename cont9-name1 cont9-name2
34. Issue docker run --link cont9-name2:cont9alias busybox ping -c2 cont9alias
35. Issue docker run busybox ping -c2 cont9-name2

# Expected Outcome:
* Step 2 should result in an error with the following message:
```
Error: No such container: foo
```
* Steps 3-15 should return without errors
* Step 16 should result in an error containing the following message:
```
Neither old nor new names may be empty
```
* Steps 17 and 18 should return without errors
* Step 19 and 20 should return with errors
* Steps 21-23 should return without errors
* Steps 24 and 25 should succeed and their output should contain:
```
2 packets transmitted, 2 packets received 
```
* Steps 25-29 should return without errors
* Steps 30 and 31 should succeed and their output should contain:
```
2 packets transmitted, 2 packets received 
```
* Steps 32 and 33 should return without errors
* Steps 34 and 35 should succeed and their output should contain:
```
2 packets transmitted, 2 packets received 
```

# Possible Problems:
None
