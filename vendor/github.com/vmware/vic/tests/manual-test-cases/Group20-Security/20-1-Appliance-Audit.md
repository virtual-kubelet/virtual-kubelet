Test 20-1 - Appliance OS Security Audit
======

# Purpose:
To detect known OS security issues and warnings

# References:
[1 - Lynis Reference](https://cisofy.com/documentation/lynis/#using_lynis)

# Environment:
* requires a working VCSA setup with VCH installed
* Target VCSA has Bash enabled for the root account

# Test Steps:
1. Provision lynis and its dependencies on appliance over ssh
2. run lynis audit system # More customizations can be added in future from here
3. copy lynis log and report under /var/log/ to current directory

# Expected Outcome:
* Each step should return success

# Possible Problems:
None
