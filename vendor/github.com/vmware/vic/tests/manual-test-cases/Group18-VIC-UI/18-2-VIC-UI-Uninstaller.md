Test 18-2 - VIC UI Uninstallation
======

# Purpose:
To test all possible uninstallation failures and success scenarios on VCSA

# References:

# Environment:
* Testing VIC UI requires a working VCSA setup with VCH installed
* Target VCSA has Bash enabled for the root account

# Test Steps:
1. Check if the configs file exists
2. Ensure UI plugin is already registered with VC before testing
3. Try uninstalling UI without the configs file
4. Try uninstalling UI with vsphere-client-serenity folder missing
5. Try uninstalling UI with vCenter IP missing
6. Try uninstalling UI with wrong vCenter credentials
7. Uninstall UI successfully
8. Try uninstalling UI when it's already uninstalled

# Expected Outcome:
* Each step should return success

# Possible Problems:
