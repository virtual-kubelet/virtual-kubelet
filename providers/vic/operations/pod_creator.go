package operations

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kr/pretty"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	vicerrors "github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type VicPodCreatorError string

func (e VicPodCreatorError) Error() string { return string(e) }
func NewPodCreatorPullError(image, msg string) VicPodCreatorError {
	return VicPodCreatorError(fmt.Sprintf("VicPodCreator failed to get image %s's config from the image store: %s", image, msg))
}
func NewPodCreatorNilImgConfigError(image string) VicPodCreatorError {
	return VicPodCreatorError(fmt.Sprintf("VicPodCreator failed to get image %s's config from the image store", image))
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

	// Errors
	PodCreatorPortlayerClientError = VicPodCreatorError("PodCreator called with an invalid portlayer client")
	PodCreatorImageStoreError      = VicPodCreatorError("PodCreator called with an invalid image store")
	PodCreatorIsolationProxyError  = VicPodCreatorError("PodCreator called with an invalid isolation proxy")
	PodCreatorPodCacheError        = VicPodCreatorError("PodCreator called with an invalid pod cache")
	PodCreatorPersonaAddrError     = VicPodCreatorError("PodCreator called with an invalid VIC persona addr")
	PodCreatorPortlayerAddrError   = VicPodCreatorError("PodCreator called with an invalid VIC portlayer addr")
	PodCreatorInvalidPodSpecError  = VicPodCreatorError("CreatePod called with nil pod")
	PodCreatorInvalidArgsError     = VicPodCreatorError("Invalid arguments")
)

func NewPodCreator(client *client.PortLayer, imageStore proxy.ImageStore, isolationProxy proxy.IsolationProxy, podCache cache.PodCache, personaAddr string, portlayerAddr string) (PodCreator, error) {
	if client == nil {
		return nil, PodCreatorPortlayerClientError
	} else if imageStore == nil {
		return nil, PodCreatorImageStoreError
	} else if isolationProxy == nil {
		return nil, PodCreatorIsolationProxyError
	} else if podCache == nil {
		return nil, PodCreatorPodCacheError
	}

	return &VicPodCreator{
		client:         client,
		imageStore:     imageStore,
		podCache:       podCache,
		personaAddr:    personaAddr,
		portlayerAddr:  portlayerAddr,
		isolationProxy: isolationProxy,
	}, nil
}

// CreatePod creates the pod and potentially start it
//
// arguments:
//		op		operation trace logger
//		pod		pod spec
//		start	start the pod after creation
// returns:
// 		error
func (v *VicPodCreator) CreatePod(op trace.Operation, pod *v1.Pod, start bool) error {
	defer trace.End(trace.Begin("", op))

	if pod == nil {
		op.Errorf(PodCreatorInvalidPodSpecError.Error())
		return PodCreatorInvalidPodSpecError
	}

	defer trace.End(trace.Begin(pod.Name, op))

	// Pull all containers simultaneously
	err := v.pullPodContainers(op, pod)
	if err != nil {
		op.Errorf("PodCreator failed to pull containers: %s", err.Error())
		return err
	}

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

	err = v.podCache.Add(op, "", pod.Name, vp)
	if err != nil {
		//TODO:  What should we do if pod already exist?
	}

	if start {
		ps, err := NewPodStarter(v.client, v.isolationProxy)
		if err != nil {
			op.Errorf("Error creating pod starter: %s", err.Error())
			return err
		}

		err = ps.Start(op, id, pod.Name)
		if err != nil {
			return err
		}
		now := metav1.NewTime(time.Now())
		vp.Pod.Status.StartTime = &now
	}

	return nil
}

