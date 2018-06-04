# Specification to support containers with NFS based shared volumes in VIC

Container users want to be able to access shared storage between their containers programatically.  Docker solves this by way of host specific bind mounts adding [NFS volumes](https://docs.docker.com/engine/reference/commandline/volume_create/#/driver-specific-options).  VIC can streamline this functionality for the user by abstracting this away from the container user and allowing the VI admin to configure the VCH with NFS based volumes access by way of our `VolumeStore`.  This way, the VI admin can add an NFS based VolumeStore to the VCH, and the container user need only create volumes on it without needing to know the details of the NFS target.

### Requirements

Allow the VI admin to
 1. add an NFSv3 based `VolumeStore`

Allow the container user to
 1. specify one or many NFS based `VolumeStore`s to create the container volumes on
 1. verify network connectivity from created containers to the NFS targets
 1. create volumes on the NFS based `VolumeStore`
 1. create 1 or greater containers with NFS based volumes at the given location in the container filesystem namespace
 1. validate the volume is no longer in use and delete it

### Non Requirements

 1. Instantiation or provisioning of shared storage
 2. Exposing shared storage configuration via VIC (e.g. IOPS per client, storage policy, etc.)
 3. Management of shared storage via VIC (e.g. container quiesce for storage maintenance, quota manipulation of the target, etc.)

### Networking

Containers will need to be on an appropriate network for the NFS volume store to be accessible.

There are ways this could be done:
 - allow association of volume-store with container-network to allow direct coupling
 - note in the volume store list what network is required

In our current networking model, this can potentially result in the container using the endpoint vm to NAT NFS traffic to/from the NFS target.  This is a potential bottleneck and single point of failure.  The other mode we support is adding an NFS container network, and then adding containers requiring the target to the same network.  This removes the point of failure but has other issues (*).

_(*) Note: Without microsegmentation support, the services run in the container can potentially be exposed to the network the NFS target is on.  This means containers connecting to a specific NFS target all have direct connectivity to eachothers ports._

Ultimately, (once we have microsegmentation support), we'd like to add the container to the appropriate container network in order for the container to have connectivity to the NFS target.

### Implementation

Adding shared storage to our model fits with the `VolumeStore` interface.  At install, a VI admin can specify an NFS target as a `VolumeStore` (potentially) using a `nfs://host/<path>` URI with a volume store name.  And provided the VCH has access to the network the target is on, the container user only needs to pass the volume store name as one of the `volume create` driver opts to create a volume which will be backed by this shared storage target.  Then many containers can be created with the specified volume attached.

#### Runtime network connectivity validation
We need to inform the user when a container is being created without the appropriate network required to get connectivity to the NFS target. The container will attempt to mount the `Target` on `start` and fail early if the volume cannot be mounted.  It will be up to the user to communicate with the VI admin and create the container on the appropriate network (*).  If the container _is_ on the appropriate network _OR_ the `Target` can be reached via the NAT, the container should mount the volume successfully and move on with `start`.

(*) Note:  This requires a doc change.

We want to fail early in the case of issues mounting the volume.  Possible errors are
 * network connectivity releated
 * `Target` permission related
 
The expectation is the error will be actionable by the user such that if it is a configuration issue related to networking or access, the user can either try the operation again with the right container network configuration, or contact the admin with the action item to allow access to the storage device.

#### VolumeStore
The `VolumeStore` interface is used by the storage layer to implement the volume storage layer on different backend implementations.  The current implementation used by VIC is to manipulate vsphere `.vmdk` backed block devices on the Datastore.  We have create a similar implementation for `NFS` based volume stores.

The advantage to using the interface is the storage layer maintains consistency of the volumes regardless of the storage backend used.  For instance it checks all containers during `volume destroy` to see if the named volume is still referenced by another container (whether the container is powered `on` or `off`).

[For reference](https://github.com/vmware/vic/blob/master/lib/portlayer/storage/volume.go#L36)
```
 35 // VolumeStorer is an interface to create, remove, enumerate, and get Volumes.
 36 type VolumeStorer interface {
 37 »···// Creates a volume on the given volume store, of the given size, with the given metadata.
 38 »···VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*Volume, error)
 39
 40 »···// Destroys a volume
 41 »···VolumeDestroy(op trace.Operation, vol *Volume) error
 42
 43 »···// Lists all volumes
 44 »···VolumesList(op trace.Operation) ([]*Volume, error)
  ...
 48 }
```

When we create the NFS `VolumeStore`, we'll store the NFS target parameters (`host` + `path`) in the implementation's struct.  This is the only information we'll need to mount the NFS target on the container. NFS based volume stores will work for the default volume store as well.

```

// VolumeStore stores nfs-related volume store information
type VolumeStore struct {
	// volume store name
	Name string

	// Service is the interface to the nfs target.
	Service MountServer

	// SelfLink to volume store.
	SelfLink *url.URL
}


```


On creation the volume store target path is mounted to the VCH and the volumes directory and metadata direectories are made under that target path. The client used is one that sits in the user space of the portlayer, this is from the vendored go-nfs client in our vendor directory [For Reference](github.com/fdawg4l/go-nfs-client/nfs). This avoids the issue of The `linux` VFS implementation throwing `sync` errors when mounts are unavailable.

In a container, the volume will be mounted during tether boot time, using the man (2) version of the nfs mount call found in the golang syscall package. Currently, the mount options are not configurable, there are plans to address this going forward as an improvement to the nfs volume implementation. These will be configurable by the VI admin at volume store creation time. They will be configured as query parameters for the url supplied as an nfs target. 

Currently a decision must still be made on how we respond to failed mounts due to a target being unreachable at mount time. This is possible when the target endpoint goes down, or the network topology falls out from under the container. Currently, we have no bidirectional communication between the tether and the portlayer. The current option is to fail the launch of the container, though this is not optimal without returning an appropriate error message to the user(we want the user to be able to rectify the issue, or as above to be able to submit an actionable ticket to their vi admin).

#### VolumeCreate
In the vsphere model, a volume is a `.vmdk` backed block device.  Creation of a volume entails attaching a new disk to the VCH, preparing it with a filesystem, and detaching it.  The resulting `.vmdk` lives in its own folder in the volume store directory (specified during install w/ `vic-machine`).  We're going to follow the same model except there is nothing to prepare.  Each volume will be a directory (which the container client will mount directly) and live at the top of the volume store directory (which we will prepare during install).  We will create the directory for the volume content as well as a directory that sits next to the `volumes` directory. Under the `volumes` directory a directory named after the volume name will be made, that will be the mount target at tether mount time (this path will look like `<vs path>/volumes/<volume id>/<volume contents>`). Likewise, the metadata will be housed under a path denoted under a metadata directory and fetched along with the volume(this path will look like `<vs path>/metadata/<volume id>/dockerMetadata`, the file name is determined by the personality that makes the create request).

Some pseudocode: 
```
func VolumeCreate() {
// volPath := vicVolumePath(nameOfVolume)
// mkdir volPath
// metadataPath := vicVolumeMetaDataPath(nameOfVolume, infomap)
.. mkdir metadataPath
// return volPath
}
```
#### VolumeDestroy
Likewise destroying the volume is simply removing the volume's top level directory as well as removing the metadata file. 
```
func VolumeDestroy() {
// volPath := vicVolumePath(nameOfVolume)
// rm -rf volPath
// metadataPath := vicVolumeMetaDataPath(nameOfVolume, infomap)
// rm -rf metadataPath
// return $?
}
```

#### VolumeList
Listing the volumes is just listing the diretories at the top of the volume store location
```
func VolumesList() {
// volums :=  ls -l vicVolumePath(.)
// volumeMetadatas := ls -l vicVolumeMetaDataDir(.)
// volSlice := MatchVolumesAndMetada(volumes, volumeMetadatas)
// return volMap
}
```

### Testing

#### Functional

 1. Create a VCH with an NFS backed `VolumeStore`, create a volume on the `VolumeStore`, create 2 containers with the volume attached, touch a file from the first container, verify it exists on the 2nd.  Destroy the 2nd container, attempt to destroy the volume and expect a failure.  Poweroff the first container, reattempt destroy of the volume, it should fail.  Then destroy the container and destroy the volume. 
 2. Create a VCH with a nonreachable NFS backed `VolumeStore`.  Check that vic-machine warns about the lack of creation of that volume store. Possibly check for the appropriate logging in the portlayer logs. 
 3. Create a VCH with an NFS backed `VolumeStore`, create a volume on said store. Attach this volume to two different cotainers. Touch a file in the volume with container 1. Confirm that this file shows up in container 2. Remove the VCH and confirm that the volume store is removed as well. 
 4. Create a VCH with an NFS volume store. create a volume and attach it to a container. Touch a file inside the volume attached to that container. Tear down the VCH. Create a new VCh with the same NFS volume store target. Check that the created volume is still there. Attach it to a container. Check that the touched file appears inside the volume. 
 5. Create a volume store with multiple NFS Volume Stores. Create volumes on each store. Mount each of those volumes to the same container. Touch files to them. attach all volumes to a second container. Make sure that all touched files are present (multiple volume version of test 3)
 6. Create a VCH with an nfs volume store. Create a volume attach it to a running container. Attempt to remove the volume. Expect to get a `Volume in Use` error. 
 7. Create a VCH with an nfs store tagged as the `default store`. Test all the functionalities of the anonymous volume functionality (this will likely be multiple tests there are already several tests to mirror from our vmdk based volume store).
 
 Note: there are more tests, and these tests should also be added to the integration tests ticket.
 
 
#### Unit

Whether the `VolumeStore` implementation uses the local VCH to mount the NFS or uses a client library to manipulate the target, the Storer implementation should sit in front of an interface which can be mocked.  The mock should write to the local filesystem so the storer interface can be tested end to end without requiring an NFS server.

The NFS volume store implementation has unit tests. These unit tests can be run with or without an NFS server. To test it against an active NFS server you must run `go test` with the environment var `NFS_TEST_URL` set to the nfs target e.g. `NFS_TEST_URL=nfs://my.nfs.com/my/share/point`.

### Possible questions
 1. Is there any mechanism by which we can indicate available space in the volumestore? Is this necessary data for functional usage.
    * Answer: See Non-requirement 3
 1. Should we allow for read-only volume store? - e.g. publishing datasets for consumption
    * Answer: Needs investigation.  What is RO here (the target or the directory) and what would the container user want to see or expect when such a target was used? This could involve us passing in more arguments at volume creation time. 
 1. Failure handling;  what do we do if a mount is unavailable, does the container go down?
    * Answer:  Needs investigation.  We're relying on the kernel nfs client in the container to handle failures to the target.  There is little we can do during run-time, but we can check availability during container create at a minimum. This has been detailed further above. Currently, the result of an unavailable mount is a silent failure. This is actually detrimental to customers, if they do not check the status of their mounts manually (very unlikely) then it is very possible they will run a container believing that data is being persisted. [Tracking Ticket Reference](https://github.com/vmware/vic/issues/4850)
 1. NFS target mount options-  How do you pass a `uid`/`gid` or `nosquash` mapping?  How do you map `uid`/`gid` to the container at all?
    * Answer:  Requirement and usage needs to be thought through at a higher level.  Mapping of users into containers and mapping of credentials into containers need to be solved in the system as a whole.  However things like `sync` and `retries` will be specified as a driver option invoked with `vic-machine`, and passed out of band to the container.  The container users will not be able to specify mount options specific to the target. Currently, `uid` and `gid` are supported via passing them in as query arguments when specifying the NFS volume store target. These are the only configurable options for now, more discussion will be needed surrounding how we want to handle allowing configuration at target creation time and volume creation time. It is likely that customer feedback( and seeing how customers want to use this) will help drive part of the architecture for mount options. 
