Test 1-17 - Docker Network Connect
=======

# Purpose:
To verify that docker network connect command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/network_connect/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker network create cross1-network
3. Issue docker network create cross1-network2
4. Issue docker create --net cross1-network --name cross1-container busybox /bin/top
5. Issue docker network connect cross1-network2 <containerID>
6. Issue docker start <containerID>
7. Issue docker create --net cross1-network --name cross1-container2 debian ping -c2 cross1-container
8. Issue docker network connect cross1-network2 <containerID>
9. Issue docker start <containerID>
10. Issue docker logs --follow cross1-container2
11. Issue docker create --name cross1-container3 --net cross1-network busybox ping -c2 cross1-container3
12. Issue docker network connect cross1-network2 cross1-container3
13. Issue docker start cross1-container3
14. Issue docker logs --follow cross1-container3
15. Issue docker network create test-network
16. Issue docker create busybox ifconfig
17. Issue docker network connect test-network <containerID>
18. Issue docker start <containerID>
19. Issue docker logs <containerID>
20. Issue docker network connect test-network fakeContainer
21. Issue docker network connect fakeNetwork <containerID>
22. Issue docker network create cross2-network
23. Issue docker network create cross1-network2
24. Issue docker run -itd --net cross2-network --name cross2-container busybox /bin/top
25. Get the above container's IP - ${ip}
26. Issue docker run --net cross2-network2 --name cross2-container2 debian ping -c2 ${ip}
27. Issue docker logs --follow cross2-container2
28. Issue docker run -d --net cross2-network -p 8080:80 nginx
29. Get the above container's IP - ${ip}
30. Issue docker run --net cross2-network2 --name cross2-container3 debian ping -c2 ${ip}
31. Issue docker logs --follow cross2-container3
32. Issue docker network create --internal internal-net
33. Issue docker run --net internal-net busybox ping -c1 www.google.com
34. Issue docker network create public-net
35. Issue docker run --net internal-net --net public-net busybox ping -c2 www.google.com
36. Issue docker run -itd --net internal-net busybox
37. Get the above container's IP - ${ip}
38. Issue docker run --net internal-net busybox ping -c2 ${ip}
39. Issue docker network create foonet
40. Issue docker network create barnet
41. Issue docker network create baznet
42. Issue docker pull busybox and docker create 
43. Issue docker network connect to connect the above container to the networks in Steps 39-41 concurrently
44. Issue docker inspect to check that the container is connected to the networks
45. Issue docker start and then rm -f for the container for a quick lifecycle check

# Expected Outcome:
* Steps 2-9 should return without errors
* Step 10's output should contain "2 packets transmitted, 2 packets received"
* Steps 11-13 should return without errors
* Step 14's output should contain "2 packets transmitted, 2 packets received"
* Step 15-17 should complete successfully
* Step 19 should print the results of the ifconfig command and there should be two network interfaces in the container(eth0, eth1)
* Step 20 should result in an error with the following message:
```
Error response from daemon: No such container: fakeContainer
```
* Step 21 should result in an error with the following message:
```
Error response from daemon: network fakeNetwork not found
```
* Steps 22-26 should return without errors
* Step 27's output should contain "2 packets transmitted, 0 packets received, 100% packet loss"
* Steps 28-30 should return without errors
* Step 31's output should include "2 packets transmitted, 0 packets received, 100% packet loss"
* Step 32 should return without an error
* Step 33 should return with a non-zero exit code and the output should contain "Network is unreachable"
* Step 34 should return without an error
* Step 35's output should contain "2 packets transmitted, 2 packets received"
* Steps 36-37 should return without errors
* Step 38's output should contain "2 packets transmitted, 2 packets received"
* Steps 39-43 should succeed
* Step 44's output should contain "foonet", "barnet" and "baznet"
* Step 45 should succeed

# Possible Problems:
None
