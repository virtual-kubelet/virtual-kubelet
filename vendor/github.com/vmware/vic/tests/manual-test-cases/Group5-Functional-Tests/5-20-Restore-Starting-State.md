Test 5-20 - Restore Starting State
=======

# Purpose:
To verify that a container that is starting will have it's starting state restored
during appliance reboot

# Test Steps:
1. Deploy VCH
2. Enable ESXi Firewall
3. Pull busybox
4. docker run -i busybox
5. verify run timeout
6. restart VCH
7. verify state starting

# Expected Outcome:
All test steps should complete without error

# Possible Problems:
Only will work with stand alone esxi
