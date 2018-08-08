Test 5-21 - Datastore-Path
=======

# Purpose:
To verify that we can specify datastore paths that have spaces.
Also, to test datastore path with specified DS schemes.

# Environment:
This test requires an ESX with a deployed VIC Appliance.

# Test Steps:
1. Deploy Nimbus ESX server and install VCH
2. Run custom vic-machine create command where we specify an image store with DS scheme included
3. Rename 'datastore1' to 'datastore (1)'
4. Install VCH using 'datastore (1)
5. Run custom vic-machine create command using 'datastore (1)' where we specify an image store with DS scheme

# Expected Outcome:
1. The VCH should deploy w/o errors
2. Vic-machine create command should pass when installing using image store with DS scheme included
3. Renaming of datastore to 'datastore (1)' should succeed
4. VCH should deploy w/o errors when installing to datastore (1)
5. Vic-machine create command should pass when installing using image store with DS scheme include and space in path