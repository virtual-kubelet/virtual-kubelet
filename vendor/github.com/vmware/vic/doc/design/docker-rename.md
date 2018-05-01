This is the design document for implementing “docker rename” on vSphere Integrated Containers Engine (VIC Engine).

## Design

### The problem:

Rename involves the containerVM’s display name and the name of the datastore folder (and the files in the folder) of the containerVM on the vsphere UI, the container cache of the VIC Engine and the container network configuration (e.g., network alias). Currently both the containerVM's display name and the folder name are created during VM creation using containerName-containerID, which is done for matching the container information obtained from `docker ps` to the VM displayed on the vSphere UI. Renaming the VM display name on the UI can be achieved by using govc, which however does not update the datastore folder name. In this case, the vi admin would observe inconsistent VM display name and datastore folder name, and it becomes difficult the admin to reference the datastore folder based on the new VM display name.

### Solution:

- We use `containerName-containerShortID` to assemble the VM display name. We do not use containerName-containerID in order to avoid the scenario wherein the containerName gets truncated to satisfy the maximum length of a VM display name in vSphere. In addition, we use the `containerID` to set the name of the datastore folder, thus there is no need to worry about the VM display name and datastore folder name being inconsistent. 

- VM configuration during creation: 

  - vSAN: Since vSAN requires the VM display name to be the same as the datastore folder name during VM creation, we set both the VM display name and the datastore folder name to `containerName-containerShortID` during VM creation.
  - Non-vSAN: We set the VM display name to `containerName-containerShortID` and set the datastore folder name to `containeriD`.
  - VM guestinfo: We replace `guestinfo.vice./common/name` with `common/name` in the `ExtraConfig` of a containerVM and set the scope of `common/name` to `hidden`. Then when `docker rename` is called after the VM is created, we can update the `common/name` with the new name. By doing this, we can persist the new name for a running containerVM.  

- Docker support for rename: When a customer calls `docker rename`, we update the VM display name to the new name in both the docker persona and the portlayer. 
  
  - Network: 

    - Network alias should be updated.
    - If `--link` is used when creating the container, HostConfig of relevant containers should be automatically updated based on the backend data.
    - `/etc/hosts`: it will not have the containerName, since `guestinfo.vice/common/name` does not exist.      
          
  - Storage: Nothing needs to be updated if we set the datastore folder name to containerID.
  - Backward compatibility for containers created by a VCH of an older version that does not support `docker rename`: During VCH upgrade, if an existing containerVM has `guestinfo.vice./common/name` instead of `common/name`, a data migration plugin will add `common/name` to the `ExtraConfig` of this containerVM and update the value accordingly, so that the docker operations that are supported by the old VCH still work. However, since we won't be able to update the binaries of the existing containerVMs, we disable `docker rename` on these containerVMs.

## Testing and Acceptance Criteria

Robot scripts will be written to test the following:

1. VM configuration:
  - After a containerVM is created, use govc to check the display name (containerName-containerShortID) and datastore folder name (containerID if on a non-vSAN setup).

2. Docker support for rename:
  - The basic functionality of `docker rename`
  - Check validity of network alias and HostConfig 
  - `docker-compose up` when there are existing containers for the same service but the configuration or image has been changed
  - `docker-compose up –force-recreate` when there are existing containers for the same service even if the configuration or image has not been changed
  
3. Backward compatibility
  - Add a test case in the upgrade test. Create a container using a VCH that does not support `docker rename`. After upgrading the VCH, the basic docker operations that are supported by the old VCH should work.