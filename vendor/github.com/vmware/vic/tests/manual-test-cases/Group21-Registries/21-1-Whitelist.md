Test 21-01 - Whitelist Registries
=======

# Purpose:
To verify that VIC appliance can whitelist registries

# Environment:
This test requires that a vSphere server is running and available

Test Case -- Basic whitelisting
=========

##Test Steps:
1. Use ovftool to deploy two harbor server, one using HTTP, the other HTTPS
2. Deploy VIC appliance to vSphere server with vic-machine and --whitelist-registry and --registry-ca options
3. Issue docker info
4. Issue docker login against the whitelist registry
5. Issue docker pull against the whitelist registry
6. Issue docker login against the whitelist registry:443
7. Issue docker pull against the whitelist registry:443
8. Issue docker login against docker.io
9. Issue docker pull against docker.io
10. Tear down VCH

##Expected Outcome:
* Step 2 has no Warning for registry and whitelist registries are listed and were confirmed
* Step 3 have docker hub for registry and whitelist registries listed
* Step 4-5 succeeds
* Step 6-7 succeeds.  VCH should not care if port is added at the end of docker operation or vic-machine creation
* Step 8-9 fails and return a message containing 'Access denied to unauthorized registry'

Test Case -- Insecure Registry Login With HTTP
=========
1. Install VCH w/o specifying the insecure registry
2. Try to log in / pull with docker -- expect failure 
3. Destroy the VCH
4. Install a new VCH and specify insecure registry w/ insecure HTTP
5. Try to log in / pull with docker -- expect success 
6. Destroy the VCH

Test Case -- Configure Registry CA
=========
This test ensures that we can change the registry CA cert installed on the VCH
1. Install a new VCH without specifying a registry CA
2. Try to log in (should fail)
3. Use vic-machine configure --registry-ca to add the CA to the appliance
4. Try to log in & pull (should succeed)
5. Run vic-machine configure without --registry-ca to check that no change occurs in this case
6. Try to log in and pull (should succeed)
5. Tear down VCH


Test Case -- Basic whitelisting with NO certs
=========
1. Deploy VIC appliance to vSphere server with vic-machine and --whitelist-registry and NO --registry-ca options
2. Issue docker login against the whitelist registry
3. Issue docker pull against the whitelist registry
4. Tear down VCH

##Test Steps:
* Step 1 has a warning the registry cannot be confirmed
* Step 2-3 fails

##Possible Problems:
None


Test Case -- Whitelist + HTTP Insecure-registry
=========

##Test Steps:
1. Deploy VIC appliance to vSphere server with vic-machine and --registry-ca, --whitelist-registry and --insecure-registry options with NON-overlapping whitelist and insecure servers and one fake registry
2. Issue docker info
3. Issue docker login against the whitelist registry
4. Issue docker pull against the whitelist registry
5. Issue docker login against the insecure registry
6. Issue docker pull against the insecure registry
7. Issue docker login against docker.io
8. Issue docker pull against docker.io
9. Tear down VCH

##Expected Outcome:
* Step 1 has no warnings for registry, whitelist registries are listed and includes all whitelist and insecure servers
* Step 2 have docker hub, whitelist registries and have insecure registries listed
* Step 3-6 succeeds
* Step 7-8 fails and return a message containing 'Access denied to unauthorized registry'

##Possible Problems:
None


Test Case -- Whitelist + overlapping HTTP Insecure-registry with certs
=========

1. Deploy VIC appliance to vSphere server with vic-machine and --registry-ca, --whitelist-registry and --insecure-registry options with the same servers for both
2. Issue docker info
3. Issue docker login against the registry
4. Issue docker pull against a public library on the registry
5. Tear down VCH

##Expected Outcome:
* Step 1 has no warnings for registry, whitelist registries are listed, and the registry was confirmed as insecure
* Step 2 have docker hub, whitelist registries and have insecure registries listed
* Step 3-4 succeeds

##Possible Problems:
None


Test Case -- Whitelist + overlapping HTTP Insecure-registry with NO certs
=========

1. Deploy VIC appliance to vSphere server with vic-machine and --whitelist-registry and --insecure-registry options with the same servers for both
2. Issue docker login against the overlapping registry
3. Issue docker pull against a public library on the registry
4. Tear down VCH

##Expected Outcome:
* Step 1 has no warnings for registry, whitelist registries are listed, and the registry was confirmed as insecure
* Step 2-3 succeeds as insecure-registry modifies the whitelist-registry server

##Possible Problems:
None


Test Case -- Whitelist registry in CIDR format
=========

1. Deploy VIC appliance to vSphere server with vic-machine and --registry-ca, --whitelist-registry
2. Issue docker info
3. Issue docker login against the registry with IP address
4. Issue docker pull against a public library on the registry with IP address
5. Tear down VCH

##Expected Outcome:
* Step 1 have no warnings for registry, whitelist registries are listed, and a message that the registry confirmation was skipped
* Step 2 have docker hub and whitelist registries listed
* Step 3-4 succeeds

##Possible Problems:
None


Test Case -- Whitelist registry in wildcard domain format
=========

1. Deploy VIC appliance to vSphere server with vic-machine and --registry-ca, --whitelist-registry
2. Issue docker info
3. Issue docker login against the registry with IP address
4. Issue docker pull against a public library on the registry with IP address
5. Tear down VCH

##Expected Outcome:
* Step 1 have no warnings for registry, whitelist registries are listed, and a message that the registry confirmation was skipped
* Step 2 have docker hub and whitelist registries listed
* Step 3-4 succeeds

##Possible Problems:
None
