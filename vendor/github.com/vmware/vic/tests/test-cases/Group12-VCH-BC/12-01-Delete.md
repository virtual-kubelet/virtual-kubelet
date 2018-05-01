Test 12-01 - Delete
=======

# Purpose:
To verify vic-machine delete can delete VCH and its containers created by vic 1.1.1

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Download vic_1.1.1.tar.gz from gcp
2. Deploy VIC 1.1.1 to vSphere server
3. Create container
3. Using latest version vic-machine to delete this VCH

# Expected Outcome:
* All steps should result in success
