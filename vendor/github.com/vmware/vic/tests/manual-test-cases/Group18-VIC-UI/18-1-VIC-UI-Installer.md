Test 18-1 - VIC UI Installation
======

# Purpose:
To test all possible installation failures and success scenarios on VCSA

# References:

# Environment:
* Testing VIC UI requires a working VCSA setup with VCH installed
* Target VCSA has Bash enabled for the root account

# Test Steps:
1. Check if the configs file exists
2. Ensure UI plugin is not registered with VC before testing
3. Try installing UI without the configs file
4. Try installing UI with vsphere-client-serenity folder missing
5. Try installing UI with vCenter IP missing
6. Try installing UI with invalid vCenter IP
7. Try installing UI with wrong vCenter credentials
8. [SKIP] Try installing UI with wrong vCenter root password
9. Try installing UI with Bash disabled
10. Install UI successfully without a web server
11. Try installing UI when it is already installed
12. Install UI successfully with the --force flag when the plugin is already registered
13. Try installing UI with a web server and an invalid URL to the plugin zip file

# Expected Outcome:
* Each step should return success

# Possible Problems:
Attempting to `ssh` into VCSA with a wrong root password three times locks the account for a certain amount of time. For this reason Step 8 is skipped.
