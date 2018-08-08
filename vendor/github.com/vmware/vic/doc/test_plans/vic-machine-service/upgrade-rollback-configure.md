Background
==========

The VCH Management API is implemented by `vic-machine-server` and used by the VCH Management UI, including the VCH Creation Wizard. Support for other uses of the API is an eventual goal.

Initial API functionality was implemented for 1.3, including the ability to retrieve the API server version and list, create, inspect, and delete Virtual Host Containers (VCHs).

For 1.4, we hope to add support for upgrade/rollback of a VCH as well as VCH re-configuration. This will allow us to expose this functionality via the VCH Management UI, which will in turn allow users to perform more of their VCH management tasks via the UI instead of requiring use of the CLI.

The API can be used to manage VCHs running directly on the ESX host running the OVA as well as those using Virtual Center (VC), including multi-datacenter VC configurations.

Each API operation is implemented by a handler method. Currently, this results in duplication of code present in the `vic-machine` CLI command. This is undesirable, so this project will include work to establish patterns to avoid such duplication and refactor existing code accordingly.


Testing Scope
=============

The primary focuses of testing will the combinatorial correctness of the re-configuration operation (including appropriate error handling for properties which may not be modified) and the end-to-end behavior of API workflows in a variety of realistic scenarios.

As the API handler methods are invoked by go-swagger, testing of call routing and similar logic is out of scope.

In addition to the test discussed below, careful attention should be paid to existing code that is modified as a part of the refactoring effort mentioned above. In some cases, testing coverage for that existing code may be insufficient.


Testing Strategy
================

Unit tests
----------

Unit testing is the natural choice for verifying the behavior of the re-configuration logic; there are many potential combinations of properties that may or may not be changed as a part of a given operation and a full end-to-end test of each such combination would take prohibitively long. These tests should consider common error cases, such as ensuring proper handling of non-existent VCHs, VCHs in an invalid state, changes to properties which may not be modified, and "no-op" changes (which must be mindful of the request method used: PUT and PATCH must treat "no-op" changes to properties which may not be modified differently).

Unit testing is also a natural choice for verifying any changes we make to the way that VCHs are "locked" during the upgrade process. (See https://github.com/vmware/vic/issues/6899, https://github.com/vmware/vic/issues/7083)

Care must be taken when structuring the API handler logic to ensure that this functionality can be exercised without onerous mocking.


Robot tests
-----------

Robot-based tests will be used to verify end-to-end functionality.

These tests should verify functional correctness of each new API endpoint, and should as a whole exercise each parameter/feature of the endpoint, but need not execute all potential combinations of parameters/features (our end-to-end tests can focus on the most common combinations).

Special attention should be paid to the interaction between combinations of operations initiated via both the CLI and API (using the API to re-configure a VCH created via the CLI, using the CLI to rollback an upgrade attempted via the API, attempts to upgrade via the CLI and API concurrently, etc.).

Attention should also be paid to concurrent attempts at operations, using both the PUT and PATCH codepaths. It will be necessary to describe the desired behavior (should concurrent disjoint PATCH operations succeed?) and produce a matrix of cases to test.

Additionally, attention should be paid to ways in which the CLI and API are different. For example, upgrade is a potentially long-running operation which may complete asynchronously (w.r.t. the API call that initiated it), which is a different model than the CLI (where the entire operation is synchronous). This might suggest a need for tests which verify the behavior when the API server terminates while performing an asynchronous operation (i.e., an operation associated with an HTTP request for which a response has already been sent).

Lastly, attention should be paid to cases in which old and new code must interact. For example, rollback of a failed upgrade may involve executing code shipped with previous releases. Multiple versions of this code may exist. Because we can modify only the new version of the code, the behavior of existing versions "in the wild" may limit our design and/or implementation choices, and so it is especially important to identify these issues early in the release. 

These tests should verify both username/password- and session-based codepaths (issues were seen late in 1.3 that were a result of code that worked via one codepath, but not the other).

Tests should be written that exercise the API directly against ESX, against VC, against VC using a datacenter, and against VC using a datacenter in a multi-datacenter environment (issues were seen late in 1.3 that were a result of code that worked in one or more of these environments, but not others). Tests against VC should include testing of VC in an enhanched linked mode (ELM) configuration. It may not be possible, or reasonable, to test all of these environments in CI; automation as a part of nightly tests or manual invocation against another environment may be viable.


Key Scenarios
=============

In addition to the comprehensive granular testing described above, there are some key customer scenarios we should be sure to test.

Successful VCH Upgrade
----------------------

Environment: multi-datacenter VC

1. Using 1.3.1, create a VCH
2. Using that VCH, deploy containers running webservers serving a page from a volume:
   - Container A serves the content via NAT
   - Container B serves the content via a container network
   - Container C, separately, serves content to A and B via a bridge network
3. Upgrade the VCH using the VCH Management API
4. Verify that the upgrade completed without error
5. Verify that each webserver has been and is responding to requests in the expected way:
   - Connections to A may drop during the upgrade, but should be restored
   - Connections to B may not drop
   - Connections from A and B to C may not drop
6. Verify that the upgraded VCH can be used to create another container


Rollback of a failed VCH upgrade
--------------------------------

Environment: multi-datacenter VC

1. Using 1.3.1, create a VCH
2. Using that VCH, deploy a container running a webserver serving a page from a volume
3. Upgrade the VCH using the VCH Management API
4. While the upgrade is still running, kill the API server (or otherwise force the upgrade to fail)
5. Verify that the upgrade has failed
6. Verify that the VCH can be rolled back to 1.3.1
7. Verify that the webserver is still responding to requests in the expected way
8. Verify that the VCH can be used to create another container
9. Re-attempt the upgrade using the VCH Management API
10. Verify that the upgrade succeeds, as in the above scenario


Update the certificates used by an existing VCH
-----------------------------------------------

Environment: multi-datacenter VC

1. Generate two client certificates (C and C') and two server certificates (S and S')
2. Create a VCH using the VCH Management API with certificates C and S 
3. Verify the configuration by:
    a) successfully connecting to the VCH via Docker using the --tlsverify flag with C and S
    b) failing to connect to the VCH via Docker using the --tlsverify flag with C' and S
    c) failing to connect to the VCH via Docker using the --tlsverify flag with C and S'
    d) failing to connect to the VCH via Docker using the --tlsverify flag with C' and S'
4. Re-configure the VCH to add C'
5. Verify the configuration as in (3), except expecting both (a) and (b) to succeed
6. Re-configure the VCH to remove C
7. Verify the configuration as in (3), except expecting only (b) to succeed
8. Re-configure the VCH to use S' instead of S
9. Verify the configuration as in (3), except expecting only (d) to succeed
