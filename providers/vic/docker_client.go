// Copyright 2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vmware/vic/pkg/trace"
)

// Super simplistic docker client for the virtual kubelet to perform some operations
type DockerClient interface {
	Ping(op trace.Operation) error
	CreateContainer(op trace.Operation, config string) error
	PullImage(op trace.Operation, image string) error
}

type VicDockerClient struct {
	serverAddr string
}

func NewVicDockerClient(personaAddr string) DockerClient {
	return &VicDockerClient{
		serverAddr: personaAddr,
	}
}

func (v *VicDockerClient) Ping(op trace.Operation) error {
	personaServer := fmt.Sprintf("http://%s/v1.35/info", v.serverAddr)
	resp, err := http.Get(personaServer)
	if err != nil {
		op.Errorf("Ping failed: error = %s", err.Error())
		return err
	}

	if resp.StatusCode >= 300 {
		op.Errorf("Ping failed: status = %d", resp.StatusCode)
		return fmt.Errorf("Server Error")
	}

	return nil
}

func (v *VicDockerClient) CreateContainer(op trace.Operation, config string) error {
	personaServer := fmt.Sprintf("http://%s/v1.35/containers/create", v.serverAddr)
	reader := bytes.NewBuffer([]byte(config))
	resp, err := http.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from from docker create: error = %s", err.Error())
		return err
	}
	if resp.StatusCode >= 300 {
		op.Errorf("Error from from docker create: status = %d", resp.StatusCode)
		return fmt.Errorf("Image not found")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	op.Infof("Response from docker create: status = %d", resp.StatusCode)
	op.Infof("Response from docker create: body = %s", string(body))
	var createResp CreateResponse
	err = json.Unmarshal(body, &createResp)
	if err != nil {
		op.Errorf("Failed to unmarshal response from container create post")
		return err
	}
	startContainerUrl := fmt.Sprintf("http://%s/v1.35/containers/%s/start", v.serverAddr, createResp.Id)
	op.Infof("Starting container with request - %s", startContainerUrl)
	_, err = http.Post(startContainerUrl, "", nil)
	if err != nil {
		op.Errorf("Failed to start container %s", createResp.Id)
		return err
	}

	return nil
}

func (v *VicDockerClient) PullImage(op trace.Operation, image string) error {
	pullClient := &http.Client{Timeout: 60 * time.Second}
	personaServer := fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s", v.serverAddr, image)
	op.Infof("POST %s", personaServer)
	reader := bytes.NewBuffer([]byte(""))
	resp, err := pullClient.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from docker pull: error = %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error from docker pull: status = %d", resp.StatusCode)
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading docker pull response: error = %s", err.Error())
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}
	op.Infof("Response from docker pull: body = %s", string(body))

	return nil
}

//------------------------------------
// Utility Functions
//------------------------------------

func DummyCreateSpec(image string, cmd []string) string {
	var command string
	for i, c := range cmd {
		if i == 0 {
			command = fmt.Sprintf("\"%s\"", c)
		} else {
			command = command + fmt.Sprintf(", \"%s\"", c)
		}
	}

	config := `{
			"Hostname":"",
			"Domainname":"",
			"User":"",
			"AttachStdin":false,
			"AttachStdout":false,
			"AttachStderr":false,
			"Tty":false,
			"OpenStdin":false,
			"StdinOnce":false,
			"Env":[

			],
			"Cmd":[
			` + command + `
			],
			"Image":"` + image + `",
			"Volumes":{

		},
		"WorkingDir":"",
		"Entrypoint":null,
		"OnBuild":null,
		"Labels":{

		},
		"HostConfig":{
		"Binds":null,
		"ContainerIDFile":"",
		"LogConfig":{
		"Type":"",
		"Config":{

		}
		},
		"NetworkMode":"default",
		"PortBindings":{

		},
		"RestartPolicy":{
		"Name":"no",
		"MaximumRetryCount":0
		},
		"AutoRemove":false,
		"VolumeDriver":"",
		"VolumesFrom":null,
		"CapAdd":null,
		"CapDrop":null,
		"Dns":[

		],
		"DnsOptions":[

		],
		"DnsSearch":[

		],
		"ExtraHosts":null,
		"GroupAdd":null,
		"IpcMode":"",
		"Cgroup":"",
		"Links":null,
		"OomScoreAdj":0,
		"PidMode":"",
		"Privileged":false,
		"PublishAllPorts":false,
		"ReadonlyRootfs":false,
		"SecurityOpt":null,
		"UTSMode":"",
		"UsernsMode":"",
		"ShmSize":0,
		"ConsoleSize":[
		0,
		0
		],
		"Isolation":"",
		"CpuShares":0,
		"Memory":0,
		"NanoCpus":0,
		"CgroupParent":"",
		"BlkioWeight":0,
		"BlkioWeightDevice":[

		],
		"BlkioDeviceReadBps":null,
		"BlkioDeviceWriteBps":null,
		"BlkioDeviceReadIOps":null,
		"BlkioDeviceWriteIOps":null,
		"CpuPeriod":0,
		"CpuQuota":0,
		"CpuRealtimePeriod":0,
		"CpuRealtimeRuntime":0,
		"CpusetCpus":"",
		"CpusetMems":"",
		"Devices":[

		],
		"DeviceCgroupRules":null,
		"DiskQuota":0,
		"KernelMemory":0,
		"MemoryReservation":0,
		"MemorySwap":0,
		"MemorySwappiness":-1,
		"OomKillDisable":false,
		"PidsLimit":0,
		"Ulimits":null,
		"CpuCount":0,
		"CpuPercent":0,
		"IOMaximumIOps":0,
		"IOMaximumBandwidth":0
		},
		"NetworkingConfig":{
		"EndpointsConfig":{

		}
		}
		}`

	return config
}
