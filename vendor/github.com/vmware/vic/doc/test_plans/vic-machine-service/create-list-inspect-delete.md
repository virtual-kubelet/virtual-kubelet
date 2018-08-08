Background
==========

The VCH Management API is implemented by `vic-machine-server` and used by the VCH Management UI, including the VCH Creation Wizard. Support for other uses of the API is an eventual goal.

Initial API functionality was implemented for 1.3, including the ability to retrieve the API server version and list, create, inspect, and delete Virtual Host Containers (VCHs).

The list and inspect are not yet used by the VCH Management UI, but will be in the future.

The API can be used to manage VCHs running directly on the ESX host running the OVA as well as those using Virtual Center (VC), including multi-datacenter VC configurations.


Testing Scope
=============

The focus of testing will be the functional correctness of each of the implemented VCH operations (list, create, inspect, and delete) as well as the end-to-end behavior of API workflows in a variety of realistic scenarios.

Additionally, some validation of basic "framework" functionality may be desired. We should ensure that the API server version is properly returned, that response headers (e.g., for CORS) are as expected, etc.

As the API handler methods are invoked by go-swagger, testing of call routing and similar logic is out of scope.


Testing Strategy
================

Unit tests
----------

Unit testing many of the VCH operations, as implemented, is challenging. The handler logic is not structured in a way that separates "interesting" functionality from "boilerplate", and much of the handler logic is simply about transforming data from one representation to another. Additionally, the API handler logic can not easily be exercised without onerous mocking.


Robot tests
-----------

Robot-based tests will be used to verify end-to-end functionality.

These tests should verify functional correctness of each new API endpoint, and should as a whole exercise each parameter/feature of the endpoint, but need not execute all potential combinations of parameters/features (our end-to-end tests can focus on the most common combinations).

Special attention should be paid to the interaction between combinations of operations initiated via both the CLI and API (using the API to re-configure a VCH created via the CLI, using the CLI to rollback an upgrade attempted via the API, attempts to upgrade via the CLI and API concurrently, etc.).

Attention should also be paid to concurrent attempts at operations (issues were see late in 1.3 when listing a set of VCHs while one of those is being deleted).

These tests should verify both username/password- and session-based codepaths (issues were seen late in 1.3 that were a result of code that worked via one codepath, but not the other).

Tests should be written that exercise the API directly against ESX, against VC, against VC using a datacenter, and against VC using a datacenter in a multi-datacenter environment (issues were seen late in 1.3 that were a result of code that worked in one or more of these environments, but not others). Tests against VC should include testing of VC in an enhanched linked mode (ELM) configuration. It may not be possible, or reasonable, to test all of these environments in CI; automation as a part of nightly tests or manual invocation against another environment may be viable.

