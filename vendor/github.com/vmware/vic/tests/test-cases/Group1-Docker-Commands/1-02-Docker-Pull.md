Test 1-02 - Docker Pull
=======

# Purpose:
To verify that docker pull command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/pull/)

# Environment:
This test requires that an vSphere server is running and available.

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue a docker pull command to the new VIC appliance for each of the top 3 most popular images in hub.docker.com
    * nginx, busybox, ubuntu
3. Issue a docker pull command to the new VIC appliance using a tag that isn't the default latest
    * ubuntu:14.04
4. Issue a docker pull command to the new VIC appliance using a digest
    * nginx@sha256:7281cf7c854b0dfc7c68a6a4de9a785a973a14f1481bc028e2022bcd6a8d9f64
    * ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2
5. Issue a docker pull command to the new VIC appliance using a different repo than the default
    * myregistry.local:5000/testing/test-image
6. Issue a docker pull command to the new VIC appliance using all tags option
    * --all-tags nginx
7. Issue a docker pull command to the new VIC appliance using an image that doesn't exist
8. Issue a docker pull command to the new VIC appliance using a non-default repository that doesn't exist
9. Issue a docker pull command for an image with a tag that doesn't exist
10. Issue a docker pull command for an image that has already been pulled
11. Issue a docker pull command multiple times for the same image
12. Issue a docker pull command for each of two images that share layers
13. Issue docker images, rmi ubuntu, pull ubuntu, docker images commands
14. Issue docker pull command for the same image using multiple tags
18. Issue docker pull on digest outputted by previous pull
19. Issue docker pull for these gcr.io images:
    * gcr.io/google_containers/hyperkube:v1.6.2
    * gcr.io/google_samples/gb-redisslave:v1
    * gcr.io/google_samples/cassandra:v11
    * gcr.io/google_samples/cassandra:v12

# Expected Outcome:
VIC appliance should respond with a properly formatted pull response to each command issued to it. No errors should be seen, except in the case of step 7, 8 and 9. In step 13, the image ID and size for ubuntu should match before and after removing and re-pulling the image.

# Possible Problems:
None
