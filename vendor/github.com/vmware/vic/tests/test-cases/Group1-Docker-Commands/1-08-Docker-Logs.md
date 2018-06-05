Test 1-08 - Docker Logs
=======

# Purpose:
To verify that docker logs command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/logs/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC build 0.8.0 to appliance to vSphere server
2. Issue docker run -d busybox sh -c "echo These pretzels are making me thirsty"
3. Issue docker logs <ID1>
4. Issue docker logs --timestamps <ID1>
5. Upgrade the VCH to current build
6. Issue docker run -d busybox sh -c "echo Whats the deeeal with Ovaltine"
7. Issue docker logs --timestamps <ID2>
8. Issue docker logs --timestamps <ID1>
9. Issue docker create busybox /bin/sh -c 'seq 1 5000' to the VIC appliance
10. Issue docker start <containerID> to the VIC appliance
11. Issue docker logs <containerID> to the VIC appliance
12. Issue docker logs --tail=all <containerID> to the VIC appliance
13. Issue docker logs --tail=200 <containerID> to the VIC appliance
14. Issue docker logs --tail=0 <containerID> to the VIC appliance
15. Issue docker create -t busybox /bin/sh -c 'for i in $(seq 1 5) ; do sleep 1 && echo line $i; done'
16. Issue docker start <containerID> to the VIC appliance
17. Issue docker logs --follow <containerID> to the VIC appliance
18. Issue docker create busybox /bin/sh -c 'trap "seq 11 20; exit" HUP; seq 1 10; while true; do sleep 1; done'
19. Issue docker start <containerID> to the VIC appliance
20. Issue docker logs <containerID> to the VIC appliance, waiting for the first 10 lines
21. Issue docker kill -s HUP <containerID> to the VIC appliance, generating the next 10 lines
22. Issue docker logs --tail=5 --follow <containerID> to the VIC appliance
23. Issue docker pull ubuntu
24. Issue docker run ubuntu /bin/cat /bin/hostname >/tmp/hostname
25. Issue docker logs <containerID> >/tmp/hostname-logs
26. Issue sha256sum on /tmp/hostname and /tmp/hostname-logs
27. Issue docker run ubuntu /bin/ls >/tmp/ls
28. Issue docker logs <containerID> >/tmp/ls-logs
29. Issue sha256sum on /tmp/ls and /tmp/ls-logs
30. Issue docker logs --since=1s <containerID> to the VIC appliance
31. Issue docker logs --timestamps <containerID> to the VIC appliance
32. Issue docker logs
33. Issue docker logs fakeContainer

# Expected Outcome:
* Steps 1-3,5-7,9-29 should all complete without error
* Step 3 should output "These pretzels are making me thirsty"
* Step 4 should output "vSphere Integrated Containers does not yet support '--timestamps'"
* Step 7 should output "Whats the deeeal with Ovaltine?" with a timestamps
* Step 8 should output "container <ID1> does not support '--timestamp'"
* Step 13 should output 200 lines
* Step 14 should output 0 lines
* Step 17 should have last line be
```
line 5
```
* Step 20 should output 10 lines
* Step 22 should output 15 lines
* Steps 26 and 29 should produce matching sha256 hashes for both files
* Step 30 should output 3 lines
* Step 31 should result in an error with the following message:
```
Error: vSphere Integrated Containers does not yet support timestamped logs.
```
* Step 32 should output all lines
* Step 33 should result in an error with the following message:
```
Error: No such container: fakeContainer
```

# Possible Problems:
* This suite may fail when run locally due to a `vic-machine upgrade` issue. Since `vic-machine` checks the build number of its binary to determine upgrade status and a locally-built `vic-machine` binary may not have the `BUILD_NUMBER` set correctly, `vic-machine upgrade` may fail with the message `foo-VCH has same or newer version x than installer version y. No upgrade is available.` To resolve this, follow these steps:
  * Set `BUILD_NUMBER` to a high number at the top of the `Makefile` - `BUILD_NUMBER ?= 9999999999`
  * Re-build binaries - `sudo make distclean && sudo make clean && sudo make all`
