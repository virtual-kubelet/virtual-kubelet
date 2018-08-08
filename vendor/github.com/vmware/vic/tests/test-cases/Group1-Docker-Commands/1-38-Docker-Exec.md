Test 1-38 - Docker Exec
=======

# Purpose:
To verify that docker exec command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/exec/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker run -d busybox /bin/top
3. Issue docker exec <containerID> /bin/echo ID - 5 times with incrementing ID
4. Issue docker exec -i <containerID> /bin/echo ID - 5 times with incrementing ID
5. Issue docker exec -t <containerID> /bin/echo ID - 5 times with incrementing ID
6. Issue docker exec -it <containerID> /bin/echo ID - 5 times with incrementing ID
7. Issue docker exec -it <containerID> NON_EXISTING_COMMAND

# Expected Outcome:
* Step 2-6 should echo the ID given
* Step 7 should return an error

# Possible Problems:
None

# Concurrent Simple Exec
## Purpose:
This test is designed to prove that we maintain the ability to complete 5 exec operations on a container in under 30 seconds. We should strive to improve upon this further. But for the moment this test is the gate guardian for 5 concurrent simple execs against a running container. 

## Test Steps
1. Pull an image that contains `sleep` and `/bin/ls`. Busybox suffices here.
2. Create a container running `sleep` to simulate a bounded time process.
3. Run the container and detach from it.
4. Start 5 simple exec operations(/bin/ls) in parallel against the detached container.
5. Wait for each exec to finish and check the rc and output for correctness
6. Wait for the container to finish the sleep and exit.
7. Check container run for correctness

## Expected Outcome
* step 1 Should successfully complete with an rc of 0.
* step 2 Should successfully complete with an rc of 0.
* step 3 Should successfully launch the container and detach from it. The container should remain running for 5 seconds only.
* step 4 All 5 execs should be started successfully, we expect all to succeed.
* step 5 All execs should have an rc of 0 and the correct root directories of the stashed busybox file system.
* step 6 The container should exit.
* step 7 The container should have an exit code of 0.

# Exec Power Off test for long running Process
## Purpose:
This test is designed to test running exec's against a container running a process which would not naturally exit(long running). Then after running many execs concurrently we explicitly stop the container. We should then handle the execs in their varying states properly.

## Test Steps
1. Pull an image that contains `/bin/top` and `/bin/ls`. Busybox suffices here.
2. Create a container running `/bin/top` to simulate a long running process that should not exit on it's own.
3. Run the container and detach from it.
4. Start 10 simple exec operations in parallel against the detached container.
5. Explicitly stop the container while the execs are still running in order to trigger the exec power off errors.
6. collect all output from the parallel exec operations.

## Expected Outcome
* step 1 should successfully complete with an rc of 0
* step 2 should successfully complete with an rc of 0
* step 3 should successfully launch the container and detach from it. The container should remain running until explicitly stopped.
* step 4 all 10 execs should be started successfully, we expect most if not all to fail.
* step 5 container should halt successfully.
* step 6 should contain the error message for exec operations that are interrupted by a power off operation. Specifically a poweroff that was explicitly triggered.
* step 6 should also contain atleast one successful exec if not more(we are not counting).

# Exec Power Off test for short Running Process
## Purpose:
This test is designed to start a container and then initiate a long running exec operation(/bin/top). The expectation is that the exec will succeed in execution and run for the length of the containers life. Once the container exits the exec operation should also exit cleanly without error.

## Test Steps
1. Pull an image that contains `sleep` and `/bin/top`. Busybox suffices here.
2. Create a container running `sleep` to simulate a short running process.
3. Run the container and detach from it.
4. Start a simple exec operations running `/bin/top` in parallel against the detached container.

## Expected Outcome
* step 1 should successfully complete with an rc of 0
* step 2 should successfully complete with an rc of 0
* step 3 should successfully launch the container and detach from it. The container should remain running for 20 seconds and should return with an RC of 0.
* step 4 the exec of `/bin/top` should execute successfully and stay running for the entire life of the container.
* step 5 container should halt successfully.
