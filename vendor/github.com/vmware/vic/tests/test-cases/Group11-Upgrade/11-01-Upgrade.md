Test 11-01 - Upgrade
=======

# Purpose:
To verify vic-machine upgrade can upgrade VCH from a certain version

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Download vic_1.3.1.tar.gz from gcp
2. Deploy VIC 1.3.1 to vsphere server
3. Issue docker network create bar, creating a new network called "bar"
4. Create container with port mapping
5. Upgrade VCH to latest version with short timeout 1s
6. Create a named volume
7. Create a container with a mounted anonymous and named volume
8. Upgrade VCH to latest version
9. Check that one of the older VCH's containers has a create timestamp in seconds, and one from the upgraded VCH uses nanoseconds
10. Check that the above two containers have valid human-readable create times in docker ps output
11. Verify that the volumes are still there using inspect
12. Roll back to the previous version
13. Upgrade again to the upgraded version
14. Verify that the volumes are still there using inspect
15. Check the previous created container and image are still there
15. Attempt to rename an old container created with a VCH that doesn't support rename
16. Rename a new container created with a VCH that supports rename
17. Check the previous created container's display name and datastore folder name
18. Check the display name and datastore folder name of a new container created after VCH upgrade

# Expected Outcome:
* Step 5 should fail with timeout
* Step 15 should result in an error containing the following message:
```
does not support rename
```
* Step 16 should succeed and the container's new name should be present in ps, inspect and govc vm.info output.
* Step 17 should show that both the container's display name and datastore folder name are containerName-containerID
* Step 18 should show that (1) on a non-vsan setup, the container's display name is containerName-containerShortID while the datastore folder name is containerID, or (2) on a vsan setup, both the container's display name and datastore folder name are containerName-containerShortID
* All other steps should result in success

# Possible Problems:
* Upgrade test will upgrade VCH from build 1.3.1, because that build has VCH restart and configuration restart features done.
* Before GA, if there is any VCH configuration change, please bump upgrade from version, and be sure to add cases to cover those changes.
* After GA, the upgrade from version will be GA release version.
* This suite may fail when run locally due to a `vic-machine upgrade` issue. Since `vic-machine` checks the build number of its binary to determine upgrade status and a locally-built `vic-machine` binary may not have the `BUILD_NUMBER` set correctly, `vic-machine upgrade` may fail with the message `foo-VCH has same or newer version x than installer version y. No upgrade is available.` To resolve this, follow these steps:
  * Set `BUILD_NUMBER` to a high number at the top of the `Makefile` - `BUILD_NUMBER ?= 9999999999`
  * Re-build binaries - `sudo make distclean && sudo make clean && sudo make all`
