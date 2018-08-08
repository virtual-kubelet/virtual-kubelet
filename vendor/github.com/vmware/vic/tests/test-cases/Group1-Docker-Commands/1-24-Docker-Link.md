Test 1-24 - Docker Link
=======

# Purpose:
To verify that docker --link/--net-alias commands are supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/run/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker network create jedi
3. Issue docker pull busybox
4. Issue docker run -it -d --net jedi --name first busybox
5. Issue docker run -it --net jedi busybox ping -c3 first
6. Issue docker run -it --net jedi --link first:1st busybox ping -c3 1st
7. Issue docker run -it -d --net jedi --net-alias 2nd busybox
8. Issue docker run -it --net jedi busybox ping -c3 2nd


# Expected Outcome:
* Every step should result in success

# Possible Problems:
None