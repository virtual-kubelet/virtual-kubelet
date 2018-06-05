Test 1-25 - Docker Port Mapping
=======

# Purpose:
To verify that docker create works with the -p option

# References:
[1 - Docker Command Line Reference](https://docs.docker.com/engine/reference/commandline/create/)

# Environment:
This test requires that a vSphere server is running and available

# Test Cases

## Create container with port mappings
1. Deploy VIC appliance to vSphere server
2. Issue `docker create -it -p 10000:80 -p 10001:80 --name webserver nginx`
3. Issue `docker start webserver`
4. Issue `curl vch-ip:10000 --connect-timeout 20`
5. Issue `curl vch-ip:10001 --connect-timeout 20`
6. Issue `docker stop webserver`
7. Issue `curl vch-ip:10000`
8. Issue `curl vch-ip:10001`

### Expected Outcome:
* Steps 2-6 should all return without error
* Steps 7-8 should both return error


## Create container with conflicting port mapping
1. `Issue docker create -it -p 8083:80 --name webserver2 nginx`
2. `Issue docker create -it -p 8083:80 --name webserver3 nginx`
3. `Issue docker start webserver2`
4. `Issue docker start webserver3`

### Expected Outcome:
* Steps 1-3 should all return without error
* Step 4 should return error


## Create container with port range
1. Issue `docker create -it -p 8081-8088:80 --name webserver5 nginx`

### Expected Outcome:
* Step 1 should return error


## Create container with host IP
1. Issue `docker create -it -p 10.10.10.10:8088:80 --name webserver5 nginx`

### Expected Outcome:
* Step 1 should return error


## Create container without specifying host port
1. Issue `docker create -it -p 6379 --name test-redis redis:alpine`
2. Issue `docker start test-redis`
3. Issue `docker stop test-redis`

### Expected Outcome:
* Steps 1-3 should return without error


## Run after exit remapping mapped ports
1. Deploy VIC appliance to vSphere server
2. Issue `docker run -i -p 1900:9999 -p 2200:2222 busybox /bin/top`
3. Issue `q` to the container
4. Issue `docker run -i -p 1900:9999 -p 3300:3333 busybox /bin/top`
5. Issue `q` to the container

### Expected Outcome:
* All steps should return without error

## Remap mapped ports after OOB Stop
1. Deploy VIC appliance to vSphere server
2. Issue `docker create -it -p 10000:80 -p 10001:80 busybox`
3. Issue `docker start <containerID>` to the VIC appliance
4. Power off the container with govc
5. Issue `docker create -it -p 10000:80 -p 20000:2222 busybox`
6. Issue `docker start <containerID>` to the VIC appliance

### Expected Outcome:
* All steps should return without error


## Remap mapped ports after OOB Stop and Remove
1. Issue `docker run -itd -p 5001:80 --name nginx1 nginx`
2. Hit Nginx Endpoint at VCH-IP:5001
3. Power off the container with govc
4. Issue `docker rm nginx1`
5. Issue `docker run -itd -p 5001:80 --name nginx2 nginx`
6. Hit Nginx Endpoint at VCH-IP:5001

### Expected Outcome:
* All steps should return without error


## Container to container traffic via VCH public interface
1. Deploy VIC appliance to vSphere server
2. Issue `docker create -p 8080:80 --net bridge nginx`
3. Issue `docker start <containerID>` to the VIC appliance
4. Issue `docker run busybox /bin/ash -c wget -O index.html <VCH IP>:8085; md5sum index.html`
6. Verify the contents of `index.html`
7. Issue `docker run busybox /bin/ash -c wget -O index.html <server IP>:80; md5sum index.html`
8. Verify the contents of `index.html`

### Expected Outcome:
* All steps should return without error

## Remap mapped port after stop container, and then remove stopped container
1. Issue `docker run -itd -p 6001:80 --name remap1 nginx`
2. Hit Nginx Endpoint at VCH-IP:6001
3. Issue `docker stop remap1`
4. Issue `docker run -itd -p 6001:80 --name remap2 nginx`
5. Issue `docker rm remap1`
6. Hit Nginx Endpoint at VCH-IP:6001

### Expected Outcome:
* All steps should return without error
