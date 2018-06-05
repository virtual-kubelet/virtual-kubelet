Test 6-04 - Verify vic-machine create basic function
=======

# Purpose:
Verify vic-machine create basic connection variables, certificates, timeout, and all arguments after appliance-iso

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available



DNS Servers
=======

### Create VCH - supply DNS server
1. Create VCH while supplying the `--dns-server` option twice with values `1.1.1.1` and `2.2.2.2`
2. Enable SSH on the VCH using the `vic-machine debug` command
3. SSH into the VCH run `cat /etc/resolv.conf`


### Expected Outcome
* The top two lines of the output from `cat /etc/resolv.conf` should contain `1.1.1.1` and `2.2.2.2` in that order.

Image size
=======

## Create VCH - custom base disk
1. Issue the following command:
```
vic-machine-linux create --name=${vch-name} --target=%{TEST_URL} --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --bridge-network=%{BRIDGE_NETWORK} --public-network=%{PUBLIC_NETWORK} --image-store=%{TEST_DATASTORE} --password=%{TEST_PASSWORD} --base-image-size=6GB ${vicmachinetls}
```

### Expected Outcome
* VCH is deployed successfully
* Container has correct disk size
* Regression tests pass


Folder Structure
=======
## Create VCH - Folder Structure Correctness
This will be a basic test which will confirm that the correct folder gets created for the appliance. Additionally, 
it will confirm that the VCH is created inside of that folder.

### Steps
1. Deploy a standard CI VIC appliance. No special parameters are needed for folder support
2. Confirm that the folder exists with the correct name.
3. Confirm the VCH exists inside of that folder also with the correct name.
4. Delete the VCH(Should also remove all folders, this will be a separate test.)

### Expected Outcome
Step 1 Should succeed without error
Step 2-3 should have rc's of 0 and Should pass their checks successfully.
Step 4 should succeed without error

Connection
=======

## Create VCH - URL without user and password
1. Create with vSphere URL in --target parameter, without --user and --password

### Expected Outcome
* Command should fail for no user password available


## Create VCH - URL without password
1. Create with vSphere URL in --target parameter and --user provided, but without --password

### Expected Outcome
* Command should promote interactive password input


## Create VCH - target URL
1. Create with vSphere URL and user password encoded in the same --target parameter
```
vic-machine-linux create --name=<VCH_NAME> --target="<TEST_USERNAME>:<TEST_PASSWORD>@<TEST_URL>" \
    --image-store=<TEST_DATASTORE>
```
2. Run regression tests

## Create VCH - operations user
1. Create with an operations user (the same as the administrative user used for deployment in this case)
```
vic-machine-linux create --ops-user="<TEST_USERNAME>" --ops-password="<TEST_PASSWORD>"
```
2. Run regression tests

### Expected Outcome
* Deployment succeed
* Regression test pass


## Create VCH - specified datacenter
1. Prepare test env with multiple DC exists
2. Create with vSphere URL with correct DC appended as <ip>/DC1

### Expected Outcome
* Verify deployed successfully
* Verify VCH is in correct DC through govc



vic-machine create Parameters
=======

## Create VCH - defaults
1. Issue the following command:
```
vic-machine create --name=<VCH_NAME> --target=<TEST_URL> \
    --user=<TEST_USERNAME> --image-store=<TEST_DATASTORE> --password=<TEST_PASSWORD> \
    --bridge-network=<NETWORK> --compute-resource=<TEST_RESOURCE>
```
2. Run regression tests

### Expected Outcome
* Deployment succeed
* Regression test pass


## Create VCH - full params
1. Issue the following command:
```
vic-machine-linux create --name=<VCH_NAME> --target=<TEST_URL> \
    --user=<TEST_USERNAME> --image-store=<TEST_DATASTORE> \
    --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso \
    --password=<TEST_PASSWORD> --force=true --bridge-network=network \
    --compute-resource=<TEST_RESOURCE> --timeout <TEST_TIMEOUT> \
    --volume-store=<TEST_DATASTORE>/test:default
```
2. Run regression tests

### Expected Outcome
* Deployment succeed
* Regression test pass

## Create VCH - using environment variables
1. Issue the following command:
```
vic-machine-linux create --name=<VCH_NAME> --image-store=<TEST_DATASTORE> \
    --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso \
    --force=true --bridge-network=network --public-network=%{PUBLIC_NETWORK} \
    --compute-resource=<TEST_RESOURCE> --timeout <TEST_TIMEOUT> \
    --volume-store=<TEST_DATASTORE>/test:default
```
2. Run regression tests

### Expected Outcome
* Deployment succeed
* Regression test pass


