Test 7-01 - Regression test
=======

# Purpose:
To verify general functionality of the product in a rapid, repeatable manner for regression testing of all commits

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Issue a docker pull busybox
3. Issue a docker images, verify that busybox image shows up
4. Issue a docker create busybox /bin/top
5. Issue a docker start <containerID>
6. Issue a docker ps, verify that the container shows up as running
7. Issue a docker stop <containerID>
8. Issue a docker ps, verify that the container is stopped
9. Issue a docker rm <containerID>
10. Pull container log bundle from VICadmin and ensure that the container's vmware.log is present
11. Issue a docker ps, verify that the container is removed
12. Issue a docker rmi busybox
13. Issue a docker images, verify that busybox image is gone
14. Remove the VIC appliance with vic-machine delete command, verify that the appliance was removed

# Expected Outcome:
VIC appliance should respond without error to each of the commands

# Possible Problems:
None
