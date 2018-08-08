# Upgrade

After installing a VCH, a VIC engine admin will need to manage the lifecycle of the VCH. One aspect of this lifecycle management is upgrading and patching. For our purposes, patching and upgrading will be treated the same. 

The VIC engine product will release a new, complete, software download bundle for both patches and upgrades, versus, having a sparse patch or a separate upgrade model.

## Requirements

### Scope

vic-machine can upgrade VCH endpointVM no matter it's running or not, so vic-machine upgrade cannot rely on services running in VCH endpointVM.

For the VCH endpointVM, vic-machine upgrade will detect the version difference between the old VCH and itself, this includes detecting the guestinfo changes between the two versions. Then vic-machine upgrade will migrate the guestinfo metadata, and upload updated iso files. If anything incorrect happens during this process, vic-machine upgrade will revert back to old version and status. The endpoint API will have downtime during upgrade, but the running containers will not. 

The containers managed by this VCH will be left running existing version. If the container is running, it will work well if it's not coupled with endpointVM through port-forwarding. After VCH is upgraded, the container management will be resumed even there is container configuration change. Portlayer will be backward compatible to make sure old version's container still work well.

But if there is security patch for container VM kernel or tether, user still need to recreate new container and then replace the old containers at once or gradually based on business requirement. vic-machine upgrade does not take care of container update, which is the same behavior with vanilla docker.

The exception is that in near future we might replace serial port with VMCI, to solve the VM vMotion issue. If after that change, portlayer lose backward compatibility, which means portlayer cannot talk with old container through tether, after endpointVM upgrade, part of container VM management function will loss, which means container list, inspect, network, volume, start/stop/remove still work, but container interaction, including attach, exec, log will not work.

Summary of upgrade requirement:
- Only upgrade VCH endpointVM
- Portlayer support backward compatibility
- If container functionality is impact, user is responsible to replace old container with new container after VCH endpointVM upgrade
- If there is vSphere credential changes, upgrade will replace old credentials with new one

### Version Difference

VIC version includes three parts, "release tag"-"build id"-"github commit hash". vic-machine upgrade will rely on build id to detect one version is newer or older than another one.

User need to provide newer binary with bigger build id, to upgrade existing VCH. This also requires our build system to keep increase build id number no matter any kind of system change. 

Note: after introduced data migration between builds, we introduced one more internal used data migration version. That version is related to data migration plugin only, so will not be shown in vic-machine version. This will be described in data migration section.

### Impact

The VCH endpointVM (control plane) will have downtime during upgrade. Container lifecyles are not coupled with that of the endpointVM and, when using mapped vsphere networks (vic-machine --container-network argument) instead of port forwarding, the network data paths are not dependent on the endpointVM either. 

There will be impact on container interaction while the endpointVM is down:

- no direct access to container logs
- no attach ability
- network outage for NAT based port forwarding

All of those facets should resume normal operation after upgrade is complete. 

Note: Exception might happen if there is container communication changes. Additional operations are required to fix the problem.

### VCH status

vic-machine ls and inspect will show the VCH versions and upgrade status.

### One Step Roll Back

If anything wrong happens during upgrade, vic-machine will rollback VCH to original version and status.

And a little bit further, even vic-machine upgrade succeeds, user is still able to run vic-machine upgrade --rollback to do one step roll back, which means the endpointVM will be rollback to wherever version it was before this time's upgrade.

The benefit is that if the docker endpoint API does not actually work after upgrade (due to vic-machine upgrade mis judgement) or user appliacation still prefer old version's API for whatever reason, they still could get back to old working version easily.

Limitation:
- If there is new version container created after upgrade, rollback will fail. In that case, user should delete new container through docker CLI or portlayer API.
- Only one step roll-back is supported. As long as new upgrade is executed, no matter it's succeeds or not, the old version's rollback persisence will be removed.

### Resume Interrupted Upgrade

