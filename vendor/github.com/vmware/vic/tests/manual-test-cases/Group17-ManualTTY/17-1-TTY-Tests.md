Test 17-1 - TTY Tests
=======

# Purpose:
To verify that docker commands using TTY work with VIC

# References:


# Environment:
This test requires that a vSphere server is running and available

# Test Cases
1. Deploy VIC appliance to vSphere server
2. Issue docker run -it busybox date to the new VCH
3. Issue docker run -it busybox df to the new VCH
4. Issue docker run -it busybox top to the new VCH
5. Issue docker create -it busybox /bin/top to VIC appliance
6. Issue docker start -ai <containerID> from previous step
7. Issue commands to make container stuck in starting status, and then test docker stop can stop the container
8. Issue commands to test the second start works after docker run with -it

### Expected Outcome:
* Steps 1-8 should all succeed and return the expected output from those commands
