Test 5-22 - NFS Volume
=======

# Purpose:
To verify that NFS shared volumes work with currently supported docker commands

# References:
[1 - Best practices for running VMware vSphere on NFS](http://www.vmware.com/content/dam/digitalmarketing/vmware/en/pdf/techpaper/vmware-nfs-bestpractices-white-paper-en.pdf)

[2 - Docker Command Line Reference - Volume Create](https://docs.docker.com/engine/reference/commandline/volume_create/)

[3 - Docker Command Line Reference - Exec](https://docs.docker.com/engine/reference/commandline/exec/)

[4 - Docker Command Line Reference - Volume Inspect](https://docs.docker.com/engine/reference/commandline/volume_inspect/)

[5 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/volume_ls/)


# Environment:
This test requires access to VMware Nimbus for dynamic ESXi and NFS server creation

# Test Steps:
1. Deploy VIC appliance to an ESX and use a read only NFS mount point
2. Issue docker volume create on read only volume
3. Deploy VIC appliance to an ESX and use fake NFS mount point
4. Deploy VIC appliance to an ESX and use valid NFS mount point
5. Issue docker volume create using no name for the volume (unnamed) on VolumeStore=nfsVolumeStore (NFS volume)
6. Issue docker run -v using unnamed volume and the mount command, run docker wait, then docker rm on container
7. Issue docker volume create --name=nfs_default_%{VCH-NAME} on VolumeStore=nfsVolumeStore (NFS volume)
8. Issue docker run -v using unnamed volume and the mount command, run docker wait, then docker rm on container
9. Issue docker volume create on unnamed volume
10. Issue docker volume create on named volume
11. Issue docker volume create --name="test!@\#$%^&*()"
12. Create container ${createFileContainer} using named nfs volume
13. Issue docker exec -i ${createFileContainer} echo # --> to write contents to a file (created by this echo command) on NFS volume
14. Issue docker exec -i ${createFileContainer} ls   # to verify file is created in the correct directory on NFS volume
15. Issue docker exec -i ${createFileContainer} cat  # to verify contents of the file
16. Create a container using named nfs volume and echo # to append more contents to the file used by earlier container
17. Create a container using named nfs volume and echo # to append to the same file
18. Create a container using named nfs volume and echo # to append to the same file
19. Create a container using named nfs volume and cat # verify contents of the file
20. Create a detached container using named nfs volume using named nfs volume to cat the file from last test
21. Issue docker logs to see the results of the cat command # verify contents of the file
22. Create a container using named nfs volume and rm the file just used.
23. Create a container using named nfs volume and cat the file that was just removed
24. Issue docker start on detached container from earlier
25. Issue docker logs on detached container
26. Spin up on container per item in a list to write once a sec to a file the value passed in from the list and save the container ids
27. Create container using named nfs volume and cat the contents of the file from the previous step
28. Check output from each container that was writing to the file.
29. Stop all the running write containers.
30. Issue docker volume inspect ${nfsNamedVolume}
31. Issue docker volume ls
32. Issue docker volume rm ${nfsDefaultVolume}
33. Issue docker volume rm ${nfsNamedVolume}
34. Create a container using a standard volume and named NFS volume
35. Inspect the container to check volume info
37. Restart the VCH
38. Inspect the container to check that the volume info is the same as before
39. Create a detached container using named nfs volume and write to file every second
40. Create a container using named nfs volume and tail the file from previous step
41. Kill the NFS Server from Nimbus
42. Create a container using named nfs volume from killed NFS server and tail the file from previous step
43. Create a container using named nfs volume from killed NFS server and write to file from previous step
44. Create a container using named nfs volume from killed NFS server and ls the mydata directory



# Expected Outcome:
* Step 1 will pass VCH creation but should fail in mounting the read only NFS mount point
* Step 2 should result in error with the following error message:
```
Error response from daemon: No volume store named (${nfsReadOnlyVolumeStore}) exists
```
* Step 3 will pass VCH creation but should fail in mounting the fake NFS mount point
* Step 4 should complete successfully; VCH should be created/installed
* Step 5 should complete successfully and return a long string name for the volume created
* Step 6 should verify that the NFS volume is mounted on a temp container; container rm should succeed
* Step 7 should complete successfully and return named volume
* Step 8 same as step 4 but using the named volume instead
* Step 9 should result in error with the following error message:
```
Error response from daemon: A volume named ${nfsDefaultVolume} already exists. Choose a different volume name.
```
* Step 10 should result in error with the following error message:
 ```
 Error response from daemon: A volume named ${nfsNamedVolume} already exists. Choose a different volume name.
 ```
* Step 11 should result in error with the following message:
```
Error response from daemon: create test???: "test???" includes invalid characters for a local volume name, only "\[a-zA-Z0-9][a-zA-Z0-9_.-]" are allowed
```
* Steps 12 - 22 should result in success
* Step 23 should result in error with the following error message:
```
cat: can't open 'mydata/test_nfs_file.txt': No such file or directory
```
* Step 24 and 25 should succeed, however step 25 will show the same error as above in the logs
* Steps 26 - 29 should result in success
* Step 30 should result in a properly formatted JSON response
* Step 31 should result in each nfs volume being listed with both driver and volume name
* Step 32 should result in success and the volume should not be listed anymore
* Step 33 should result in error with the following message:  
```
Error response from daemon: volume ${nfsNamedVolume} in use by
```
* Steps 34-38 should result in success
* Steps 39 - 41 should result in success; step 41 should kill/drop the server
* Step 42 should result in error with the following message:
```
Server error from portlayer: unable to wait for process launch status:
```
* Steps 43 - 44 should result in error with the rc = 125.


# Possible Problems:
Mount command may be affected by Nimbus's performance returning '' when the volume was successfully created/mounted