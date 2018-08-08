# Development focused helper scripts for VCH management

The file `bash-helpers.sh` combines with a set of VCH 'profiles' to allow easy switching of VCH configurations for new deployments, and easy selection of existing VCHs for interaction, modification or deletion.
The element likely to be used most frequently, if leveraging these functions, is that they configure the shell environment so that the docker client will target the selected VCH by default.

The helper scripts are used by `sourcing` them into your shell (assumes `bash-4.0` currently). Run the following from the root of your repo:
```console
. ./infra/scripts/bash-helpers.sh
```
or
```console
source ./infra/scripts/bash-helpers.sh
```

This will configure your _current shell_ with functions to activate a profile and manage VCHs.


## Profiles

The profiles are defined as shell functions in either `~/.vic` or `<vic-repo>/.vic.profiles`. These shell functions are simple declarative lists of variables that will be processed into vic-machine command lines. This is a small sample profile with only the basics set (suited for ESX not VC given no pre-existing DPG). The file `sample-helper.profiles` has a full example with all of the values:
```shell
my-vch () {
    ## first is some boiler plate that is common to all profiles
    #   clear any settings from a different profile
    #   set the variable $vch_name to the name of the profile
    init-profile

    ## below here are the profile specific variables
    #
    #   vSphere target
    vsphere='host.domain.com'
    datacenter='datacenter'
    user='username'
    password='password'

    # uncomment opsuser and opspass if user/password above should not be used for ongoing vch operations
    opsuser='ops-user'
    opspass='password'
    # uncomment to automatically grant appropriate privileges to the ops user
    opsgrant=1

    # add two volume stores
    #  "default" enables anonymous volumes and is the default if -opt VolumeStore=name is not specified
    #  "non-default" is just a second volume store for easy use/testing

    # uncomment preserveVolumestores if the stores should not be deleted by default during vic-delete
    volumestores=("--volume-store=${datastore:-datastore1}/${vch_name}_default_vols:default" "--volume-store=${datastore:-datastore1}/${vch_name}_non-default_vols:non-default")
    #preserveVolumestores=1

    # adds a DHCP container network on the same vSwitch/Portgroup as the publicNet. If publicNet is static then this needs the additional network options
    # added to the array
    containernet=("--container-network=${publicNet:-VM Network}:public")
}
```

## Helper commands

The follow are the set of available helper commands and some minor usage examples. In general the commands are written so that you can add additional arguments to the end of the function and it will be passed through to the underlying command.

These examples all assume that you have selected a profile by running the desired shell function and that you have the requisite binary files built in the `bin` folder of your repo:
```console
$ my-vch
```
The set of commands available are:
* vic-create
* vic-ls
* vic-select
* vic-inspect
* vic-upgrade
* vic-delete
* vic-ssh
* vic-path
* vic-admin
* vic-tail-docker
* vic-tail-portlayer

The _VCH_ created by or identified by the active profile is referred to as the _current_ VCH in this document.

### Create a VCH

Creates a VCH from the files built in the repo and configures the shell environment to use it by default (see `vic-select`):
```console
$ vic-create
INFO[0000] ### Installing VCH ####
WARN[0000] Using administrative user for VCH operation - use --ops-user to improve security (see -x for advanced help)
INFO[0000] Using public-network-ip as cname where needed - use --tls-cname to override: 192.168.78.127/24
...
```
or with debug enabled
```console
$ vic-create --debug=2
INFO[0000] ### Installing VCH ####
WARN[0000] Using administrative user for VCH operation - use --ops-user to improve security (see -x for advanced help)
DEBU[0000] client network: IP {<nil> <nil>} gateway <nil> dest: []
...
```

### List VCHs

List the VCHs deployed _in the vSphere environment targeted by the current profile_:
```console
$ vic-ls
INFO[0000] ### Listing VCHs ####
INFO[0000] Validating target

ID         PATH                                                       NAME                   VERSION                     UPGRADE STATUS
193        /ha-datacenter/host/localhost.localdomain/Resources        my-vch                 v1.4.0-dev-0-1a82e8c        Up to date
```

### Select an existing VCH

This will configure your environment to use and reference an existing VCH, deployed via a profile. `vic-select` is called automatically during `vic-create` and does the following:
* configures or clears a shell alias for `docker --tls` as necessary
* configures the `DOCKER_HOST`, `DOCKER_CERT_PATH`, and `DOCKER_TLS_VERIFY` environment variables as appropriate for the profile

This means that following `vic-select` _that_ shell will be configured so the docker client talks to the current VCH by default.

```console
$ other-vch
$ vic-select
$ docker info
...
```

### Inspect VCH

Inspects the current VCH. If the first argument to the shell function is `config` then this will call the `inspect config` subcommand instead of the top level `inspect` command:
```console
$ vic-inspect
INFO[0000] ### Inspecting VCH ####
INFO[0000] Validating target
...
```

### Upgrade VCH

Upgrade the current VCH using the binaries build in your repo:
```console
$ vic-upgrade
INFO[0000] ### Upgrading VCH ####
INFO[0000] Validating target
...
```

### Delete VCH

