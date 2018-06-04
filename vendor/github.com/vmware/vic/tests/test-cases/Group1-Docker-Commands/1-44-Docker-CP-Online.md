Test 1-44 - Docker CP Online
=======

# Purpose:
To verify that docker cp command for online containers is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/cp/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server and set up test files, directories and volumes
2. Pull busybox image and run a container named online
3. Create directory online:/newdir and file online:/newdir/test.txt
4. Issue docker cp online:/newdir newdir to the new VIC appliance
5. Inspect the host cwd to verify that the copy operation succeeded and clean up copied files
6. Issue docker cp online:/newdir/. bar to the new VIC appliance
7. Inspect bar on the host to verify that the copy operation succeeded and clean up copied files
8. Issue docker cp online:/newdir/test.txt foo.txt to the new VIC appliance
9. Verify that the copy operation succeeded
10. Issue docker cp foo.txt online:/doesnotexist/ to the new VIC appliance
11. Issue docker cp ./foo.txt online:/ to the new VIC appliance
12. Issue docker cp ./bar online:/ to the new VIC appliance
13. Inspect online:/ to verify that the copy operations succeeded
14. Remove online
15. Run a container called online_vol with a single volume attached to it
16. Issue docker cp ./bar online_vol:/vol1/ to the new VIC appliance
17. Inspect online_vol:/vol1 to verify that the copy operation succeeded
18. Create a container called offline that shares a volume with online
19. Issue docker cp content offline:/vol1 to the new VIC appliance
20. Inspect online_vol:/vol1 to verify that the copy operation succeeded
21. Issue docker cp offline:/vol1 . to the new VIC appliance
22. Verify that /vol1 and its content are copied over to host successfully and clean up copied files
23. Remove offline
24. Issue docker cp largefile.txt online_vol:/vol1/ to the new VIC appliance
25. Inspect online_vol:/vol1 to verify that the large file is copied successfully
26. Issue docker cp online_vol:/dne/dne . to the new VIC appliance
27. Issue docker cp online_vol:/dne/. . to the new VIC appliance
28. Remove online_vol
29. Start 10 background processes that issues docker cp foo.txt concurrent:/foo-${idx} to the new VIC appliance
30. Wait for these processes to finish
31. Inspect concurrent:/ to verify that copy operation succeeded
32. Start 10 background processes that issues docker cp largefile.txt concurrent:/vol1/lg-${idx} to the new VIC appliance
33. Wait for these processes to finish
34. Inspect concurrent:/vol1 to verify that copy operation succeeded
35. Start 10 background processes that issues docker cp concurrent:/vol1/lg-${idx} . to the new VIC appliance
36. Wait for these processes to finish
37. Verify that the copy operation succeeded and clean up all the files copied to the host
38. Remove concurrent
39. Run a container called subVol with 2 volumes attached to it
40. Issue docker cp ./mnt subVol:/ to the new VIC appliance
41. Inspect subVol:/mnt, subVol:/mnt/vol1 and subVol:/mnt/vol2 to verify that the copy operation succeeded
42. Issue docker cp subVol:/mnt ./result1 to the new VIC appliance
43. Inspect ./result1 on the host to verify that copy succeeded and remove it afterwards
44. Remove subVol
45. Run a detached container called subVol_on with one volume attached to it
46. Create a container called subVol_off with a volume that's shared with the online container subVol_on
47. Issue docker cp ./mnt subVol_off:/ to the new VIC appliance
48. Stop subVol_on container
49. Start subVol_off to inspect subVol_off:/mnt, subVol_off:/mnt/vol1 and subVol_off:/mnt/vol2 to verify the copy operation succeeded
50. Stop subVol and start subVol_on
51. Issue docker cp subVol_off:/mnt ./result2 to the new VIC appliance
52. Inspect ./result2 on the host to verify that copy succeeded and remove it afterwards
53. Remove subVol_off and subVol_on
54. Clean up created files, directories and volumes

# Expected Outcome:
* Step 1-9 should all succeed
* Step 10 should fail with no such directory
* Step 11-14 should all succeed
* Step 15-25 should all succeed
* Step 26-27 should both fail with no such directory
* Step 28-38 should all succeed
* Step 39-54 should all succeed

# Possible Problems:
* 29-38 are online concurrent tests and are unstable
* 39-53 are online sub volume cp tests and are unstable