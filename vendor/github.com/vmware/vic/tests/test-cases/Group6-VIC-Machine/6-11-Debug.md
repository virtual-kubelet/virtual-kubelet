Test 6-11 - Verify vic-machine debug
=======

# Purpose:
Verify vic-machine debug functions

# References:
* vic-machine-linux debug -h

# Environment:
This test requires that a vSphere server is running and available


# Test Cases
======

# Enable SSH
1. Create VCH
2. Generate ssh keypair with ssh-keygen
3. Run vic-machine debug to enable SSH, supplying public key for authorized_keys file
4. ssh to endpointVM and run `/bin/true`, asserting success via exit status

# Expected Results
* All steps should succeed

# Password Change When Expired
1. Create VCH
2. Generate ssh keypair with ssh-keygen
3. Run vic-machine debug to enable SSH, supplying public key for authorized_keys file
4. ssh to endpointVM using private key and run `/bin/true`, asserting success via exit status
5. Change date to +6 years on current time - this is past the support window
6. ssh to endpointVM using private key and run `/bin/true`, asserting failure via exit status
7. Run vic-machine debug to enable SSH, supplying a dictionary password that would be rejected by cracklib if change were interactive
8. ssh to endpointVM using password and run `/bin/true`, asserting success via exit status

# Expected Results
* Step 6 should fail due to expired password
* All other steps should succeed
