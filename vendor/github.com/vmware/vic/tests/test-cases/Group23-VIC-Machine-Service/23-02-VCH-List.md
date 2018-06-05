Test 23-02 - VCH List
=======

# Purpose:
To verify vic-machine-server can return a list of VCHs including the expected information

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

###  1. Without creating any VCHs, verify that an empty list is properly returned

(Not yet implemented)

###  2. Create a VCH and verify it is returned in the list, with correct information

###  3. Create two VCHs, power one of them off, and verify both are returned in the list, with correct information for each

(Not yet implemented)

###  4. Create 64 VCHs and verify all are returned in the list, with correct information for each

(Not yet implemented)

**Note:** This case may be better implemented as a nightly test, or similar; it is too onerous to run as a part of CI.


Negative Cases
--------------

These tests are intended to verify the behavior in various failure cases. We must gracefully handle invalid input and unexpected user behavior.

###  1. Attempt to list the VCHs in an invalid datacenter

###  2. Attempt to list the VCHs in an invalid compute resource

###  3. Attempt to list the VCHs in an invalid compute resource and datacenter

###  4. Ensure that a VCH in one datacenter is not returned when listing the VCHs in another datacenter

(Not yet implemented)


Interoperability
----------------

These tests are intended to verify that the API and CLI can coexist without issue.

###  1. Create a VCH using the CLI and verify that it is correctly included when listing via the API

###  2. Create a VCH using an old version of the CLI and verify that it is correctly included when listing via the API


Concurrency
-----------

These tests are intended to verify that the API behaves as expected when performing concurrent operations.

###  1. List VCHs while one is being created

(Not yet implemented)

###  2. List VCHs while one is being deleted

(Not yet implemented)

###  3. List VCHs while one is being updated in a way that would affect its list-level representation

(Not yet implemented)
