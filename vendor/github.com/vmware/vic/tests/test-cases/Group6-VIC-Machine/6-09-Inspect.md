Test 6-09 - Verify vic-machine inspect
=======

# Purpose:
Verify vic-machine inspect functionality

# References:
* vic-machine-linux inspect -h

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install VCH
2. Issue a basic vic-machine inspect command
3. Issue vic-machine inspect config command
4. Issue vic-machine inspect config --format raw command
5. Create a VCH with custom resource settings
6. Issue vic-machine inspect config command
7. Issue vic-machine inspect config --format raw command
8. Create a VCH with some container-network options
9. Issue vic-machine inspect config --format raw command
10. Create a VCH with tlsverify
11. Inspect the VCH without specifying --tls-cert-path
12. Inspect the VCH with a valid --tls-cert-path
13. Inspect the VCH with an invalid --tls-cert-path
14. Create a VCH with --no-tls
15. Inspect the VCH without specifying --tls-cert-path
16. Create a VCH with --no-tlsverify
17. Inspect the VCH without specifying --tls-cert-path
18. Create a VCH with some container-network options

# Expected Outcome:
* Step 1 should succeed 
* Step 2 should succeed and the output should contain the following:
  * VCH ID
  * VCH upgrade information
  * VCH Admin address
  * Address of published ports
  * The docker info command for the VCH
* Steps 3-9 should succeed
* Output from steps 3 and 4 should contain expected flags & values
* Output from steps 6 and 7 should contain the expected resource flags and values
* Output from step 9 should contain the expected container network flags and values
* Steps 10-18 should complete successfully, however, step 12 should show a warning in the output (see below)
* The output of steps 11 and 12 should contain the correct `DOCKER_CERT_PATH`
* The output of step 13 should not contain a `DOCKER_CERT_PATH` and should contain:
```
Unable to find valid client certs
DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments
```
* The outputs of steps 15 and 17 should not contain a `DOCKER_CERT_PATH` and should not contain:
```
Unable to find valid client certs
DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments
```