## Create VCH - custom image store directory
1. Issue the following command:
```
vic-machine-linux create --name=${vch-name} --target=%{TEST_URL} \
    --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} \
    --image-store %{TEST_DATASTORE}/vic-machine-test-images \
    --appliance-iso=bin/appliance.iso --bootstrap-iso=bin/bootstrap.iso \
    --password=%{TEST_PASSWORD} --force=true --bridge-network=%{BRIDGE_NETWORK} \
    --public-network=%{PUBLIC_NETWORK} --compute-resource=%{TEST_RESOURCE} \
    --timeout %{TEST_TIMEOUT} ${vicmachinetls}
```
2. Run regression tests

### Expected Outcome
* Deployment succeeds
* Regression tests pass


## Create VCH - long VCH name
1. Provide long name to create VCH, e.g. 100 characters

### Expected Outcome
* Command failed for name is too long


## Create VCH - Existing VCH name
1. Create with same name with existing VCH

### Expected Outcome
* Command failed for VCH is found

======
## Create VCH - Existing VM name
1. Create with existing VM name
2. Run regression tests

### Expected Outcome
* Deployment succeeds
* Regression tests pass

## Create VCH - Folder Conflict
This test is designed to confirm that we report an already exist style error if the VCH folder 
already exists in the vm folder.

### Steps
1. Create a folder with the same name as the vch in the vm folder.
2. Attempt to Create a VCH with the name of the dummy vm.
3. Cleanup the Dummy vm and folder

### Expected Outcome
Step 1 Should succeed without error
Step 2 Should fail with the correct error message.
Step 3 should succeed without error

======

## Create VCH - Existing RP on ESX
1. Create resource pool on ESX
2. Create VCH with the same (already existing) name

### Expected Outcome
* Deployment succeeds
* Regression tests pass


Image files
=======

## Create VCH - wrong ISOs
1. Provide wrong iso files

### Expected Outcome
* Command failed for no iso files found


Creation log file
======

## Creation log file uploaded to datastore
1. Issue the following commands:
```
vic-machine create --name=<VCH_NAME> --target=<TEST_URL> \
    --user=<TEST_USERNAME> --image-store=<TEST_DATASTORE> --password=<TEST_PASSWORD> \
    --bridge-network=<NETWORK> --compute-resource=<TEST_RESOURCE>
```
2. Verified that the creation log file prefixed by `vic-machine-create` is uploaded to datastore folder
3. Verified that the creation log file is complete

## Expected Outcome
* Deployment succeeds
* The creation log file is uploaded to datastore folder
* The creation log file is complete



Timeout
=======

## Basic timeout
1. Specify short timeout to 2s

### Expected Outcome
* Command fail for timeout error #1557


Short time creation
===================

# Stop VCH creation immediately
=============================
1. Interrupt creation process after 2s,
2. Delete the VCH

### Expected Outcome
* Delete should succeed


Appliance size
=======

## Basic VCH resource config
1. Specify appliance size to 4cpu, 4096MB

### Expected Outcome
* Deployed successfully
* Appliance VM size is set correctly in vsphere
* Regression test pass


## Invalid VCH resource config
1. Specify appliance size to 1cpu, 256MB

### Expected Outcome
* Deployment failed for no enought resource
* Should have user-friendly error message


## Use resource pool
1. --use-rp=true

### Expected Outcome
* Deployed successfully
* VCH is created under resource pool against VC
* Regression test pass


## CPU reservation shares invalid
1. Specify VCH CPU size to reservation: 4, limit: 8, shares: wrong

### Expected Outcome
* Deployment failed for wrong shares format


## CPU reservation invalid
1. Specify VCH CPU size to reservation: 4, limit: 2, shares: normal

### Expected Outcome
* Deployment failed for user-friendly error message


## CPU reservation valid
1. Specify VCH CPU size to reservation: 4, limit: 8, shares: high

### Expected Outcome
* Deployed successfully
* Check rp resource settings are correct through govc
* Integration test passed


## Memory reservation shares invalid
1. Specify VCH Memory size to reservation: 4096, limit: 8192, shares: wrong

### Expected Outcome
* Deployment failed for wrong shares format


## Memory reservation invalid 1
1. Specify VCH Memory size to reservation: 4096, limit: 2048, shares: normal

### Expected Outcome
* Deployment failed for user-friendly error message


## Memory reservation invalid 2
1. Specify VCH Memory size to reservation: 256, limit: 256, shares: high

### Expected Outcome
* Deployment failed with user-friendly error message


## Memory reservation invalid 3
1. Specify VCH Memory size to reservation: 200, limit: 200, shares: high

### Expected Outcome
* Deployment failed with user-friendly error message


## Memory reservation valid
1. Specify VCH Memory size to reservation: 4096, limit: 8192, shares: high

### Expected Outcome
* Deployed successfully
* Check rp resource settings are correct through govc
* Integration test passed
