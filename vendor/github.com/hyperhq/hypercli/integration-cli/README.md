Integration test for hyper cli
==================================

Functional test for hyper cli.  
> use apirouter service on packet(dev env) or zenlayer(online env) as backend.

<!-- TOC depthFrom:1 depthTo:6 withLinks:1 updateOnSave:1 orderedList:0 -->

- [Project status](#project-status)
	- [cli test case](#cli-test-case)
	- [api test case](#api-test-case)
	- [extra](#extra)
	- [skip](#skip)
- [Test case name](#test-case-name)
- [Command list](#command-list)
	- [hyper only](#hyper-only)
	- [both](#both)
	- [docker only](#docker-only)
- [Run test case](#run-test-case)
	- [Clone hypercli repo](#clone-hypercli-repo)
	- [Run test in docker container for dev](#run-test-in-docker-container-for-dev)
		- [build docker image for dev](#build-docker-image-for-dev)
		- [make hyper cli](#make-hyper-cli)
		- [enter container](#enter-container)
		- [run test in container](#run-test-in-container)
	- [Run test on localhost for dev](#run-test-on-localhost-for-dev)
		- [prepare](#prepare)
		- [run test case](#run-test-case)
	- [Run test in docker container for qa](#run-test-in-docker-container-for-qa)
		- [build docker image for qa](#build-docker-image-for-qa)
		- [run test in docker container](#run-test-in-docker-container)
			- [run test via util.sh](#run-test-via-utilsh)
			- [run test via docker cli](#run-test-via-docker-cli)
			- [run test via hyper cli](#run-test-via-hyper-cli)

<!-- /TOC -->


# Project status

## cli test case

- [ ] cli_attach_test
- [ ] cli_attach_unix_test
- [x] cli_config_test
- [x] cli_create_test
- [x] cli_commit_test
- [x] cli_exec_test
- [x] cli_exec_unix_test
- [x] cli_fip_test
- [x] cli_help_test
- [x] cli_history_test
- [x] cli_images_test
- [x] cli_info_test
- [x] cli_inspect_experimental_test
- [x] cli_inspect_test
- [x] cli_kill_test
- [x] cli_links_test
- [ ] cli_links_unix_test
- [x] cli_load_basic_test
- [x] cli_load_large_test
- [x] cli_load_legacy_test
- [x] cli_load_local_test
- [x] cli_login_test
- [x] cli_logs_test
- [x] cli_port_test
- [x] cli_ps_test
- [x] cli_pull_test
- [x] cli_push_test
- [x] cli_region_test
- [x] cli_rename_test
- [x] cli_restart_test
- [x] cli_rm_test
- [x] cli_rmi_test
- [x] cli_run_test
- [x] cli_run_unix_test
- [x] cli_search_test
- [x] cli_share_volume_test
- [x] cli_snapshot_test
- [x] cli_start_test
- [ ] cli_stats_test
- [x] cli_version_test
- [x] cli_volume_test


## api test case

- [ ] api_attach_test
- [ ] api_containers_test
- [x] api_create_test
- [x] api_exec_test
- [x] api_exec_resize_test
- [x] api_images_test
- [x] api_info_test
- [x] api_inspect_test
- [x] api_logs_test
- [x] api_stats_test
- [x] api_snapshots_test
- [x] api_version_test
- [x] api_volumes_test


## extra

[Extra Test Case](EXTRA_TEST.md)


## skip

> not support build, tag

- [ ] cli_authz_unix_test
- [ ] cli_build_test
- [ ] cli_build_unix_test
- [ ] cli_by_digest_test
- [ ] cli_cp_from_container_test
- [ ] cli_cp_test
- [ ] cli_cp_to_container_test
- [ ] cli_cp_utils
- [ ] cli_daemon_test
- [ ] cli_diff_test
- [ ] cli_events_test
- [ ] cli_events_unix_test
- [ ] cli_experimental_test
- [ ] cli_export_import_test
- [ ] cli_external_graphdriver_unix_test
- [ ] cli_import_test
- [ ] cli_nat_test
- [ ] cli_netmode_test
- [ ] cli_network_unix_test
- [ ] cli_oom_killed_test
- [ ] cli_pause_test
- [ ] cli_proxy_test
- [ ] cli_pull_local_test
- [ ] cli_pull_trusted_test
- [ ] cli_save_load_test
- [ ] cli_save_load_unix_test
- [ ] cli_sni_test
- [ ] cli_start_volume_driver_unix_test
- [ ] cli_tag_test
- [ ] cli_top_test
- [ ] cli_update_unix_test
- [ ] cli_v2_only_test
- [ ] cli_volume_driver_compat_unix_test
- [ ] cli_wait_test


# Test case name

- TestCliConfig
- TestCliCreate
- TestCliCommit
- TestCliExec
- TestCliFip
- TestCliHelp
- TestCliHistory
- TestCliInfo
- TestCliInspect
- TestCliKill
- TestCliLinks
- TestCliLoadFromUrl
- TestCliLoadFromLocal
- TestCliLogin
- TestCliLogs
- TestCliPort
- TestCliPs
- TestCliPull
- TestCliPush
- TestCliRename
- TestCliRestart
- TestCliRmi
- TestCliRm
- TestCliRun
- TestCliSearch
- TestCliSnapshot
- TestCliStart
- TestCliVersion
- TestCliVolume


# Command list

## hyper only
```
config  fip     snapshot
```

## both
```
attach	create	exec	history	images
info    inspect	kill    login   logout
logs    port    ps      pull    rename
restart rm      rmi     run     search
start   stats   stop    version volume
```

## docker only

> not support for hyper currently

```
build   commit  cp      diff    events
export  import  load    network pause
push    save    tag     top     unpause
update  wait
```

# Run test case

## Clone hypercli repo
```
$ git clone https://github.com/hyperhq/hypercli.git
```

## Run test in docker container for dev

### build docker image for dev

> build docker image in host OS
> Use `CentOS` as test env
> Image name is `hyperhq/hypercli-auto-test:dev`

```
// run in dir hypercli/integration-cli on host os
$ ./util.sh build-dev
```

### make hyper cli

> build hyper cli binary from source code

```
// run in dir hypercli/integration-cli on host os
$ ./util.sh make
```

### enter container

> update `ACCESS_KEY` and `SECRET_KEY` in `integration-cli/util.conf`

```
// run in dir hypercli/integration-cli on host os
$ ./util.sh enter
```

### run test in container
```
$ ./util.sh test all
```

## Run test on localhost for dev

### prepare

```
// ensure hyperhq and docker dir
mkdir -p $GOPATH/src/github.com/{hyperhq,docker}

// clone and build hypercli
cd $GOPATH/src/github.com/hyperhq
git clone git@github.com:hyperhq/hypercli.git
cd hypercli
./build.sh

// copy hyper binary to /usr/bin/hyper
sudo cp hyper/hyper /usr/bin/hyper

// create link
cd $GOPATH/src/github.com/docker
ln -s ../hyperhq/hypercli docker

// generate util.conf
$ cd $GOPATH/src/github.com/hyperhq/hypercli
$ git checkout integration-test
$ cd integration-cli
$ ./util.sh

// config util.conf
$ vi util.conf
ACCESS_KEY="<hyper access key>"
SECRET_KEY="<hyper secret key>"
```

### run test case

```
// run all test cases
$ ./util.sh test all

// run test case with timeout
$ ./util.sh test all -timout 20m

// run specified test case
$ ./util.sh test -check.f ^TestCliLoadFromLocalTar$

// run test cases start with specified prefix
$ ./util.sh test -check.f TestCliLoadFromLocalTar

// combined use
$ ./util.sh test -check.f 'TestCliLoadFromLocalTarEmpty|TestCliLoadFromLocalPullAndLoad' -timeout 20m
```

## Run test in docker container for qa

### build docker image for qa

> build docker image in host OS
> Use `CentOS` as test env
> Image name is `hyperhq/hypercli-auto-test:qa`

```
// run in dir hypercli/integration-cli on host os
$ ./util.sh build-qa
```

### run test in docker container

> run all test case for qa

#### run test via util.sh
```
$ cd integration-cli

//test master branch
$ ./util.sh qa

//test integration-test branch
$ ./util.sh qa integration-test

//test PR
$ ./util.sh qa "#222"
```

#### run test via docker cli

Required parameters:

`APIROUTER`: apirouter entrypoint  
`REGION`: could be us-west-1(zl2), eu-central-1(eu1), RegionOne(packet)  
`ACCESS_KEY`,`SECRET_KEY`: Hyper credential for test  
`BRANCH`: hyper cli branch name or PR number  

```
//test `master` branch of hypercli with `eu-west-1` apirouter
$ docker run -it --rm \
    -e ACCESS_KEY=${ACCESS_KEY} \
    -e SECRET_KEY=${SECRET_KEY} \
    -e DOCKER_HOST=tcp://us-west-1.hyper.sh:443 \
    -e REGION=us-west-1 \
    -e BRANCH=master \
    hyperhq/hypercli-auto-test:qa go test -check.f TestCli -timeout 180m

//test `specified PR`
$ docker run -it --rm \
    -e DOCKER_HOST=${APIROUTER} \
    -e REGION=${REGION} \
    -e ACCESS_KEY=${ACCESS_KEY} \
    -e SECRET_KEY=${SECRET_KEY} \
    -e BRANCH="#221" \
    hyperhq/hypercli-auto-test:qa go test -check.f TestCli -timeout 180m


//test `specified case name`
$ docker run -it --rm \
    -e DOCKER_HOST=${APIROUTER} \
    -e REGION=${REGION} \
    -e ACCESS_KEY="${ACCESS_KEY}" \
    -e SECRET_KEY="${SECRET_KEY}" \
    -e BRANCH=${BRANCH} \
    hyperhq/hypercli-auto-test:qa go test -check.f 'TestCliInfo|TestCliFip' -timeout 180m


//test with `packet` apirouter
$ docker run -it --rm \
    -e ACCESS_KEY=${ACCESS_KEY} \
    -e SECRET_KEY=${SECRET_KEY} \
    -e BRANCH=${BRANCH} \
    -e DOCKER_HOST=tcp://147.75.x.x:6443 \
    -e REGION=RegionOne \
    hyperhq/hypercli-auto-test:qa go test -check.f TestCli -timeout 180m


//test with http proxy
$ docker run -it --rm \
    -e DOCKER_HOST=${APIROUTER} \
    -e REGION=${REGION} \
    -e ACCESS_KEY=${ACCESS_KEY} \
    -e SECRET_KEY=${SECRET_KEY} \
    -e BRANCH=${BRANCH} \
    -e http_proxy=${http_proxy} \
    -e https_proxy=${https_proxy} \
    hyperhq/hypercli-auto-test:qa go test -check.f TestCliInfo
    

//test basic test case only
$ docker run -it --rm \
    -e DOCKER_HOST=${APIROUTER} \
    -e REGION=${REGION} \
    -e ACCESS_KEY=${ACCESS_KEY} \
    -e SECRET_KEY=${SECRET_KEY} \
    -e BRANCH=${BRANCH} \
    hyperhq/hypercli-auto-test:qa go test -check.f "TestCli.*Basic" -timeout 180m

```

#### run test via hyper cli

Just replace `docker` with `hyper` in command line.
