# VIC Engine Integration & Functional Test Suite

To run the integration tests locally:

## Automatic with defaults

Use ./local-integration-test.sh

## Manually configure local Drone

* Create a `test.secrets` file containing secrets in KEY=VALUE format which includes:

  ```
    GITHUB_AUTOMATION_API_KEY=<token from https://github.com/settings/tokens>
    TEST_BUILD_IMAGE=""
    TEST_URL_ARRAY=<IP address of your test server>
    TEST_USERNAME=<username you use to login to test server>
    TEST_PASSWORD=<password you use to login to test server>
    TEST_RESOURCE=<resource pool, e.g. /ha-datacenter/host/localhost.localdomain/Resources>
    TEST_DATASTORE=<datastore name, e.g. datastore1>
    TEST_TIMEOUT=60s
    VIC_ESX_TEST_DATASTORE=<datastore path, e.g. /ha-datacenter/datastore/datastore1>
    VIC_ESX_TEST_URL=<user:password@IP address of your test server>
    DOMAIN=<domain for TLS cert generation, may be blank>
  ```

  If you are using a vSAN environment or non-default ESX install, then you can also specify the two networks to use with the following command (make sure to add them to the yaml file in Step 2 below as well):

  ```
    BRIDGE_NETWORK=bridge
    PUBLIC_NETWORK=public
  ```

  If you want to use an existing VCH to run a test (e.g. any of the group 1 tests) on, add the following secret to the secrets file:

  ```
    TARGET_VCH=<name of an existing VCH>
  ```

  The above TARGET_VCH is best used for tests where you do not want to exercise vic-machine's create/delete operations.  The Group 1 tests is a great example.  Their main goal is to test docker commands.

  If TARGET_VCH is not specified, and you have a group initializer and cleanup file (see the group 1 tests), there is another variable to control whether use a shared VCH.

  ```
    MULTI_VCH=<1 for enable>
  ```

  Enabling MULTI_VCH forces each suite to install a new VCH and cleans it up at the end of the test.  If the test is in 'single vch' mode, it will respect the group initializer and cleanup file.  If the initializer creates the shared VCH, then all tests will use that shared VCH.  If TARGET_VCH exist, MULTI_VCH is ignored.

  ```
    DEBUG_VCH=<1 to enable>
  ```

  Enabling DEBUG_VCH will log existing docker images and containers on a VCH at the start of a test suite.


* Execute Drone from the project root directory:

  Drone will run based on `.drone.local.yml` - defaults should be fine, edit as needed. Set secrets as env variables:

  *  To run only the regression tests:
     ```
     drone exec .drone.local.yml
     ```

  * To run the full suite:
     ```
		 drone exec --repo-name "vmware/vic" .drone.local.yml
     ```

## Test a specific .robot file

* Set environment in robot.sh
* Run robot.sh with the desired .robot file

  From the project root directory:
  ```
  ./tests/robot.sh tests/test-cases/Group6-VIC-Machine/6-04-Create-Basic.robot
  ```

## Run Docker command tests via makefile target

There exists a makefile target for developers to run the docker command tests locally (not in CI environment) against a pre-deployed VCH. This is a fast way for contributors to test their potential code chages, against the CI tests locally, before pushing the commit. There is another benefit gained from using the makefile target, the way it is setup, logs from the run are written out to vic/ci-results, even if the tests fail. The method described above, to run the tests locally with drone, has the weakness that a failure in the test can sometimes result in no written logs to help debug the failure.

There are a few requirements before using this makefile target.

1. A VCH must be pre-deployed before calling this makefile target
2. The makefile target relies on a script that looks for a few more secrets variable.  When running the script directly, these secrets variables may be passed into the script via commandline arguments, environment variables, or via a secrets file.  When running the makefile target via make, the secrets must be defined in environment variables.

To run these tests using the makefile target,

```
make local-ci-test

SECRETS_FILE=test.secrets.esx make local-ci-test

DOCKER_TEST=Group1-Docker-Commands/1-01-Docker-Info.robot make local-ci-test
```
In the above example, the first command assumes all environment variables are defined.  The second command defines one environment variable, SECRETS_FILE, before calling the make target.  This allows calling the make target with all the necessary secrets variable defined in the secrets file instead of in environment variables.  The third command defines a specific test to run using the environment variable, DOCKER_TEST.

Currently, only the Group1 tests are setup to use an existing VCH so this makefile target only works on the group 1 tests.

It is also possible to run the docker command tests, without using make, by calling the internal script itself.  The script is located at "infra/scripts/local-ci.sh".  As stated above, the scripts also allows command line arguments to be passed directly into the script.

A helpful tip is to create different secrets files for different environments.  For instance, test.secrets.esx and test.secrets.vc for an ESX host and VC cluster, respectively.


## Find the documentation for each of the tests here:

* [Automated Test Suite Documentation](test-cases/TestGroups.md)
* [Manual Test Suite Documentation](manual-test-cases/TestGroups.md)

## Tips on running tests more efficiently

Here are some recommendations that will make running tests more effective.

1. If a group of tests do not need an independent VCH to run on, there is a facility to use a single VCH for the entire group.  The Group 1 tests utilizes this facility.  To utilize this in a group (a folder of robot files),
    - Add an __init__.robot file as the first robot file in your group.  This special init file should install the VCH and save the VCH-NAME to environment variable REUSE-VCH.  The bootrap file also needs to save the VCH to the removal exception list.
    - Every robot file should neither assume a group-wide VCH.  It should install and remove a VCH for it's own use.  This allows the single robot file to be properly targeted for testing as a single test or as part of a group of test (with group-wide VCH).  When a group wide VCH is in use, the exception list will bypass the per-robot file VCH install and removal.
    - Write individual tests within a robot file with NO assumption of a standalone VCH.  Assume a shared VCH.  This will allow the
    tests to run in either shared VCH or standalone VCH mode.
    - Add a cleanup.robot file that handles cleaning up the group-wide VCH.  It needs to remove the group-wide VCH-NAME from the cleanup exception list.
2. Write all tests within robot file with the assumption that the VCH is in shared mode.  Don't assume there are no previously created containers and images.  If a robot file needs this precondition, make sure the suite setup cleans out the VCH before running any test.
3. If there is an existing VCH available, it is possible to bypass the VCH installation/deletion by adding a TARGET_VCH into the list of test secrets.