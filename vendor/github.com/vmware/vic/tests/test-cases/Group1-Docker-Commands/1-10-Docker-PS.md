Test 1-10 - Docker PS
=======

# Purpose:
To verify that docker ps command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/ps/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker ps
3. Issue docker create busybox /bin/top
4. Issue docker start <containerID>
5. Issue docker create busybox ls
6. Issue docker start <containerID>
7. Issue docker create busybox dmesg
8. Issue docker ps
9. Issue docker ps -a
10. Issue docker create --name jojo busybox /bin/top
11. PowerOn container jojo-* out of band via govc
12. Issue docker ps -q
13. Issue docker create --name koko busybox /bin/top
14. Issue docker start koko
15. Issue docker ps -q
16. PowerOff container koko* out of band via govc
17. Issue docker ps -q
18. Issue docker create -p 8000:80 -p 8443:443 nginx
19. Issue docker ps -a
20. Issue docker run -d -p 6379 redis:alpine
21. Issue docker ps
22. Issue docker create --name lolo busybox /bin/top
23. Issue docker start lolo
24. Issue docker stop lolo
25. Issue docker ps -aq
26. Destroy container lolo* out of band via govc
27. Issue docker ps -aq
28. Issue docker ps -l
29. Issue docker ps -n=2
30. Issue docker ps -ls
31. Issue docker ps -aq
32. Create 3 containers
33. Issue docker ps -aq
34. Issue docker ps -f status=created
35. Issue docker create --name abe --label prod busybox /bin/top
36. Issue docker ps -a -f label=prod
37. Issue docker ps -a -f name=abe
38. Issue docker create -v foo:/dir --name fooContainer busybox
39. Issue docker ps -a -f volume=foo
40. Issue docker ps -a -f volume=foo -f volume=bar
41. Issue docker ps -a -f volume=fo
42. Issue docker network create fooNet
43. Issue docker create --net=fooNet --name fooNetContainer busybox
44. Issue docker ps -a -f network=fooNet
45. Issue docker ps -a -f network=fooNet -f network=barNet
46. Issue docker ps -a -f network=fo
47. Issue docker ps -a -f volume=foo -f network=bar
48. Issue docker ps -a -f network=bar -f volume=foo
49. Issue docker ps -a -f volume=foo -f volume=buz -f network=bar
50. Issue docker create -v buz:/dir --net=fooNet --name buzFooContainer busybox
51. Issue docker ps -a -f volume=buz -f network=fooNet

# Expected Outcome:
* Steps 2-13 should all return without error
* Step 2 should return with only the printed ps command header and no containers
* Step 8 should return with only the information for the /bin/top container
* Step 9 should return with the information for all 3 containers
* Step 10-11 should return without error
* Step 12 should include jojo-* containerVM
* Steps 13-16 should return without error
* Step 17 should not include koko and have one less container than in Step 15
* Step 18 should return without error
* Step 19 should include the port-mappings of Step 18's container
* Step 20 should return without error
* Step 21 should include the port-mappings of Step 20's container
* Steps 22-25 should return without errors
* Step 26 should succeed on ESXi and fail on vCenter with the error:
```
govc: ServerFaultCode: The method is disabled by 'VIC'
```
* Step 27 should include one less container than in Step 25
* Step 28 should include only redis
* Step 29 should include only redis and nginx
* Step 30 should include only redis with SIZE present
* Steps 31-32 should return with error
* Step 33 should include 3 more containers than in Step 31
* Step 34 should include 4 created containers
* Step 35 should return without error
* Step 36 should include only abe
* Step 37 should include only abe
* Step 38 should return without error
* Step 39 should include only fooContainer
* Step 40 should include only fooContainer
* Step 41 should not include any containers
* Steps 42-43 should return without error
* Step 44 should include only fooNetContainer
* Step 45 should include only fooNetContainer
* Step 46 should not include any containers
* Steps 47-49 should not include any containers
* Step 50 should return without error
* Step 51 should include only buzFooContainer

# Possible Problems:
None
