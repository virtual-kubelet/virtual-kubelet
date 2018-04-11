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
	"fmt"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"

	"k8s.io/api/core/v1"
)

type PodCreator interface {
	CreatePod(op trace.Operation, pod *v1.Pod, start bool) error
}

type VicPodCreator struct {
	client         *client.PortLayer
	imageStore     proxy.ImageStore
	isolationProxy proxy.IsolationProxy
	podCache       cache.PodCache
	personaAddr    string
	portlayerAddr  string
}

type CreateResponse struct {
	Id       string `json:"Id"`
	Warnings string `json:"Warnings"`
}

const (
	// MemoryAlignMB is the value to which container VM memory must align in order for hotadd to work
	MemoryAlignMB = 128
	// MemoryMinMB - the minimum allowable container memory size
	MemoryMinMB = 512
	// MemoryDefaultMB - the default container VM memory size
	MemoryDefaultMB = 2048
	// MinCPUs - the minimum number of allowable CPUs the container can use
	MinCPUs = 1
	// DefaultCPUs - the default number of container VM CPUs
	DefaultCPUs   = 2
	DefaultMemory = 512
	MiBytesUnit   = 1024 * 1024

	defaultEnvPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

func NewPodCreator(client *client.PortLayer, imageStore proxy.ImageStore, isolationProxy proxy.IsolationProxy, podCache cache.PodCache, personaAddr string, portlayerAddr string) *VicPodCreator {
	return &VicPodCreator{
		client:         client,
		imageStore:     imageStore,
		podCache:       podCache,
		personaAddr:    personaAddr,
		portlayerAddr:  portlayerAddr,
		isolationProxy: isolationProxy,
	}
}

func (v *VicPodCreator) CreatePod(op trace.Operation, pod *v1.Pod, start bool) error {
	defer trace.End(trace.Begin(pod.Name, op))

	// Transform kube container config to docker create config
	id, err := v.createPod(op, pod, start)
	if err != nil {
		op.Errorf("pod_creator failed to create pod: %s", err.Error())
		return err
	}

	vp := &vicpod.VicPod{
		ID:  id,
		Pod: pod.DeepCopy(),
	}

	err = v.podCache.Add(op, pod.Name, vp)
	if err != nil {
		//TODO:  What should we do if pod already exist?
	}

	if start {
		ps := NewPodStarter(v.client, v.isolationProxy)
		err := ps.Start(op, id, pod.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// portlayerCreatePod creates a pod using the VIC portlayer.
//
//	returns id of pod as a string and error
func (v *VicPodCreator) createPod(op trace.Operation, pod *v1.Pod, start bool) (string, error) {
	defer trace.End(trace.Begin("", op))

	//ip := proxy.NewIsolationProxy(v.client, v.portlayerAddr, v.imageStore, v.podCache)

	id, h, err := v.isolationProxy.CreateHandle(op)
	if err != nil {
		return "", err
	}

	for idx, c := range pod.Spec.Containers {
		// Pull image config from VIC's image store if policy allows
		var realize bool
		if c.ImagePullPolicy == v1.PullIfNotPresent {
			realize = true
		} else {
			realize = false
		}

		imgConfig, err := v.imageStore.Get(op, c.Image, "", realize)
		if err != nil {
			err = fmt.Errorf("VicPodCreator failed to get image %s's config from the image store: %s", c.Image, err.Error())
			op.Error(err)
			return "", err
		}
		if imgConfig == nil {
			err = fmt.Errorf("VicPodCreator failed to get image %s's config from the image store", c.Image)
			op.Error(err)
			return "", err
		}

		op.Info("** Receive image config from imagestore = %#v", imgConfig)

		// Create the initial config
		ic, err := IsolationContainerConfigFromKubeContainer(op, &c, imgConfig, pod)
		if err != nil {
			return "", err
		}

		op.Infof("isolation config %#v", imgConfig)

		h, err = v.isolationProxy.AddImageToHandle(op, h, c.Name, imgConfig.V1Image.ID, imgConfig.ImageID, imgConfig.Name)
		if err != nil {
			return "", err
		}

		//TODO: Fix this!
		//HACK: We need one task with the container ID as the portlayer uses this to track session.  Longer term, we should figure out
		//	a way to fix this in the portlayer?
		if idx == 0 {
			h, err = v.isolationProxy.CreateHandleTask(op, h, id, imgConfig.V1Image.ID, ic)
		} else {
			h, err = v.isolationProxy.CreateHandleTask(op, h, fmt.Sprintf("Container-%d-task", idx), imgConfig.V1Image.ID, ic)
		}
		if err != nil {
			return "", err
		}

		h, err = v.isolationProxy.AddHandleToScope(op, h, ic)
		if err != nil {
			return id, err
		}
	}

	// Need both interaction and logging added or we will not be able to retrieve output.log or tether.debug
	h, err = v.isolationProxy.AddInteractionToHandle(op, h)
	if err != nil {
		return "", err
	}

	h, err = v.isolationProxy.AddLoggingToHandle(op, h)
	if err != nil {
		return "", err
	}

	err = v.isolationProxy.CommitHandle(op, h, id, -1)
	if err != nil {
		return "", err
	}

	op.Infof("Created Pod: %s, Handle: %s, ID: %s", pod.Name, h, id)

	return id, nil
}

//------------------------------------
// Utility Functions
//------------------------------------

func IsolationContainerConfigFromKubeContainer(op trace.Operation, cSpec *v1.Container, imgConfig *metadata.ImageConfig, pod *v1.Pod) (proxy.IsolationContainerConfig, error) {
	defer trace.End(trace.Begin("", op))

	op.Infof("** IsolationContainerConfig... imgConfig = %#v", imgConfig)
	config := proxy.IsolationContainerConfig{
		Name:       cSpec.Name,
		WorkingDir: cSpec.WorkingDir,
		ImageName:  cSpec.Image,
		Tty:        cSpec.TTY,
		StdinOnce:  cSpec.StdinOnce,
		OpenStdin:  cSpec.Stdin,
		PortMap:    make(map[string]proxy.PortBinding, 0),
	}

	setResourceFromKubeSpec(op, &config, cSpec)

	// Overwrite or append the image's config from the CLI with the metadata from the image's
	// layer metadata where appropriate
	if len(cSpec.Command) > 0 {
		config.Cmd = make([]string, len(cSpec.Command))
		copy(config.Cmd, cSpec.Command)

		config.Cmd = append(config.Cmd, cSpec.Args...)
	} else {
		config.Cmd = make([]string, len(imgConfig.Config.Cmd))
		copy(config.Cmd, imgConfig.Config.Cmd)
	}

	config.User = ""
	if imgConfig.Config.User != "" {
		config.User = imgConfig.Config.User
	}

	// set up environment
	config.Env = setEnvFromImageConfig(config.Tty, config.Env, imgConfig.Config.Env)

	op.Infof("config = %#v", config)

	// TODO:  Cache the container (so that they are shared with the persona)

	return config, nil
}

func setEnvFromImageConfig(tty bool, env []string, imgEnv []string) []string {
	// Set PATH in ENV if needed
	env = setPathFromImageConfig(env, imgEnv)

	containerEnv := make(map[string]string, len(env))
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		var val string
		if len(kv) == 2 {
			val = kv[1]
		}
		containerEnv[kv[0]] = val
	}

	// Set TERM to xterm if tty is set, unless user supplied a different TERM
	if tty {
		if _, ok := containerEnv["TERM"]; !ok {
			env = append(env, "TERM=xterm")
		}
	}

	// add remaining environment variables from the image config to the container
	// config, taking care not to overwrite anything
	for _, imageEnv := range imgEnv {
		key := strings.SplitN(imageEnv, "=", 2)[0]
		// is environment variable already set in container config?
		if _, ok := containerEnv[key]; !ok {
			// no? let's copy it from the image config
			env = append(env, imageEnv)
		}
	}

	return env
}

func setPathFromImageConfig(env []string, imgEnv []string) []string {
	// check if user supplied PATH environment variable at creation time
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			// a PATH is set, bail
			return env
		}
	}

	// check to see if the image this container is created from supplies a PATH
	for _, v := range imgEnv {
		if strings.HasPrefix(v, "PATH=") {
			// a PATH was found, add it to the config
			env = append(env, v)
			return env
		}
	}

	// no PATH set, use the default
	env = append(env, fmt.Sprintf("PATH=%s", defaultEnvPath))

	return env
}

func setResourceFromKubeSpec(op trace.Operation, config *proxy.IsolationContainerConfig, cSpec *v1.Container) error {
	if config == nil {
		return errors.BadRequestError("invalid config")
	}

	// Get resource request.  If not specified, use the limits.  If that's not set, use default VIC values.
	config.CPUCount = cSpec.Resources.Requests.Cpu().Value()
	if config.CPUCount == 0 {
		config.CPUCount = cSpec.Resources.Limits.Cpu().Value()
		if config.CPUCount == 0 {
			config.CPUCount = DefaultCPUs
		}
	}
	config.Memory = cSpec.Resources.Requests.Memory().Value()
	if config.Memory == 0 {
		config.Memory = cSpec.Resources.Limits.Memory().Value()
		if config.Memory == 0 {
			config.Memory = DefaultMemory
		}
	}

	// convert from bytes to MiB for vsphere
	memoryMB := config.Memory / MiBytesUnit
	if memoryMB == 0 {
		memoryMB = MemoryDefaultMB
	} else if memoryMB < MemoryMinMB {
		memoryMB = MemoryMinMB
	}

	// check that memory is aligned
	if remainder := memoryMB % MemoryAlignMB; remainder != 0 {
		op.Warnf("Default container VM memory must be %d aligned for hotadd, rounding up.", MemoryAlignMB)
		memoryMB += MemoryAlignMB - remainder
	}

	config.Memory = memoryMB
	op.Infof("Container memory: %d MB", config.Memory)

	return nil
}