If upgrade failed, it will rollback to old version dynamically to make sure user application is not stopped due to upgrade failure. But there is one scenario still might break everything, that is user input Ctrl+C during upgrade, cause vic-machine upgrade is interrupte, it is not able to roll back everything it did, and then it is useful to resume this kind of operation from vic-machine. 

Open Issue: if this is high priority user requirement?

### Downgrade

Verified docker-engine upgrade/downgrade, which works well between 1.11.2 and 1.12. The running container will be stopped after upgrade/downgrade, but is good to start again in new version. And all images and containers information is not lost. But think of the complexity to support version downgrade, we'll not go with this at this moment, only with a limited one step roll back.

### Internet connection
Internet connection is not required to upgrade/downgrade VCH, but newer version's binary should be available for vic-machine.

## Design/Implementation - Phase1

This section described the first simple implementation of upgrade. After this phase, user could upgrade VCH endpointVM from build to build, as long as there is no metadata changes, which means no changes in guestinfo, key value store and image metadata.

This code is already merged, and works for security patch update.

### Versioning

#### Freeze following attributes definition in VCH configuration and container VM configuration
- VCH configuration: ID, Name, Version
- Container VM serial port configuration
- Container VM log/debug file location

#### Configuration Version

Both VCH and container configuration will have version and the value is same with what vic-machine version command shown. After upgrade, VCH configuration version should be updated to new version, but container VM configuration version will still be old one.

New version's VCH will work with both old and new versions containers.

#### Embed iso file version

vic-machine need to identify the iso file version before and after upgrade. Two options here:
- Add version into iso file Primary Volume Descriptor, and read back from vic-machine
- Leverage iso file name including version, e.g. appliance-0.5.0.iso

vic-machine should check iso file version during deployment, and after upload to datastore, version should be appended to iso file name, to make sure mutilple iso files could co-exist in the same VCH, and used for different version's container.

Image file name should not be changed in datastore, cause the file path is used to create container VM. And then vic-machine will leverage file name for version checking during upgrade, to avoid download iso file from datastore back to where vic-machine is running.

Note: No feasible golang library found for this function, so will write our own library to read iso metadata.

#### Refactor current VCH configuration and container VM configuration structure
To make data migration easier, we'll need to refactor current VCH and container VM configuration structure.

VirtualContainerHostConfigSpec is too big, which includes everything in one structure. We're not be able to update part of configuration structure, it will help to update part of data with separated structures based on functionality, e.g. network, storage, and common attributes.

For ContainerVM configuration, there are a few attributes not used for container VM setup, e.g. in ExecutorConfig, Key, LayerID, RepoName, are all not container VM configuration related. Move out irrelated attributes can help the structure stability in the future. 

### Transactional Operation/Roll Back
VM snapshot is used to keep current status for upgrade roll back. Before upgrade, vic-machine will create one snapshot of VCH endpointVM. If anything wrong happens, vic-machine can switch back to the pre-upgrade snapshot.
The one step roll-back will roll back the snapshot as well.

Note: VM snapshot does not persist data in serial port, as we will use datastore files through serial port for vch logs, after roll back, log files will have all error message during upgrade if VCH endpointVM is ever started with new configuration.

### Upgrade Workflow

Here is the upgrade workflow
- Find existing endpointVM
- Query VCH configuration
- Verify new version and existing VCH's version
- Cleanup snapshot and images created from last time's upgrade, if there is any. This step is to make sure the rollback after upgrade is not confused.
- Read back old VCH configuration, and migrate to new version. (data migration will be in phase 2 impl)
- Upload iso files
- Prepare combined endpointVM configuration, both hardware spec (ISO path) and the extraconfig portion (VCH version)

--- everything until this portion can be done without interrupting ongoing operation ---
- Snapshot endpointVM
- Poweroff endpointVM if it's not
- Update VCH configuration together with the migrated configuration data
- Reconfigure VCH endpointVM
- Power on endpointVM, and wait VCH service initialization
- Anything wrong in the above steps, roll back to upgrade snapshot
- Ensure endpointVM is powered on and initialized correctly after rollback to snapshot
- Cleanup env after failed upgrade (remove upgrade snapshot, remove uploaded iso files)
- Leave snapshot and old image files for one step roll-back

