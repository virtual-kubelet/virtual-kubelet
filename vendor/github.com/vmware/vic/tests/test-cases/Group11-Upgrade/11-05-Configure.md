Test 11-05 - Configure
=======

# Purpose:
To verify vic-machine configure can upgrade with --upgrade specified

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Download vic_1.2.1.tar.gz from gcp
2. Deploy VIC 1.2.1 to vsphere server
3. Using latest version vic-machine to configure this VCH

# Expected Outcome:
* Step 3 should get expected error

# Possible Problems:
* This suite may fail when run locally due to a `vic-machine upgrade` issue. Since `vic-machine` checks the build number of its binary to determine upgrade status and a locally-built `vic-machine` binary may not have the `BUILD_NUMBER` set correctly, `vic-machine upgrade` may fail with the message `foo-VCH has same or newer version x than installer version y. No upgrade is available.` To resolve this, follow these steps:
  * Set `BUILD_NUMBER` to a high number at the top of the `Makefile` - `BUILD_NUMBER ?= 9999999999`
  * Re-build binaries - `sudo make distclean && sudo make clean && sudo make all`
