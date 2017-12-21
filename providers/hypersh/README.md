hyper.sh provider for virtual-kubelet
=====================================

# Configure for hyper.sh

## Use environment variable

- necessary
  - HYPER_ACCESS_KEY
  - HYPER_SECRET_KEY
- optional
  - HYPER_INSTANCE_TYPE:  default s4
  - HYPER_DEFAULT_REGION: default us-west-1
  - HYPER_HOST: tcp://${HYPER_DEFAULT_REGION}.hyper.sh:443

> You can use You can use either HYPER_HOST or HYPER_DEFAULT_REGION


## Use config file

> default config file for hyper.sh is ~/.hyper/config.json

```
//example configuration file for Hyper.sh
{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "xxxxxx",
			"email": "xxxxxx"
		},
	},
	"clouds": {
		"tcp://*.hyper.sh:443": {
			"accesskey": "xxxxxx",
			"secretkey": "xxxxxx",
			"region": "us-west-1"
		}
	}
}
```

# Usage of virtual-kubelet cli

```
// example 1 : use environment variable
export HYPER_ACCESS_KEY=xxxxxx
export HYPER_SECRET_KEY=xxxxxx
export HYPER_DEFAULT_REGION=eu-central-1
export HYPER_INSTANCE_TYPE=s4
./virtual-kubelet --provider=hyper


// example 2 : use default config file(~/.hyper/config.json)
unset HYPER_ACCESS_KEY
unset HYPER_SECRET_KEY
export HYPER_DEFAULT_REGION=eu-central-1
./virtual-kubelet --provider=hyper


// example 3 : use custom config file, eg: ~/.hyper2/config.json
$ ./virtual-kubelet --provider=hyper --provider-config=$HOME/.hyper2
```


# Quick Start

## create pod yaml

```
$ cat pod-nginx
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  nodeName: virtual-kubelet
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
```

## create pod

```
$ kubectl create -f pod-nginx
```

## list container on hyper.sh

```
$ hyper ps
CONTAINER ID   IMAGE          COMMAND                  CREATED         STATUS         PORTS                NAMES            PUBLIC IP
a0ae3d4112d5   nginx:latest   "nginx -g 'daemon off"   9 seconds ago   Up 4 seconds   0.0.0.0:80->80/tcp   pod-nginx-nginx
```

## server log

```
$ export HYPER_DEFAULT_REGION=eu-central-1
$ ./virtual-kubelet --provider=hyper --provider-config=$HOME/.hyper3
/home/demo/.kube/config
2017/12/20 17:30:30 config file under "/home/demo/.hyper3" was loaded
2017/12/20 17:30:30 
 Host: tcp://eu-central-1.hyper.sh:443
 AccessKey: K**********
 SecretKey: 4**********
 InstanceType: s4
2017/12/20 17:30:31 Node 'virtual-kubelet' with OS type 'Linux' registered
2017/12/20 17:30:31 receive GetPods
2017/12/20 17:30:32 found 0 pods
2017/12/20 17:30:37 receive GetPods
2017/12/20 17:30:37 found 0 pods
2017/12/20 17:30:38 Error retrieving pod 'nginx' from provider: Error: No such container: pod-nginx-nginx
2017/12/20 17:30:38 receive CreatePod "nginx"
2017/12/20 17:30:38 container "a0ae3d4112d53023b5972906f2f15c0d34360c132b3c273b286473afad613b63" for pod "nginx" was created
2017/12/20 17:30:43 container "a0ae3d4112d53023b5972906f2f15c0d34360c132b3c273b286473afad613b63" for pod "nginx" was started
2017/12/20 17:30:43 Pod 'nginx' created.
2017/12/20 17:30:43 receive GetPods
2017/12/20 17:30:43 found 1 pods
2017/12/20 17:30:47 receive GetPods
2017/12/20 17:30:47 found 1 pods
```