This ensures that a failure to upload ISOs for whatever reason is detected before we take down the existing version. It limits the failure modes after shutting down the endpointVM to:

1. failed to update endpointVM configuration
2. failed to power on the endpointVM (e.g. system resource constraints)
3. failed to boot endpointVM
4. failed to acquire network addresses (this has been seen in real world examples - we may wish to attempt to preserve/reuse the IPs the endpointVM had prior to update, which should still be present in the extraconfig)
5. failed to rebuild system state from vsphere infrastructure

## Design/Implementation - Phase2

Although we did lots of code refactor for VCH endpointVM configuration, it's still unavoidable to continue changing that structure. To make sure we don't break upgrade after GA, we need a solution for the data migration anyway.

- EndpointVM guestinfo, which is used to persist VCH endpointVM configuration
- Container guestinfo, which is used to persist container configuration
- KeyValue Store and image metadata, following information is persisted
 - parentMap
 - per-image metadata
 - image layer cache
 - network portlayer data
- EndpointVM log files
- vSphere Object Management Logic

### Data Migration Framework

Guestinfo is versioned by auto generated build number, git commit hash, build timestamp etc, which are all not controllable during development. So we'll add another sequential version for data migration version control only. For each version, there will have one data migration plugin registered into data migration framework.

Each time to migrate data, either for container or endpointVM, the framework will compare latest version with the existing version to generate data migration path, e.g. r2->r3->f4->latest or r4->latest. And then corresponding plugins will be executed sequentially to migrate data.

Container data and endpointVM data will be migrated separately, because they have different lifecycle. All the metadata in endpointVM, including guestinfo, keyvalue store, image metadata, log files are all part of endpointVM scope, which will be considered as one set and migrated at the same time.

As old container binary will not be updated by vic-machine upgrade, we'll not be able to write container data back to container VM guestinfo. In that case, we need to try to avoid container data change even there is version difference between VCH endpointVM and container. And the framework should explicitly provide methods to detect if there is data version difference.

Based on the design above, we had following migration framework interface:

```
// MigrateApplianceConfigure migrate VCH appliance configuration, including guestinfo, keyvaluestore, or any other kinds of change
// InIt accept VCH appliance guestinfo map, and return all configurations need to be made in guestinfo, keyvaluestore, and can have
// more kinds of change in the future. Each kind is one map key/value pair.
// If there is error returned, returned map might have half-migrated value, this is why we don't persist any data in plugin.
func MigrateApplianceConfigure(ctx context.Context, s *session.Session, conf map[string]string) (map[string]string, bool, error)

// MigrateContainerConfigure migrate container configuration
// Migrated data will be returned in map, and input object is not changed.
// If there is error returned, returned map might have half-migrated value.
func MigrateContainerConfigure(conf map[string]string) (map[string]string, bool, error)

func IsContainerDataOlder(conf map[string]string) (bool, error)
func IsApplianceDataOlder(conf map[string]string) (bool, error)
```

Here is the interface for plugin and plugin manager:

```
type DataMigration interface {
	// Register plugin to data migration system
	Register(version int, target string, plugin Plugin) error
	// Migrate data with current version ID, return true if has any plugin executed
	Migrate(ctx context.Context, s *session.Session, target string, currentVersion int, data interface{}) (int, error)
	// GetLatestVersion return the latest plugin id for specified target
	GetLatestVersion(target string) int
}
type Plugin interface {
	Migrate(ctx context.Context, s *session.Session, data interface{}) error
}
```

Notes:
- Developers who change guestinfo, keyvalue store, image metadata, etc, will be the owner to develop migration plugin, and be responsible to increase data migration version. 
- Each migration version should have one and only one corresponding plugin.
- If endpointVM configuration and container configuration are changed at the same time, two different plugins should be added, and registered to different plugin category.
- If both endpointVM configuration and keyvalue store are changed, two plugins are recommended as well.

