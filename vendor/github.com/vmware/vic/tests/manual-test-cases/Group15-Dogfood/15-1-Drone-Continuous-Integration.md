Test 15-1 Drone Continuous Integration
=======

# Purpose:
To verify the VCH appliance can be used as the docker engine replacement for a drone continuous integration environment

# References:
[1- Drone Server Setup](http://readme.drone.io/setup/overview/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Install a new VCH appliance into the vSphere server
2. Clone the latest git repo code of VIC
3. Execute the following command in the VIC repo:
```
drone exec --docker-host <VCH-IP>:2375 --trusted -E secrets.yml -yaml .drone.yml

```

# Expected Outcome:
VIC should build properly and execute each of the unit and integration tests properly from within the drone environment.

# Possible Problems:
None
