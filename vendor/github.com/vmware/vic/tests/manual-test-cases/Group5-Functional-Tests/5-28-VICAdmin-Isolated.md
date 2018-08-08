Test 5-28 - VIC Admin Isolated
=======

# Purpose:
Verify that VIC Admin can display logs on an isolated network with no wan connection.

# Environment:
This test requires that a vSphere environment be running and available, which dSwitch test-ds available.

# Test Steps:

1. Create a DVS Port Group that does not have internet connectivity by setting a random vlan id
2. Deploy VIC appliance to the vSphere server using the no-wan port group
3. Verify login functions properly
4. Verify vic admin internet connectivity status is showing a warning icon
5. Pull the VCH-Init log and verify that it contains valid data
6. Pull the Docker Personality log and verify that it contains valid data
7. Create a container via the appliance
8. Pull the container log bundle from the appliance and verify that it contains the new container's logs

# Test Cases:

## Display HTML
1. Log in
2. Page displays vic-machine name in title

## WAN Status Should Fail
1. Log in
2. Page displays warning symbols for wan connection status

## Fail To Pull Docker Image
1. Log in
2. Pull docker busybox
3. Fail to pull busybox

## Get Portlayer Log
1. Log in
2. Portlayer Log access is allowed and logs are downloaded

## Get VCH-Init Log
1. Log in
2. VCH-Init Log access is allowed and logs are downloaded

## Get Docker Personality Log
1. Log in
2. Docker Personality Log access is allowed and logs are downloaded

## Get VICAdmin Log
1. Log in
2. VICAdmin Log access is allowed and logs are downloaded