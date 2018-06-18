Test 1-21 - Docker Volume LS
=======

# Purpose:
To verify that docker volume ls command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/volume_ls/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker volume create --name=testVol
3. Issue docker volume ls
4. Issue docker volume ls -q
5. Issue docker volume ls -f bogusfilter=test
6. Issue docker create --name=danglingVol
7. Issue docker create -v testVol:/test busybox
8. Issue docker volume ls -f dangling=true
9. Issue docker volume ls -f dangling=false
10. Issue docker volume ls -f name=dang
11. Issue docker volume ls -f driver=vsphere
12. Issue docker volume ls -f driver=vsph
13. Issue docker volume create --name=labelVol --label=labeled
14. Issue docker volume ls -f label=labeled
15. Issue docker volume ls -f dangling=true -f name=dang
16. Issue docker volume ls -f dangling=false -f name=dang

# Expected Outcome:
* Step 3 should result in each volume being listed with both driver and volume name
* Step 4 should result in each volume being listed with only the volume name being listed
* Step 5 should result in the following error:
```
Error response from daemon: Invalid filter 'bogusfilter'
```
* Step 8 should result in only danglingVol being listed
* Step 9 should result in only testVol being listed
* Step 10 should result in only danglingVol being listed
* Step 11 should result in danglingVol and testVol being listed
* Step 12 should result in no volumes being listed
* Step 14 should result in only labelVol being listed
* Step 15 should result in only danglingVol being listed
* Step 16 should result in no volumes being listed

# Possible Problems:
* VIC requires you to specify storage on creation of the VCH that volumes can be created from, so when installing the VCH make sure to specify this parameter: --volume-store=