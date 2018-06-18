Test 1-39 - Docker Stats
=======

# Purpose:
To verify that `docker stats` is supported and works as expected.

# Environment:
This test requires that a vSphere server is running and available


# Test Steps:
1. Run a busybox container and create a busybox container
2. Run Stats no-stream for running container
3. Run Stats with no-stream all which will return stats for running and stopped containers
4. Verify the API memory output against govc
5. Verify the API CPU output
6. Run Stats with no-stream for a non-existent container
7. Run Stats with no-stream for a stopped container
8. Verify basic API network and disk output


# Expected Outcome:
1. Fails if two containers are not created
2. Return stats for a running container and validate memory -- will fail if there is a variation
   of greater than 5%
3. Return stats for all containers -- will fail if output is missing either container
4. Compare API results vs. govc result for memory accuracy -- will fail if variation greater than 1000 bytes
5. Verify that CPU fields are present - fails if missing
6. Failure with error message
7. Output should include the stopped container short id
8. Fails if either the default network or disk are missing


# Possible Problems:
Stats are created by the ESXi host every 20s -- if there are long pauses between calls
in a single test the results could be incorrect and a failure could occur.
