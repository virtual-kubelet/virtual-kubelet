Test 2-01 - Docker Archive
=======

# Purpose:
Get image tar file from debug PL and docker hub, and compare each file's digest inside of the tar and also tar file digest, to get exactly same tar file content in VIC with docker.

# References:
imagec -h

# Environment:
This test requires that a debug = 3 VIC appliance is available to execute the docker integration tests against

# Test Steps:
1. Deploy VIC appliance with debug = 3 to a test server
2. Issue docker pull command to pull image ubuntu to VCH
3. Issue imagec pull command to download image ubuntu layers tar from docker hub, which will store docker image layers to local file system
4. Issue imagec save command to download image ubuntu tar files from PL, which will store VIC image layers to local file system
5. Compare tar file content through tar -tvf
6. untar tar files, and compare each file digest
7. compare tar file digest

# Expected Outcome:
1. Steps 1~4 should succeed
2. Step 5 should get equal file content
3. Step 6 should get equal file digest
4. Step 7 should get equal digest - TODO: add this test after we got tar file digest parity

# Possible Problems:
None