Reminder: extraconfig package should always be backward compatible. If it breaks this assumption, upgrade is broken.

### EndpointVM Migration Process

vic-machine upgrade will upgrade endpointVM and data migration framework will be called to migrate data, which is described in phase1 workflow. There is no new command parameters added for data migration.

#### Guestinfo migration

The plugin should migrate data in memory and return changed value directly. No need to persist any data.

The difficulty is how to migrate secret data in guestinfo. The encryption key is persisted through raw guestinfo.ovfEnv variable, which is readable from in guest, but not through vSphere API, no matter it's VC or ESXi. Decrypt and re-encrypt secret data is necessary because otherwise if old vc login credential is changed, upgrade will always fail.

To solve this issue, we'll need to download endpointVM vmx file and read back guestinfo.ovfEnv value to decrypt encrypted data. In another word, this will be another variable dependent by upgrade framework, which cannot be changed in the future, otherwise, upgrade is broken.

#### KeyValue Store and Image metadata Migration

Different to guestinfo object, VIC engine will not change configuration from guestinfo to anywhere else anytime soon. But from start, keyvalue store persistent position is in argument. Right now, vic engine has a few datastore files for keyvalue store and image metadata, but those information is not configurable, which means hardcoded in portlayer.

Based on this idea, data migration framework will not assume where to load these data, and what version is that, instead, it assume all data for endpointVM is sharing the same version written in VM guestinfo. After compare the version difference, it will invoke plugins sequentially to migrate data including all endpointVM data. The plugin, for this kind of data, should read from datastore, change data and then commit back to datastore, or to anywhere defined in new version. So the input of keyvalue store plugin will be the endpointVM configuration data.

The problem of this solution is that each single plugin will persist its own change, not like the guestinfo update, which is migrated in memory and persisted by migration framework. So the rollback for guestinfo is easy, but not possible for keyvalue store, unless we have data roll back plugin mechanism.

The solution for this issue is to have one model to write key/value store and image metadata migration plugin. Every time we need something new and have to be migrated in new version, we should create new datastore files with a suffix matching the version number, e.g. metadata.v3, and then in the migration plugin, copy all existing data into new versioned files, and modify over there. The old versioned datastore files is not changed by the plugin, so even if eventually we dropped the upgrade, the old binary could still work with the old files without any problem.

#### EndpointVM log files

EndpointVM log files can be handled similar to KeyValue store.

#### vSphere Object Management Logic

There will have new vSphere API come up, so the logic to manage vsphere objects will be enhanced, for example, as Caglar said the vmdk can be managed directly in vSphere 6.5, instead of through vm operations in vSphere 6.0. We'll switch to these new interfaces for image and volume management sometime later.

vic-machine is not supposed to migrate old image data or volume data to new vmdk files, so portlayer will need to be backward compatible.

### Container Data Migration Process

vic-machine upgrade will leave container in old version, and do not update it even container is restarted. So portlayer will be responsible for container's backward compatibility. Here is how portlayer talk to old containers.

- Load container configuration
- Check if data migration is required. If yes, migrate data, and conver to new version's data structure in memory
- Read/write from/to new version's data for whatever container operations
- While need to write container configuration back to container VM guestinfo, check if data migration is done. If yes, skip writing.

Risks:
- As the new data is not written back, there will have few container information inconsistence

  Currently, portlayer will write container status into guestinfo before start/stop, and tether will write some. If portlayer does not write this information because of data version mismatch, the container status will have problem if we still rely on portlayer to write it.
  In the future, if portlayer add more functions to modify container, and those information is persisted in guestinfo, those change will not work for old container, for example, container rename.
- If portlayer and container communication channel is changed, from serial port to VMCI, and no backward compatibility support, container attach/exec/log etc., interaction related command will not work, but container image/volume/network/start/stop/remove should still work well.

In the above cases, user should think about replace old container with new created one. And if that happens, during endpointVM upgrade, vic-machine should have clear information to mention the container functional limitation after upgrade, and the suggested solution for it.

