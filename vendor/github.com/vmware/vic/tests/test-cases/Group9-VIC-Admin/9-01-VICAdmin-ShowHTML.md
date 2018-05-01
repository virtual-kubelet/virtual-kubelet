Test 9-01 - VIC Admin ShowHTML
=======

# Purpose:
To verify that the VIC Administration appliance can display HTML

# Environment:
This test requires that a vSphere environment be running and available

# Test Steps:
1. Deploy VIC appliance to the vSphere server
2. Pull the VICadmin web page and verify that it contains valid HTML
3. Pull the Portlayer log file and verify that it contains valid data
4. Pull the VCH-Init log and verify that it contains valid data
5. Pull the Docker Personality log and verify that it contains valid data
6. Create a container via the appliance
7. Pull the container log bundle from the appliance and verify that it contains the new container's logs

# Expected Outcomes:
* VICadmin should display a web page that at a minimum includes <title>VIC Admin</title>
* VICadmin responds with a log file indicating that the portlayer sever has started
* VICadmin responds with a log file indicating VCH init has begun reaping processes
* VICadmin responds with log file indicating docker personality service has started
* VICadmin responds with a ZIP file containing at a minimum the vmware.log file from the new container

# Unauthenticated Test Cases

## Get Login Page
1. Access the authentication web page

## While Logged Out Fail To Display HTML
1. Required authentication on restricted pages
2. Page requests authentication

## While Logged Out Fail To Get Portlayer Log
1. Portlayer logs are restricted to authenticated users
2. Page requests authentication

## While Logged Out Fail To Get VCH-Init Log
1. VCH_Init logs are restricted to authenticated users
2. Page requests authentication

## While Logged Out Fail To Get Docker Personality Log
1. Personality logs are restricted to authenticated users
2. Page requests authentication

## While Logged Out Fail To Get Container Logs
1. Container logs are restricted to authenticated users
2. Page requests authentication

## While Logged Out Fail To Get VICAdmin Log
1. VICAdmin logs are restricted to authenticated users
2. Page requests authentication

# Authenticated Test Cases

## Display HTML
1. Log in
2. Page displays vic-machine name in title

## Get Portlayer Log
1. Log in
2. Portlayer Log access is allowed and logs are downloaded

## Get VCH-Init Log
1. Log in
2. VCH-Init Log access is allowed and logs are downloaded

## Get Docker Personality Log
1. Log in
2. Docker Personality Log access is allowed and logs are downloaded

## Get Container Logs
1. Log in
2. Container Log access is allowed and logs are downloaded

## Get VICAdmin Log
1. Log in
2. VICAdmin Log access is allowed and logs are downloaded

## Check that VIC logs do not contain sensitive data
1. Log in
2. Fetch all logs in /logs/ path
3. None of the downloaded logs contain the vch or vSphere password

## Wan Routes Through Proxy
1. Start a vch with a proxy defined at a localhost port. The proxy isn't actually running, so all requests will fail. IF the wan requests fail we know they were routed through the proxy.
2. Log in to VICAdmin
3. Verify wan health check is not successful.
