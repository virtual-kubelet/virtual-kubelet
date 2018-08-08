Test 23-08 - VCH Delete
=======================

#### Purpose:
To verify vic-machine-server deletes the correct set of objects as a part of an API call

#### References:
1. [VIC Machine Service API Design Doc](../../../doc/design/vic-machine/service.md)

#### Environment:
This suite requires a vSphere system where VCHs can be deployed with default and named volume stores.


Basic Validation
----------------

###  1. Delete VCH

#### Test Steps:
1. Create a VCH
2. Issue a delete request without a body
3. Verify that the VCH no longer exists

#### Expected Outcome:
* The VCH is deleted


###  2. Delete VCH within datacenter

#### Test Steps:
1. Create a VCH
2. Issue a delete request without a body to the datacenter-scoped handler
3. Verify that the VCH no longer exists

#### Expected Outcome:
* The VCH is deleted


###  3. Delete the correct VCH

#### Test Steps:
1. Create two VCHs
2. Issue a delete request for one VCH
3. Verify that the deleted VCH no longer exists
4. Verify that the other VCH still exists

#### Expected Outcome:
* Only the expected VCH is deleted


Negative Cases
--------------

###  4. Delete invalid VCH

#### Test Steps:
1. Create a VCH
2. Issue a delete request for a non-existent VCH
3. Verify that the VCH still exists

#### Expected Outcome:
* The erroneous deletion returns a 404 Not Found without side effects


###  5. Delete VCH in invalid datacenter

#### Test Steps:
1. Create a VCH
2. Issue a deletion request for that VCH scoped to an invalid datacenter
3. Verify that the VCH still exists

#### Expected Outcome:
* The erroneous deletion returns a 404 Not Found without side effects


###  6. Delete with invalid bodies

#### Test Steps:
1. Create a VCH
2. Issue a delete request for that VCH with a malformed body
3. Issue a delete request for that VCH with an invalid value for the `containers` element
4. Issue a delete request for that VCH with an invalid value for the `volume_stores` element
5. Issue a delete request for that VCH with a valid value for the `volume_stores` element, but an invalid value for the `containers` element
6. Issue a delete request for that VCH with a valid value for the `containers` element, but an invalid value for the `volume_stores` element
7. Verify that the VCH still exists

#### Expected Outcome:
* Each erroneous deletion returns a 422 Unprocessable Entity without side effects


With Containers
---------------

###  7. Delete VCH with powered off container

#### Test Steps:
1. Create a VCH with a powered off container
2. Issue a delete request without a body
3. Verify that the VCH no longer exists
4. Verify that the powered off container VM no longer exists

#### Expected Outcome:
* The VCH and powered off container are both deleted


###  8. Delete VCH with powered off container deletes files

#### Test Steps:
1. Create a VCH with a powered off container
2. Issue a delete request without a body
3. Verify that the files for the powered off container VM no longer exist
4. Verify that the VCH no longer exists

#### Expected Outcome:
* The VCH and powered off container are both deleted


###  9. Delete VCH without deleting powered on container

#### Test Steps:
1. Create a VCH with a powered off container and a powered on container
2. Issue a delete request without a body
3. Verify that the VCH still exists
4. Verify that the powered on container VM still exists
5. Verify that the powered off container VM no longer exists

#### Expected Outcome:
* The powered off container is deleted, but the deletion fails with a 500 Internal Server Error and the VCH and powered on container both remain


### 10. Delete VCH explicitly without deleting powered on container

#### Test Steps:
1. Create a VCH with a powered off container and a powered on container
2. Issue a delete request with a body expressing that only powered off containers should be deleted
3. Verify that the VCH still exists
4. Verify that the powered on container VM still exists
5. Verify that the powered off container VM no longer exists

#### Expected Outcome:
* The powered off container is deleted, but the deletion fails with a 500 Internal Server Error and the VCH and powered on container both remain


### 11. Delete VCH and delete powered on container

#### Test Steps:
1. Create a VCH with a powered off container and a powered on container
2. Issue a delete request with a body expressing that all containers should be deleted
3. Verify that the VCH still exists
4. Verify that the powered on container VM no longer exists
5. Verify that the powered off container VM no longer exists

#### Expected Outcome:
* The VCH and both containers are deleted


With Volumes
------------

Note: Because it is more difficult to verify the existence or deletion of anonymous volumes, they are not included in the following tests. If the deletion code is ever updated to treat different types of volumes differently, tests should be added.

### 12. Delete VCH and powered off containers and volumes

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered off container that has a named volume on the default volume store
3. Create a powered off container that has a named volume on the named volume store
4. Issue a delete request with a body expressing that powered off containers and volume stores should be deleted
5. Verify that the VCH no longer exists
6. Verify that the container VMs no longer exist
7. Verify that the volume stores and volumes no longer exist

#### Expected Outcome:
* The VCH, both containers, both volume stores, and both are deleted


### 13. Delete VCH and powered on containers and volumes

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered on container that has a named volume on the default volume store
3. Create a powered on container that has a named volume on the named volume store
4. Issue a delete request with a body expressing that all containers and volume stores should be deleted
5. Verify that the VCH no longer exists
6. Verify that the container VMs no longer exist
7. Verify that the volume stores and volumes no longer exist

#### Expected Outcome:
* The VCH, both containers, both volume stores, and both are deleted


### 14. Delete VCH and powered off container and preserve volumes

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered off container that has a named volume on the default volume store
3. Create a powered off container that has a named volume on the named volume store
4. Issue a delete request with a body expressing that powered off containers, but not volume stores, should be deleted
5. Verify that the VCH no longer exists
6. Verify that the container VMs no longer exist
7. Verify that the volume stores and volumes still exist
8. Create a new VCH re-using the same volume stores
9. Create a powered off container re-using the named volume on the default volume store
10. Create a powered off container re-using the named volume on the named volume store

#### Expected Outcome:
* The VCH and both containers are deleted, but both volume stores and volumes are preserved and re-usable


### 15. Delete VCH and powered on container but preserve volume

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered on container that has a named volume on the default volume store
3. Issue a delete request with a body expressing that all containers, but not volume stores, should be deleted
4. Verify that the VCH no longer exists
5. Verify that the container VM no longer exists
6. Verify that the volume store and volume still exists
7. Create a new VCH re-using the same volume store
8. Create a powered off container re-using the named volume on the default volume store

#### Expected Outcome:
* The VCH and container are deleted, but the volume store and volume are preserved and re-usable


### 16. Delete VCH and preserve powered on container and volumes

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered on container that has a named volume on the default volume store
3. Issue a delete request with a body expressing that neither powered on containers nor volume stores should be deleted
4. Verify that the VCH still exists
5. Verify that the container VM still exists
6. Verify that the volume store and volume still exist

#### Expected Outcome:
* The deletion fails with a 500 Internal Server Error and the VCH, powered on container, volume store, and volume all remain


### 17. Delete VCH and preserve powered on container and fail to delete volumes

#### Test Steps:
1. Create a VCH with a named volume store
2. Create a powered on container that has a named volume on the default volume store
3. Issue a delete request with a body expressing that volume stores, but not powered on containers, should be deleted
4. Verify that the VCH still exists
5. Verify that the container VM still exists
6. Verify that the volume store and volume still exist

#### Expected Outcome:
* The deletion fails with a 500 Internal Server Error and the VCH, powered on container, volume store, and volume all remain


Interoperability
----------------

### 18. Create an VCH with an old version of the CLI and attempt to delete it

(Not implemented.)


Concurrency
-----------

### 19. Attempt to delete a VCH while it is already being deleted

(Not implemented.)
