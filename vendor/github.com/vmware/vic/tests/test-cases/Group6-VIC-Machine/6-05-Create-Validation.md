Test 6-05 - Verify vic-machine create validation function
=======

# Purpose:
Verify vic-machine create validation functions, this does not include validation for network, datastore, and compute resources

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available


Test Cases: - suggest resources
======

## Invalid datacenter
1. Prepare vCenter environment with multiple datacenters
2. Create with --target specifying a datacenter that does not exist

### Expected Outcome:
* Output contains message indicating datacenter must be specified
* Output suggests available datacenter values
* Deployment fails

## Invalid target path
1. Prepare vCenter environment
2. Create with --target specifying a datacenter and resource pool

### Expected Outcome:
* Output contains message indicating that onlydatacenter must be specified in --target
* Output suggests available datacenter values
* Deployment fails

## Create VCH - target thumbprint verification
1. Issue the following command:
```
vic-machine-linux create --thumbprint=NOPE --name=${vch-name} \
    --target="%{TEST_USERNAME}:%{TEST_PASSWORD}@%{TEST_URL}" --image-store=ENOENT ${vicmachinetls}
```

### Expected Outcome:
* Output contains message that thumbprint does not match

## Resource pools
1. Create with wrong compute-resource: not exist resource pool, not existed vc cluster, not existed datacenter.
2. Create with wrong compute-resource format

### Expected Outcome:
* Verify resource suggestion successfully show available values
* Deployment fails

## Networks
1. Create with nonexistent bridge network
2. Create with nonexistent public network

### Expected Outcome:
* Verify resource suggestion successfully show available values
* Deployment fails

## Multiple datacenters
1. Prepare vCenter environment with multiple datacenters
2. Create with --target not specifying a datacenter

### Expected Outcome:
* Output contains message indicating datacenter must be specified
* Output suggests available datacenter values
* Deployment fails


Test Cases: - validate license
======
1. Prepare env with different license level
2. Verify license validation works for different license

### Expected Outcome:
* If license verification passed, deployment succeeds


Test Cases: - firewall
======

## Firewall disabled
1. Create with env with firewall disabled

### Expected Outcome:
* Warn firewall is not enabled with user-friendly message
* Deployment succeeds

## Firewall enabled
1. Create with env with firewall enabled, but tether port allowed

### Expected Outcome:
* Show firewall check passed
* Deployment succeeds

## Firewall misconfigured
1. Create env with firewall configured to block tether port

### Expected Outcome:
* Show error message that firewall is misconfigured
* Deployment fails


Test Cases: - drs
======
1. Prepare env with drs disabled
2. Verify deployment failed for drs disabled with user-friendly error message


Test Cases: - resource accessibility
======
1. Prepare env with datastore not connected to hosts
2. Verify deployment failed for host/datastore connectability with user-friendly error message


Test Cases: - networking
======
## vDS contains all hosts in cluster
1. Prepare vCenter environment with a vDS that is connected to all hosts in the cluster
2. Issue the following command:
```
vic-machine create --name=<VCH_NAME> --target=<TEST_URL> \
    --user=<TEST_USERNAME> --image-store=<TEST_DATASTORE> --password=<TEST_PASSWORD> \
    --bridge-network=<NETWORK> --compute-resource=<TEST_RESOURCE>
```
3. Run regression tests

### Expected Outcome:
* Output contains message indicating vDS configuration OK
* Deployment succeeds
* Regression tests pass

## vDS does not contain all hosts in cluster
1. Prepare vCenter environment with a vDS that is not connected to all hosts in the cluster
2. Issue the following command:
```
vic-machine create --name=<VCH_NAME> --target=<TEST_URL> \
    --user=<TEST_USERNAME> --image-store=<TEST_DATASTORE> --password=<TEST_PASSWORD> \
    --bridge-network=<NETWORK> --compute-resource=<TEST_RESOURCE>
```

### Expected Outcome:
* Output contains message indicating vDS configuration is incorrect with user-friendly error message
* Deployment fails

## Bridge network same as public network
1. Create with bridge network the same as public network

### Expected Outcome:
* Output contains message indicating invalid network configuration
* Deployment fails


Test Cases: - storage
======
## Default image datastore
1. Prepare env with one datastore
2. Issue `vic-machine create` without specifying `--image-store`
3. Run regression tests

### Expected Outcome:
* Deployment succeeds
* Regression tests pass

## Custom image datastore
1. Issue the following command:
```
vic-machine-linux create --name=${vch-name} --target=%{TEST_URL} \
    --thumbprint=%{TEST_THUMBPRINT} --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} \
    --image-store=%{TEST_DATASTORE}/long/weird/path ${vicmachinetls}
```
2. Run regression tests

### Expected Outcome:
* Deployment succeeds
* Regression tests pass
