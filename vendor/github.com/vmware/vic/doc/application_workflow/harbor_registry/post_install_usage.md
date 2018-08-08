# Using vSphere Integrated Container Engine with Harbor

Here we show an example of using a deployed VCH with Harbor as a private registry.  We assume that one has been setup using either static IP or FQDN.  We also assume standard docker has been updated with the certificate authority cert that can verify the deployed Harbor's server cert.
<br><br>

## Workflow

1. Develop or obtain a docker container image on a computer (or terminal) using standard docker.  Tag the image for Harbor and push the image to the server.
2. Pull down the image from Harbor to a deployed VCH and use it.
<br><br>

## Push a container image to Harbor using standard docker

In this step, we pull the busybox container image from the docker hub down to our laptop, which had the CA certificate updated for docker use earlier.  Then we tag the image for uploading to our Harbor registry and push the image up to it.  Please note, we log onto the Harbor server before pushing the image up to it.

```
loc@Devbox:~/mycerts$ docker pull busybox
Using default tag: latest
latest: Pulling from library/busybox

56bec22e3559: Pull complete 
Digest: sha256:29f5d56d12684887bdfa50dcd29fc31eea4aaf4ad3bec43daf19026a7ce69912
Status: Downloaded newer image for busybox:latest
loc@Devbox:~/mycerts$ 
loc@Devbox:~/mycerts$ docker tag busybox <Harbor FQDN or static IP>/test/busybox

loc@Devbox:~/mycerts$ docker login <Harbor FQDN or static IP>
Username: loc
Password: 
Login Succeeded

loc@Devbox:~/mycerts$ docker push <Harbor FQDN or static IP>/test/busybox
The push refers to a repository [<Harbor FQDN or static IP>/test/busybox]
e88b3f82283b: Pushed 
latest: digest: sha256:29f5d56d12684887bdfa50dcd29fc31eea4aaf4ad3bec43daf19026a7ce69912 size: 527
```

## Pull the container image down to the VCH

Now, in another terminal, we can pull the image from Harbor to our VCH.

```
loc@Devbox:~$ export DOCKER_HOST=tcp://<Deployed VCH IP>:2375
loc@Devbox:~$ export DOCKER_API_VERSION=1.23
loc@Devbox:~$ docker images
REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE

loc@Devbox:~$ docker pull <Harbor FQDN or static IP>/test/busybox
Using default tag: latest
Pulling from test/busybox
Error: image test/busybox not found

loc@Devbox:~$ docker login <Harbor FQDN or static IP>
Username: loc
Password: 
Login Succeeded

loc@Devbox:~$ docker pull <Harbor FQDN or static IP>/test/busybox
Using default tag: latest
Pulling from test/busybox
56bec22e3559: Pull complete 
a3ed95caeb02: Pull complete 
Digest: sha256:97af7f861fb557c1eaafb721946af5c7aefaedd51f78d38fa1828d7ccaae4141
Status: Downloaded newer image for test/busybox:latest

loc@Devbox:~$ docker images
REPOSITORY                                           TAG                 IMAGE ID            CREATED             SIZE
<Harbor FQDN or static IP>/test/busybox   latest              e292aa76ad3b        5 weeks ago         1.093 MB
loc@Devbox:~$ 
```

Note above, on our first attempt to pull the image down, it failed, with a 'not found' error message.  Once we log into the Harbor server, our attempt to pull down the image succeeds.