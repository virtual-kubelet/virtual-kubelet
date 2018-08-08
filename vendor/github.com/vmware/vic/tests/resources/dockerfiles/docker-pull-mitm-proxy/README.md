# MITM proxy images

This directory contains the necessary consituent parts for building the images used in `Group1-Docker-Commands/1-02-Docker-Pull.robot`.

## Building the images
### Registry Image
The registry image is in `vendor/distribution-library-image`. The registry image is a [standard docker registry image](https://github.com/docker/distribution-library-image), except that the default `VOLUME` is removed so that busybox is already in the registry when it is run. Preparing the image for use in the test will look something like this:

```console
$ cd vendor/distribution-library-image
$ docker build -t registry .
$ docker pull busybox:latest
$ docker run -itd --net=host registry
$ docker tag busybox localhost:5000/busybox
$ docker push localhost:5000/busybox
$ docker commit registry
$ docker tag registry victest/registry-busybox:latest
$ docker login dockerhub.io # you'll need secrets for this step
$ docker push victest/registry-busybox:latest
```
You may need to add the local registry to your insecure registry list in `/etc/docker/daemon.json` before you can push to the registry.
`docker build .` in either directory to recreate the image. 

### MITMproxy Image
This is a custom image that performs a MITM on the image being pulled from the registry container when Docker is configured to use it as an HTTP proxy. `docker build .` should be sufficient for this image and then just `docker push victest/docker-layer-injection-proxy:latest`. It includes a MITMproxy extension to perform a MITM of a docker image as it passes through the proxy, which injects another layer, and the `mitmdump` binary which is from [the MITMproxy project](https://github.com/mitmproxy/mitmproxy).
