Test 1-37 - Docker USER
=======

# Purpose:
To ensure that `docker run -u` and `USER` inside of a Dockerfile are respected on VIC.

# References:
[1 - Dockerfile Reference -- USER directive]( https://docs.docker.com/engine/reference/builder/#user )
[2 - Docker command line reference (run options; look for --user)](https://docs.docker.com/engine/reference/commandline/run/#options)

# Environment:
This test requires that a vSphere server is running and available

It also expects 3 Docker images exist, built from the Dockerfiles in vic/tests/resources/dockerfiles and published to a Docker image repository reachable by the machine running tests.

# Test Steps:
1. Run a container that was built with a `USER` directive using a user created with `RUN adduser`
2. Run a container that specifies the user should have UID 2000 and doesn't specify GID
3. Run a container that does not specify a user and set it manually with `docker run -u`
4. Run a container that specifies UID 2000 and GID 2000
5. Run a container that does not specify a user or group but set them manually with `docker run -u`
6. Try to run a container with `-u` specifying a nonexistent user
7. Try to run a container with `-u` specifying a nonexistent group
8. Run a container specifying `-u 0:0`

# Expected Outcome:
1-5 should run successfully with the options specified taking effect inside the container and reflected via `id` or `whoami` output
6 & 7 will fail with exit code 125 and an error message
8 will run successfully and `whoami` will report the user `root`



# Possible Problems:
Docker image repository downtime or unreachability will cause failure on the tests that pull images from Docker Hub
