Test 1-40 - Docker Restart
=======

# Purpose:
To verify that `docker restart` is supported and works as expected.

# Environment:
This test requires that a vSphere server is running and available


# Test Steps:
1. Run a busybox container and create a busybox container
2. Run restart for running container
3. Run restart for created container
4. Run restart for stopped container


# Expected Outcome:
1. Fails if two containers are not created
2. Successfully stop and restart container - failure on op failure or ip address change
3. Successfully start created container - failure if container doesn't start
4. Successfully stop running container and then restart stopped container - failure on op failure or ip address change

# Possible Problems:

