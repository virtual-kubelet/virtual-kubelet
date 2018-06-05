Test 1-23 - Docker Inspect
=======

# Purpose:
To verify that docker inspect command is supported by VIC appliance

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/inspect/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Deploy VIC appliance to vSphere server
2. Issue docker pull busybox to the VIC appliance
3. Issue docker inspect busybox to the VIC appliance
4. Issue docker inspect --type=image busybox to the VIC appliance
5. Issue docker inspect --type=container busybox to the VIC appliance
6. Issue docker create busybox to the VIC appliance
7. Issue docker inspect <containerID> to the VIC appliance
8. Issue docker inspect --type=container <containerID> to the VIC appliance
9. Issue docker inspect <containerID> to the VIC appliance and verify the Cmd and Image fields
10. Issue docker inspect --type=image <containerID> to the VIC appliance
11. Issue docker network create net-one
12. Issue docker network create net-two
13. Issue docker create --network net-one --name two-net-test busybox
14. Issue docker network connect net-two two-net-test
15. Issue docker start two-net-test
16. Issue docker inspect -f '{{range $key, $value := .NetworkSettings.Networks}}{{$key}}{{end}}' two-net-test
17. Issue docker inspect fake to the VIC appliance
18. Issue docker create -v /var/lib/test busybox
19. Issue docker inspect -f {{.Config.Volumes}} <containerID>
20. Issue docker inspect test-with-volume | jq '.[]|.["Config"]|.["Volumes"]|keys[0]' and docker volume ls
21. Issue docker inspect busybox -f '{{.RepoDigest}}'
22. Issue docker inspect on container with both an anonymous and named volume bound to mount points
23. Issue docker inspect container status across container lifecycle (created, running, exited)

# Expected Outcome:
* Step 3,4,7,8 should result in success and a properly formatted JSON response
* Step 5 should result in an error with the following message:  
```
Error: No such container: busybox
```
* Step 9 should result in success with the correct values in the Cmd and Image fields
* Step 10 should result in an error with the following message:
```
Error: No such image: <containerID>
```
* Step 16 should result in two networks listed in the inspect data
* Step 17 should result in an error with the following message:
```
Error: No such image or container: fake
```
* Step 19 should result in the map returned containing /var/lib/test
* Step 20 should find matching volume ID matching in docker inspect in volume ls
* Step 21 should result in a valid digest, previously cached
* Step 22 should result in valid Mounts data
* Step 23 should result in correct container status values (created, running, exited)

# Possible Problems:
None