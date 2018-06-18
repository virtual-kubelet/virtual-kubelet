Test 1-41 - Docker Commit
=======

# Purpose:
To verify that `docker commit` is supported and works as expected.

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/commit/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Create a new debian container
2. Install nano into the container
3. Stop the container and commit the changes to the image
4. Start a new container based on the new image, checking that nano is installed already
5. Start another container then stop the container and commit an environment variable to the image using `docker commit -c ENV TEST 'test string'`
6. Start a new container based on the new image, checking that the environment variable is set
7. Attempt to commit an unsupported command into the image like `RUN`
8. Stop the container and commit another environment variable into the image with the author set and commit message set
9. Attempt to commit to a container that doesn't exist

# Expected Outcome:
* Steps 1-3 should succeed
* Step 4 nano should now be found in the image
* Step 5-6 TEST should now be an environment variable in the container
* Step 7 should return an error:
`Error response from daemon: run is not a valid change command`
* Step 8 should be able to be verified that the author and message are set when you inspect the image afterwards
* Step 9 should return error and indicate the container does not exist

# Possible Problems:
* This test relies on our implementation of docker exec in order to work, if there is a problem in exec then these test results will likely not be valid