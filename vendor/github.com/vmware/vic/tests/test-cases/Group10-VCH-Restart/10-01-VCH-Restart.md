Test 10-01 - VCH Restart
=======

# Purpose:
To verify that restarting a VCH results in a consistent and correct state

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Issue docker network create bar, creating a new network called "bar"
3. Create container with port mapping
4. Reboot VCH
5. Issue a docker network ls
6. Issue docker inspect
7. Issue docker stop, start and ls
8. Check container service in specified port
9. Start container with same port
10. Deploy VIC appliance with open container network
11. Create container on the open network, and create container with port mapping
12. Reboot VCH
13. Check container service in specified port
14. Create container with volume and then reboot VCH
15. Inspect container to check volume info

# Expected Outcome:
* VCH should reboot within a reasonable amount of time
* After VCH restart, network ls should have the previously created network listed
* Step 6, 7 and 8 should result in success
* Step 9 should result in false
* Step 12 (VCH reboot with open container network) should succeed within a reasonable amount of time
* Step 13 would result in success
* Step 14-15 should result in success

#Possible Problems:
None
