
### Docker Personality

The docker personality needed no changes to support nfs volumes. This was due to our Volume Store architecture. Adding a new type of Volume Store still appears as a volumestore to the docker personality. In the future we are planning to distinguish between volume types by customizing our `driver` field. e.g. `vsphere-vmdk` and `vsphere-nfs`. 

This will change the way the personality operates, changes will need to be made on how we identify the volume store in order to know which kind of driver we need to return on ls. Driver can also be specified at volume creation time and it is very possible that we will require users to know which volume store is of which driver type. In order to facilitate this we should modify the output of `docker info` to list not only the available volume stores, but also the drivers associated with each store. Alternatively/additionally we can coax which type of driver belongs to a store programatically, this will require some work in the portlayer and docker around fetching a store by name and determining it's type. 

In the future some mount options like `squashfs`, `sync`, and `retries` may be desired as configuration options at volume creation time. If this is the case work will need to be done on the personality to parse the additional `driver arguments`. Naturally, the portlayer will also need some additional argument parsing logic to facilitate communicating these options to the tether at mount/run time. 

This will also require us to keep a basic set of options for anonymous volumes. At create time users can actually specify several options after the ending `:` character, some investigation will be needed as to how we can use this. The basic options can be used when nothing is specified at the end of the `-v` option on create. We also have a hard coded default capacity for anonymous volumes as this cannot be specified from the docker cli. In the future we should make this configurable at vic-machine create time, [Ticket Reference](https://github.com/vmware/vic/issues/5172). 

examples:

```

docker create -v "/mnt/pnt:<some basic opts>"

and 

docker create -v "<name>:/mnt/pnt:<some basic opts>"

```

Note: Most importantly from a vic-machine standpoint, A `default` store __must__ be tagged at vic-machine create time for anonymous volumes to be supported in a deployment. Without one any anonymous volume creation requests will be rejected and an error will be returned to the user.
    
#### Inputs

+ **mount path** is the destination of where the vdmk will be mounted inside the container. _This is required_ if it is the only value set the user is specifying an anonymous volume and we  generate a UUID as the name for the volume and this UUID is propagated to the volume metadata. 


+ **name** is the value that will be listed a as the name of the volume and the md5 sum of this name will be used as the label for the block device and the target of the mount(portlayer join operation). If this is specified it must be validated. The name is also used to namespace volumes within their respective volumestores. Because of this there is currently and issue out for having a VCH which ends up with two volumes having the same name, [Ticket with information](https://github.com/vmware/vic/issues/5173)

+ **general args** are as follows [rw|ro], [z|Z], [[r]shared|[r]slave|[r]private], and [nocopy]. These should be parsed and placed into the DriverArgs that are specified to the portlayer. right now we only support rw/ro. __TODO__ we do want to research the [nocopy] option. Theses could possibly change in the future for our support depending on how we want to manage other types of volumes beside block based. 


__NOTE:__ : in MountPoint for the volume metadata(docker perspective) we need to include something that says "Mountpoint is a block device" or something along those lines.


### Join call for attaching a volume to a vm

This call, which will be implemented in the volume portion of the storage layer within the portlayer srever, will involve a config spec change. The three things needed for this call are the handle to the container, a filled volume struct, and the driver options for the device addition(such as rw/ro). We will add a value to the extraconfig->executorConfig which will append a new Mountspec for the device to be mounted. The Op type will be an "Add"

```
[]DeviceChange{
    op:Add,
    state(?):exists,
    VirtualDevice{
    file:<vmdkPath->(should come from volume struct)>
    }
}

[]Extraconfig.append{
    executorConfig:
        label:<generated on creation, should be md5 sum>
        MountPoint:<where to mount the vmdk in the container>
}

```

The function signature should look as such

```
func (v *VolumeStore) Join(container_handle *Handle, volume *Volume, diskOpts map[string]string)
```

this will be added to a new file called vm.go as part of the vsphere package under the storage layer code in the Portlayer.

__NOTE__ : there are now two implementations for volume join: an [NFS implementation](https://github.com/vmware/vic/blob/master/lib/portlayer/storage/nfs/vm.go#L42-L57) and a [VMDK implementation](https://github.com/vmware/vic/blob/master/lib/portlayer/storage/vsphere/vm.go#L28-L48)

