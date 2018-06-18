# Telemetry Support

This is the design proposal for vSphere Integrated Containers Engine's initial integration with Vmware Analytics Cloud  (VAC for short).  This data provides Vmware with usage metrics.  Currently, vSphere Integrated Containers Engine will piggyback on one of the data field that vSphere 6 and 6.5 are already collecting, specifically the OS full name used in the VM.  By providing a custom OS name for both the VCH and container VM, Vmware can determine the install base of VIC, the distribution of version installed, and the number of container VM used in a VCH.

## Design

vSphere Integrated Containers Engine will update the name of the guest full name during VM creation time.  The vSphere SDK allows providing a custom text for the guest OS name if guest id is either otherGuest or otherGuest64 and alternate guest name has a value.  Vmware Analytics Cloud collects customer opted-in metrics (via CEIP) and will use the custom guest name to differentiate between regular VMs and vSphere Integrated Containers Engine's VMs.

For these values to get updated, vic-machine will populate the two fields described above in the VM config spec during creation of the VCH.  During container create, the portlayer will get the default linux VM spec from vic/lib/guest/linux.go with those two fields populated.  Future OS or variants of container VM's linux OS can add default VM spec go files (e.g. vic/lib/guest/redhat.go).  The following is the proposed VCH's and container VM's OS names:

| VM Type | OS Full Name |
| --- | --- |
| VCH | Photon - VCH v.v.v, b, c |
| container VM | Photon - Container v.v.v, b, c |
| container VM | Redhat - Container v.v.v, b, c |
| container VM | Windows - Container v.v.v, b, c |

In the above, v.v.v represents the actual version of vSphere Integrated Containers Engine, b represents build number, and c is the first 7 digit of the git commit sha for the source code.  Note, VIC currently supports a custom Photon OS kernel as the OS for both the VCH and container VM.  The proposed naming scheme above will allow the engine to accommodate other OSes as well as other Linux variants.

## Initial Metrics

Here are all the metrics that will be collected specifically for VIC Product (includes VIC Engine, Harbor, and Admiral):

| Metric | Available with Guest OS Name? | Note |
| --- | --- | --- |
| 1. Number of ESX hosts in the cluster | Y | |
| 2. Version number of ESX hosts | Y | |
| 3. Version number of VC hosts | Y | |
| 4. Presence of Harbor or Admiral with version number | N | Harbor and Admiral projects will handle this |
| 5. Total number of VMs | Y | |
| 6. Total number of VCHs | Y | |
| 7. Total number of container VMs | Y | |
| 8. Number of VCHs per host | Y | |
| 9. Number of container VMs per VCH | Y | |
| 10. Average lifetime of the Container VMS | Y | Will require a join of the VM and VC events tables |

## Testing and Acceptance Criteria

For metrics 1-9 (minus #4):

1. Robot scripts will be written to use govc to query the VCH's and a created container's guest name.
2. A sanity check will also be performed by checking whether the VCH and container VM can be differentiated on the Vmware Analytics Cloud query page.  This will be manual test only as the query cannot be performed outside of Vmware's firewall.
