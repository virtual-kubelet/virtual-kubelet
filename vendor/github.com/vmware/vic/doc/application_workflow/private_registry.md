# Running a private registry with VIC

In this example, we will run a private Docker registry on VIC and push and pull images using VIC.  VMware also offers an enterprise ready registry named [Harbor](https://github.com/vmware/harbor) that can be used in place of the base Docker registry.

## Prerequisite

Before going through this example, we need to re-emphasize some concepts around Docker and VIC.  The following examples are shown using Linux.  For Windows and Mac users, these examples should not differ much.

Installing VIC will also require installing Docker locally.  When Docker is installed, we will get both a client (CLI or command line interface) and a daemon that handles all local container operations.  Local containers are those that run on the user local machine instead of a VMWare vSphere/ESXi environment.  The CLI is important as it will be most user's touchpoint for working with containers on VIC and on their local system.  The distinction between using the CLI against the two environment is very important in this example.  By default, the CLI will use the local Docker daemon.  After setting some environment variables, the CLI can be instructed to send all operations to VIC instead of the local Docker daemon.  The two environment variable are DOCKER_HOST and DOCKER_API_VERSION.

In this example, we are deploying an insecure registry with no authentication for simplicity.  We will also be targeting an ESXi environment.

## Workflow

In terminal #1: (local Docker)

1. Open a terminal and make sure that it will use the local Docker daemon.  At a command prompt, issue  
    ```
        $> unset DOCKER_HOST
    ```  
2. Install a VCH for a private registry using vic-machine
3. Run [Docker's registry](https://docs.docker.com/registry/) on the first VCH
4. Install a second VCH for running applications, making sure to specify --insecure-registry to ensure this second VIC can pull images from the insecure registry in the first VCH.
5. At a terminal command prompt, using regular Docker, tag the images to be destined for the registry.
6. Modify the docker systemd config file to allow pushing to an insecure registry
7. Restart the docker daemon
8. Push the image using the full tagged name (including host IP and port)

In terminal #2: (VIC VCH)

1. Open a terminal and make sure it is using the second VCH.  At a command prompt, issue  
    ```
        $> export DOCKER_HOST=tcp://<VCH_IP>:<VCH_PORT>
        $> export DOCKER_API_VERSION=1.23
    ```
2. Pull the image from the registry VCH

### Example run

terminal 1:
```
$> unset DOCKER_HOST
$> ./vic-machine-linux create --target=192.168.218.207 --image-store=datastore1 --name=vic-registry --user=root --password=vagrant --compute-resource="/ha-datacenter/host/esxbox.localdomain/Resources" --bridge-network=vic-network --no-tls --volume-store=datastore1/registry:default --force

...
INFO[2016-10-08T17:31:06-07:00] Initialization of appliance successful       
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] vic-admin portal:                            
INFO[2016-10-08T17:31:06-07:00] http://192.168.218.138:2378                  
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] Docker environment variables:                
INFO[2016-10-08T17:31:06-07:00]   DOCKER_HOST=192.168.218.138:2375           
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] Connect to docker:                           
INFO[2016-10-08T17:31:06-07:00] docker -H 192.168.218.138:2375 info          
INFO[2016-10-08T17:31:06-07:00] Installer completed successfully             

$> DOCKER_HOST=tcp://192.168.218.138:2375 DOCKER_API_VERSION=1.23 docker run -d -p 5000:5000 --name registry registry:2

$> ./vic-machine-linux create --target=192.168.218.207 --image-store=datastore1 --name=vic-app --user=root --password=vagrant --compute-resource="/ha-datacenter/host/esxbox.localdomain/Resources" --bridge-network=vic-network --no-tls --volume-store=datastore1/vic-app:default --force --insecure-registry 192.168.218.138

...
INFO[2016-10-08T17:31:06-07:00] Initialization of appliance successful       
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] vic-admin portal:                            
INFO[2016-10-08T17:31:06-07:00] http://192.168.218.131:2378                  
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] Docker environment variables:                
INFO[2016-10-08T17:31:06-07:00]   DOCKER_HOST=192.168.218.131:2375           
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00]                                              
INFO[2016-10-08T17:31:06-07:00] Connect to docker:                           
INFO[2016-10-08T17:31:06-07:00] docker -H 192.168.218.131:2375 info          
INFO[2016-10-08T17:31:06-07:00] Installer completed successfully             

$> sudo vi /lib/systemd/system/docker.service
$> sudo systemctl daemon-reload
$> sudo systemctl restart docker
$> docker tag busybox 192.168.218.138:5000/test/busybox
$> docker push 192.168.218.138:5000/test/busybox
```

terminal 2:
```
$> export DOCKER_HOST=tcp://192.168.218.131:2375
$> export DOCKER_API_VERSION=1.23
$> docker pull 192.168.218.138:5000/test/busybox
```

Note, in this example, we disabled TLS for simplicity.  Also, we did not show what was modified in /lib/systemd/system/docker.service.  That is shown below for the example above.

```
[Service]
Type=notify
# the default is not to use systemd for cgroups because the delegate issues still
# exists and systemd currently does not support the cgroup feature set required
# for containers run by docker
ExecStart=/usr/bin/dockerd --tls=false -H fd:// --insecure-registry 192.168.218.138:5000
```

In the second step, we specify the necessary environment variables before the docker run command.  On our Linux machine, this sets the variables for the duration of the operation, and once the docker run finishes, those variables are reverted to their previous values.  We use the registry:2 image.  It is important not to specify registry:2.0 in this example.  Registry 2.0 has issues that prevents the example above from working.