### One Step Roll Back
To support one step roll back, we'll have one new option in vic-machine upgrade command as following
```
vic-machine upgrade --rollback --<same other upgrade options>
```
Following is the workflow
- Find existing VCH endpointVM
- Check if upgrade snapshot available, and consistent with previous iso file version, if not, stop rollback
- Check if there is any newer version's container created already, if yes, print warning message and stop rollback
- Check if old iso files with same snapshot version still exist, if not, cannot rollback

- Snapshot current endpointVM with version in name
- Switch to old version's snapshot
- Power on endpointVM, and wait VCH service initialization
- Anything wrong in the above steps, roll back to new version's snapshot
- Ensure endpointVM is powered on and initialized after rollback to newer version
- Remove new version's upgrade snapshot if rollback failed
- Remove new version's upgrade snapshot and new version's iso files if rollback succeeds (after rollback, user is still able to reupgrade through the above operation, do not leave anything to avoid confusing)

### Resume Interrupted Upgrade
For user interrupted upgrade through Ctrl+C during upgrade is running, the VCH endpointVM is left in partial migrated status, which is hard to handle manually. Another option in vic-machine upgrade will help on this.

```
vic-machine upgrade --resume --<same other upgrade options>
```

Following is the workflow
- Check if there is one upgrade in progress, we'll rely on upgrade snapshot to see if the VCH is in upgrade. If anything is interrupted before upgrade snapshot is created, VCH endpointVM is not actually changed, user should use general upgrade command to continue.
- Check if docker API is available, if yes, last time's upgrade is already done, no need to resume (if vic-machine upgrade succeeds, but something else still does not work correctly, user should use one step rollback, instead of resume here)
- Check if there is any newer version's container created already, if yes, stop resume. Which means new VCH endpoint API already works, but occasionally break by something else, resume cannot help on that.

- Check if old iso files with same snapshot version still exist, if not, cannot resume
- Upload new iso files no matter they exist or not, to solve any potential iso file broken issue

- Switch to old version's snapshot
- Query endpointVM guestinfo, and migrate to new version. 
- Power off endpointVM if it's not
- Update VCH configuration together with the migrated configuration data
- Reconfigure VCH endpointVM
- Power on endpointVM, and wait VCH service initialization
- Anything wrong in the above steps, roll back to upgrade snapshot
- Ensure endpointVM is powered on and initialized correctly after rollback to snapshot
- Cleanup env after failed upgrade (remove upgrade snapshot, remove uploaded iso files)
- Leave snapshot and old image files for one step roll-back

## Restrictions
User cannot run two upgrades for same VCH at the same time. 

vic-machine will check if there is already another upgrade snapshot is created before it starts to create snapshot. But as create vsphere snapshot will take some time, e.g. one minute, if at this time, another upgrade process is started, it will start upgrade again cause the snapshot of previous task is not finished yet.

## Integration Test
### Upgrade test w/o data migration
CI test already have test to upgrade from previous release to latest version, and run regression test after upgrade. With data migration framework added, no new integration test is required. But for any data migration added in the future, upgrade test should cover the new features regression.

### One Step Roll Back
After upgrade, run one step roll back and check the old version's VCH. Following scenarios should be covered.
- Roll back works without new container created, and sample function works (do not run regression test for that might have new features not suppored in old VCH)
- Roll back failed for new container created
- Roll back works after removed new version's container, and sample function works
- After roll back, VCH old version is recovered, and new feature is not supported

### Upgrade Resume
During upgrade, break the upgrade process, and check if vic-machine can resume the upgrade. Following scenarios should be covered.
- Before snapshot is created, stop upgrade, resume does not work for no upgrade is found
- Upgrade succeeds, resume should not start for docker endpoint API is ready
- After snapshot is created, stop upgrade, and remove the old version iso files, resume should fail
- After snapshot is created, stop upgrade, and do not touch everything else, resume should work, regression passed
- After resume, one step roll back should still work