Test 23-09 - VCH Upgrade
========================

#### Purpose:
To verify vic-machine-server correctly upgrades and rolls back VCHs

#### References:
1. [VIC Machine Service API Design Doc](../../../doc/design/vic-machine/service.md)

#### Environment:
This suite requires a vSphere system where VCHs can be deployed. Ideally, this suite would be executed against multiple such environments, including directly against ESX, against a VC, and against a VC with multiple datacenters configured.

**Note:** Multiple versions of tests must be implemented:
 * With and without datacenter-scoped URLs.
 * Using both username/password- and session-based authentication.

It should not be necessary to implement all four combinations for every test case, but full coverage should be provided for at least basic test cases.


Basic Validation
----------------

These tests are intended to verify the feature at a basic level.

###  1. Successfully upgrade a 1.3.0 VCH and then deploy a container

###  2. Successfully upgrade a 1.3.1 VCH and then deploy a container

###  3. Successfully roll back following a failed upgrade of a 1.3.0 VCH


Negative Cases
--------------

These tests are intended to verify the behavior in various failure cases. We must gracefully handle invalid input and unexpected user behavior.

###  1. Attempt to reconfigure a non-existent VCH

###  2. Attempt to reconfigure a VCH which has been deleted

###  3. Attempt to reconfigure a VCH while it is already being upgraded via the API

###  4. Attempt to rollback a VCH which has not begun to be upgraded


Interoperability
----------------

These tests are intended to verify that the API and CLI can coexist without issue.

###  1. Use the API to roll back an upgrade which was initiated via the CLI

###  2. Use the CLI to roll back an upgrade which was initiated via the API

###  3. Attempt to reconfigure a VCH while it is already being upgraded via the CLI


Concurrency
-----------

These tests are intended to verify that the API behaves as expected when performing concurrent operations.

###  1. Attempt to list a VCH while it is being upgraded

###  2. Attempt to inspect a VCH while it is being upgraded

###  3. Attempt to list a VCH while it is being rolled back

###  4. Attempt to inspect a VCH while it is being rolled back


Workflow-based
--------------

These tests are designed to mimic realistic customer scenarios. These tests will usually duplicate coverage provided by a test above, but provide additional validation around specific important workflows.

###  1. Create a representative VCH, deploy a container, attempt an upgrade, roll back, and then successfully upgrade (verifying that the container's workload is operational throughout)


