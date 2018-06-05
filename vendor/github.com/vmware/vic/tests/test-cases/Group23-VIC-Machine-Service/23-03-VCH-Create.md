Test 23-03 - VCH Create
=======

# Purpose:
To verify vic-machine-server can create a VCH with a specified configuration

# References:
1. [The design document](../../../doc/design/vic-machine/service.md)

# Environment:
This test requires a vSphere system where VCHs can be deployed

**Note:** Multiple versions of tests must be implemented:
 * With and without datacenter-scoped URLs.
 * Using both username/password- and session-based authentication.

It should not be necessary to implement all four combinations for every test case, but full coverage should be provided for at least one of the basic test cases.


Basic Validation
----------------

These tests are intended to verify the feature at a basic level.

###  1. Create a simple VCH (i.e., one using as many defaults as possible)

###  2. Create minimal VCH within datacenter

###  3. Create a complex VCH (i.e., one which explicitly specifies as many settings as possible)


Negative Cases
--------------

These tests are intended to verify the behavior in various failure cases. We must gracefully handle invalid input and unexpected user behavior.

###  1. Attempt to create a VCH with invalid operations credentials

###  2. Attempt to create a VCH with an invalid datastore

###  3. Attempt to create a VCH with an invalid storage settings

###  4. Attempt to create various VCHs with invalid networking settings (on bridge, public, management, and container networks)

###  5. Attempt to create a VCH with an invalid gateway settings

###  6. Attempt to create a VCH with a name containing invalid characters

###  7. Attempt to create VCH with a very long name (over 31 characters)

###  8. Attempt to create a VCH with a name that is already in use

###  9. Attempt to create a VCH with an invalid container name convention

###  10. Attempt to create a VCH specifying an ID
(Not implemented.)


### 11. Attempt to create various VCHs with invalid resource settings

### 12. Attempt to create various VCHs with invalid registry settings

### 13. Attempt to create various VCHs with invalid security settings


Interoperability
----------------

These tests are intended to verify that the API and CLI can coexist without issue.

###  1. Verify that the CLI can be used to delete a VCH created by the API


Concurrency
-----------

These tests are intended to verify that the API behaves as expected when performing concurrent operations.

###  1. Attempt to create two VCHs at the same time within a single datacenter

(Not implemented.)

###  2. Attempt to create two VCHs at the same time in separate datacenters

(Not implemented.)


Workflow-based
--------------

These tests are designed to mimic realistic customer scenarios. These tests will usually duplicate coverage provided by a test above, but provide additional validation around specific important workflows.

###  1. Create a VCH with an interesting network topology and verify that the isolation properties of the networks are as expected (using a static IP on at least one network)

(Not implemented.)

###  2. Create a VCH with a variety of volume stores and verify that they work as intended, including sharing of volumes

(Not implemented.)

###  3. Create a complex VCH using a new operations user account and the "grant permissions" feature and verify that those permissions are sufficient by exercising a variety of VCH functionality

(Not implemented.)

###  4. Create a VCH with interesting registry settings and a proxy and verify that those are used as expected

(Not implemented.)

###  5. Create a VCH with interesting security settings and verify that those are used as expected

(Not implemented.)
