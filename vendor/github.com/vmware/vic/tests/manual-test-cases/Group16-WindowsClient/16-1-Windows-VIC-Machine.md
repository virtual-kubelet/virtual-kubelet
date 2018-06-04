Test 16-1 Windows VCH Install
=======

# Purpose:
To verify the VCH appliance can be installed and used from a windows based client

# References:
* vic-machine-windows.exe -h

# Environment:
This test requires that a vSphere server is running and available and a windows client

# Test Steps:
1. From the windows client, download the latest VIC release
2. From within the release package, run vic-machine-windows.exe to install the VCH into the vSphere server with TLS enabled
3. Run a variety of docker commands on the new VCH
4. Delete the VCH
5. Install a new VCH server with TLS disabled
6. Run a variety of docker commands on the new VCH
7. Delete the VCH

# Expected Outcome:
Each VCH should install properly and all docker commands executed should complete without error

# Possible Problems:
None
