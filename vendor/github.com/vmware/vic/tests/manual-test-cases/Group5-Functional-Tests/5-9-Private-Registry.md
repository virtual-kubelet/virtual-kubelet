Test 5-9 - Private Registry
=======

# Purpose:
To verify the VIC appliance can pull an image from a private registry

# References:
[1 - Docker Registry](https://hub.docker.com/_/registry/)

# Environment:
This test requires access to a vSphere Server

# Test Steps:
1. Install a new VCH appliance into the vSphere Server
2. Start the docker registry locally:  
```docker run -d -p 5000:5000 --name registry registry```
3. Pull an image and tag it for the new local registry:  
```docker tag busybox localhost:5000/busybox:latest```
4. Push the tagged image to the registry:  
```docker push localhost:5000/busybox```
5. Attempt to pull the local registry image using the VCH appliance:  
```docker pull %{VCH-PARAMS} 172.17.0.1:5000/busybox```

# Expected Outcome:
The VCH appliance should be able to successfully pull the image from a local registry without error

# Possible Problems:
The default network on VCH containers is in the 172.17.0.x space, but this could potentially change. If you are having issues running the tests make sure that the container has an IP address in that network space.