// pullPodContainers simultaneously pulls all containers in a pod
//
// arguments:
//		op		operation trace logger
//		pod		pod spec
// returns:
//		error
func (v *VicPodCreator) pullPodContainers(op trace.Operation, pod *v1.Pod) error {
	defer trace.End(trace.Begin("", op))

	if pod == nil || pod.Spec.Containers == nil {
		return PodCreatorInvalidPodSpecError
	}

	var pullGroup sync.WaitGroup

	errChan := make(chan error, 2)

	for _, c := range pod.Spec.Containers {
		pullGroup.Add(1)

		go func(img string, policy v1.PullPolicy) {
			defer pullGroup.Done()

			// Pull image config from VIC's image store if policy allows
			var realize bool
			if policy == v1.PullIfNotPresent {
				realize = true
			} else {
				realize = false
			}

			_, err := v.imageStore.Get(op, img, "", realize)
			if err != nil {
				err = fmt.Errorf("VicPodCreator failed to get image %s's config from the image store: %s", img, err.Error())
				op.Error(err)
				errChan <- err
			}
		}(c.Image, c.ImagePullPolicy)
	}

	pullGroup.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// createPod creates a pod using the VIC portlayer.  Images can be pulled serially if not already present.
//
// arguments:
//		op		operation trace logger
//		pod		pod spec
//		start	start the pod after creation
// returns:
// 		(pod id, error)
func (v *VicPodCreator) createPod(op trace.Operation, pod *v1.Pod, start bool) (string, error) {
	defer trace.End(trace.Begin("", op))

	if pod == nil || pod.Spec.Containers == nil {
		op.Errorf(PodCreatorInvalidPodSpecError.Error())
		return "", PodCreatorInvalidPodSpecError
	}

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
			err = NewPodCreatorPullError(c.Image, err.Error())
			op.Error(err)
			return "", err
		}
		if imgConfig == nil {
			err = NewPodCreatorNilImgConfigError(c.Image)
			op.Error(err)
			return "", err
		}

		op.Debugf("Receive image config from imagestore = %# v", pretty.Formatter(imgConfig))

		// Create the initial config
		ic, err := IsolationContainerConfigFromKubeContainer(op, &c, imgConfig, pod)
		if err != nil {
			return "", err
		}

		op.Debugf("isolation config %# v", pretty.Formatter(ic))

		h, err = v.isolationProxy.AddImageToHandle(op, h, c.Name, imgConfig.V1Image.ID, imgConfig.ImageID, imgConfig.Name)
		if err != nil {
			return "", err
		}

		//TODO: We need one task with the container ID as the portlayer uses this to track session.  Longer term, we should figure out
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

	op.Debugf("Created Pod: %s, Handle: %s, ID: %s", pod.Name, h, id)

	return id, nil
}

//------------------------------------
// Utility Functions
//------------------------------------

func IsolationContainerConfigFromKubeContainer(op trace.Operation, cSpec *v1.Container, imgConfig *metadata.ImageConfig, pod *v1.Pod) (proxy.IsolationContainerConfig, error) {
	defer trace.End(trace.Begin("", op))

	if cSpec == nil || imgConfig == nil || pod == nil {
		op.Errorf("Invalid args to IsolationContainerConfigFromKubeContainer: cSpec(%#v), imgConfig(%#v), pod(%#v)", cSpec, imgConfig, pod)
		return proxy.IsolationContainerConfig{}, PodCreatorInvalidArgsError
	}

	op.Debugf("** IsolationContainerConfig... imgConfig = %#v", imgConfig)
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
	} else if imgConfig.Config != nil {
		config.Cmd = make([]string, len(imgConfig.Config.Cmd))
		copy(config.Cmd, imgConfig.Config.Cmd)
	}

	config.User = ""
	if imgConfig.Config != nil {
		if imgConfig.Config.User != "" {
			config.User = imgConfig.Config.User
		}

		// set up environment
		config.Env = setEnvFromImageConfig(config.Tty, config.Env, imgConfig.Config.Env)
	}

	op.Debugf("config = %#v", config)

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
		return vicerrors.BadRequestError("invalid config")
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
	op.Debugf("Container memory: %d MB", config.Memory)

	return nil
}