Deletes the current VCH and it's volume stores unless the profile is configured with `preserveVolumeStores`. This is a very coarse all or nothing toggle and just maps to using `--force` or not.
```console
$ vic-delete
INFO[0000] ### Removing VCH ####
INFO[0000] Validating target
...
```

### SSH into a VCH

Enables ssh on the endpointVM via vic-machine debug, configures the endpoint for ssh, extracts the IP address and ssh's into the endpoint. You are left with an interactive shell.
```console
$ vic-ssh
SSH to 192.168.78.127
Warning: Permanently added '192.168.78.127' (ECDSA) to the list of known hosts.
Warning: your password will expire in 0 days
root@ [ ~ ]#
```

### Repo Path

Simply prints the repo path (dependent on GOPATH being set correctly)

```console
$ vic-path
/home/vagrant/vicsmb//src/github.com/vmware/vic
```

### Open vicadmin (OSX only)

This uses the OSX `open` command to launch a browser pointed at the vic-admin URL
```console
$ vic-admin
### Opens https://vch-ip:2378 in a browser
```

### Tail logs from console

These allow you to tail the docker personality or port layer logs respectively:
```console
$ vic-tail-docker
SSH to 192.168.78.127
Warning: Permanently added '192.168.78.127' (ECDSA) to the list of known hosts.
Feb 27 2018 04:27:53.378Z DEBUG [ END ]  [vic/lib/apiservers/engine/backends/cache.(*ICache).GetImages:127] [75.045µs]
Feb 27 2018 04:27:53.395Z DEBUG [BEGIN]  [vic/lib/apiservers/engine/backends.(*SystemProxy).PingPortlayer:56] PingPortlayer
Feb 27 2018 04:27:53.395Z DEBUG [ END ]  [vic/lib/apiservers/engine/backends.(*SystemProxy).PingPortlayer:56] [249.164µs] PingPortlayer
Feb 27 2018 04:27:53.395Z DEBUG [BEGIN]  [vic/lib/apiservers/engine/backends.(*SystemProxy).VCHInfo:109] VCHInfo
Feb 27 2018 04:27:53.453Z DEBUG [ END ]  [vic/lib/apiservers/engine/backends.(*SystemProxy).VCHInfo:109] [57.583757ms] VCHInfo
Feb 27 2018 04:27:53.453Z DEBUG [ END ]  [vic/lib/apiservers/engine/backends.(*System).SystemInfo:88] [147.726575ms] SystemInfo
...
```

```console
$ vic-tail-portlayer
SSH to 192.168.78.127
Warning: Permanently added '192.168.78.127' (ECDSA) to the list of known hosts.
Feb 26 2018 18:46:20.051Z DEBUG [ END ] op=356.49 [vic/lib/portlayer/exec.Commit:34] [277.473256ms] 4128695bc9b68437cc4f56078b280678e22ff0b5930fda2f3750181b3be86411
Feb 27 2018 04:27:53.306Z DEBUG [BEGIN]  [vic/lib/apiservers/portlayer/restapi/handlers.(*ContainersHandlersImpl).GetContainerListHandler:297]
Feb 27 2018 04:27:53.306Z DEBUG [ END ]  [vic/lib/apiservers/portlayer/restapi/handlers.(*ContainersHandlersImpl).GetContainerListHandler:297] [305.935µs]
Feb 27 2018 04:27:53.378Z DEBUG [BEGIN]  [vic/lib/apiservers/portlayer/restapi/handlers.(*StorageHandlersImpl).VolumeStoresList:390] storage_handlers.VolumeStoresList
Feb 27 2018 04:27:53.378Z DEBUG op=356.52: [NewOperation] op=356.52 [vic/lib/apiservers/portlayer/restapi/handlers.(*StorageHandlersImpl).VolumeStoresList:392]
Feb 27 2018 04:27:53.378Z DEBUG [ END ]  [vic/lib/apiservers/portlayer/restapi/handlers.(*StorageHandlersImpl).VolumeStoresList:390] [97.88µs] storage_handlers.VolumeStoresList
Feb 27 2018 04:27:53.433Z DEBUG The VCH stats are: [-1 -1 26 1305624576]
Feb 27 2018 04:27:53.453Z DEBUG The VCH stats are: [2461 13026 26 1305624576]
Feb 27 2018 04:28:18.770Z DEBUG [BEGIN]  [vic/lib/apiservers/portlayer/restapi/handlers.(*ContainersHandlersImpl).GetContainerListHandler:297]
Feb 27 2018 04:28:18.770Z DEBUG [ END ]  [vic/lib/apiservers/portlayer/restapi/handlers.(*ContainersHandlersImpl).GetContainerListHandler:297] [86.957µs]
...
```

#  header-check

Simple header check for CI jobs, currently checks ".go" files only.

This will be called by the CI system (with no args) to perform checking and
fail the job if headers are not correctly set. It can also be called with the
'fix' argument to automatically add headers to the missing files.

Check if headers are fine:
```console
  $ ./infra/scripts/header-check.sh
```
Check and fix headers:
```console
  $ ./infra/scripts/header-check.sh fix
```
