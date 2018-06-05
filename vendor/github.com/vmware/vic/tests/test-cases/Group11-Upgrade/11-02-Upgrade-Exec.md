Test 11-02 - Upgrade Exec
=======

# Purpose:
To verify that exec does not work in VIC version 0.9.0 and lower

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Download vic_0.9.0.tar.gz from gcp
2. Deploy VIC 0.9.0 to vsphere server
3. Create a busybox container running the background
4. Upgrade VCH to latest version
5. Run docker exec on container created in step (3.)
6. Create new container
7. Run docker exec on new container in (6.)

# Expected Outcome:
* Step 5 should fail
* All other steps should result in success

# Possible Problems:
* This suite may fail when run locally due to a `vic-machine upgrade` issue. Since `vic-machine` checks the build number of its binary to determine upgrade status and a locally-built `vic-machine` binary may not have the `BUILD_NUMBER` set correctly, `vic-machine upgrade` may fail with the message `foo-VCH has same or newer version x than installer version y. No upgrade is available.` To resolve this, follow these steps:
  * Set `BUILD_NUMBER` to a high number at the top of the `Makefile` - `BUILD_NUMBER ?= 9999999999`
  * Re-build binaries - `sudo make distclean && sudo make clean && sudo make all`
