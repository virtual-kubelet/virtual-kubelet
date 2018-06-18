Test 1-19 - Docker Volume Create
=======

# Purpose:
To verify that docker volume create command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/volume_create/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker volume create to the VIC appliance
2. Create container with volume mounted over /etc and ensure that name resolution functions as expected
3. Issue docker volume create --name=test to the VIC appliance
4. Issue docker volume create --name=test to the VIC appliance
5. Issue docker volume create -d fakeDriver --name=test2
6. Issue docker volume create --name=test3 --opt VolumeStore=fakeStore
7. Issue docker volume create --name=test4 --opt Capacity=100
8. Issue docker volume create --name=test5 --opt Capacity=0
9. Issue docker volume create --name=test6 --opt Capacity=-1
10. Issue docker volume create --name=test7 --opt Capacity=2147483647
11. Issue docker volume create --name=test8 --opt Capacity=9999999999
12. Issue docker volume create --name=test???
13. Issue docker volume create --name=multipleX --opt Capacity=2MB ten times rapidly
14. Create container with an anonymous volume in the Dockerfile, and verify that the files in the volume exist
15. Create container with a named volume and verify that base image files are copied to the named volume
16. Create container with a named volume. Modify the copied image file. Remount the volume in a new container.
17. Run container with a volume, and run another container with same volume

# Expected Outcome:
* Steps 2 and 3 should complete successfully and return the name of the volume created, you should then be able to see the volume has been created
* Step 4 should result in error with the following error message:
```
Error response from daemon: A volume named test already exists. Choose a different volume name.
```
* Step 5 should result in error with the following error message:
```
error looking up volume plugin fakeDriver: plugin not found
```
* Step 6 should result in error with the following message:
```
Error looking up volume store fakeStore: datastore not found
```
* Step 7 should result in success and the volume created should be 100MB in size
* Step 8 should result in error and indicate that the capacity suggested is invalid
* Step 9 should result in error and indicate that the capacity suggested is invalid
* Step 10 should result in error and indicate that the datastore cannot create that big of a volume
* Step 11 should result in error and indicate that the capacity suggested in invalid
* Step 12 should result in error with the following message:
```
Error response from daemon: create test???: "test???" includes invalid characters for a local volume name, only "\[a-zA-Z0-9][a-zA-Z0-9_.-]" are allowed
```
* Step 13 should not result in any of the volume create operations failing
* Step 14 should result in success and print data in the volume
* Step 15 should result in success and print data in the volume
* Step 16 should result in success and the second container should contain the modified file contents
* Step 17 should result in error with message `devices <volume id> in use`

# Possible Problems:
* VIC requires you to specify storage on creation of the VCH that volumes can be created from, so when installing the VCH make sure to specify this parameter: --volume-store=
