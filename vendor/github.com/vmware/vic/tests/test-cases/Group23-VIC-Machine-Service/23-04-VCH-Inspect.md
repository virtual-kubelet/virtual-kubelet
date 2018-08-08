Test 23-04 - VCH Inspect
========================

# Purpose:
To verify vic-machine-server can accurately inspect a variety of VCHs

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

###  1. Create a simple VCH and verify that its settings can be accurately inspected

###  2. Create a complex VCH and verify that its settings can be accurately inspected


Negative Cases
--------------

These tests are intended to verify the behavior in various failure cases. We must gracefully handle invalid input and unexpected user behavior.

###  1. Attempt to inspect a VCH in an invalid datacenter

###  2. Attempt to inspect a VCH in with an invalid ID

###  3. Attempt to inspect a VCH which has been deleted

###  4. Attempt to inspect a non-VCH VM by supplying its ID as if it were a VCH


Interoperability
----------------

These tests are intended to verify that the API and CLI can coexist without issue.

###  1. Inspect a VCH created using the CLI


Concurrency
-----------

These tests are intended to verify that the API behaves as expected when performing concurrent operations.

###  1. Inspect a VCH while it is being created

###  2. Inspect a VCH while it is being deleted

###  3. Inspect a VCH while it is being reconfigured

###  4. Inspect a VCH while it is being upgraded

