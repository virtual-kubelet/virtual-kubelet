# Concurrent

Concurrent is a simple tool to create/start/stop/destroy container vms. It doesn't use persona or portlayer but entartains the same code.

# Usage

Requires a VCH to be present and also requires busybox image to be in the image store.

```
# VIC_MAX_IN_FLIGHT=32 ./concurrent -service "username:password@VC_OR_ESXI" -datacenter DATACENTER -datastore DATASTORE -resource-pool RP -cluster CLUSTER -vch VCH -concurrency 256 -memory-mb 64
Concurrent testing...

Creating 100% [===================================================================================================================================] 9s
Destroying 100% [=================================================================================================================================] 25s

```

