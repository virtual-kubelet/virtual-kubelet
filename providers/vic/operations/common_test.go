package operations

import (
	"context"
	"testing"

	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"

	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"
	proxymocks "github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy/mocks"
)

var (
	pod              v1.Pod
	imgConfig        metadata.ImageConfig
	busyboxIsoConfig proxy.IsolationContainerConfig
	alpineIsoConfig  proxy.IsolationContainerConfig
	vicPod           vicpod.VicPod
)

const (
	podID     = "123"
	podName   = "busybox-sleep"
	podHandle = "fakehandle"

	fakeEP        = "fake-endpoint"
	stateRunning  = "Running"
	stateStarting = "Starting"
	stateStopping = "Stopping"
	stateStopped  = "Stopped"
	stateRemoving = "Removing"
	stateRemoved  = "Removed"
)

func createMocks(t *testing.T) (*proxymocks.ImageStore, *proxymocks.IsolationProxy, cache.PodCache, trace.Operation) {
	store := &proxymocks.ImageStore{}
	ip := &proxymocks.IsolationProxy{}
	cache := cache.NewVicPodCache()
	op := trace.NewOperation(context.Background(), "tests")

	return store, ip, cache, op
}

func fakeError(myErr string) error {
	return fmt.Errorf("fake error: %s", myErr)
}

func initPod() {
	pod = v1.Pod{
		//TypeMeta: v1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       podName,
			GenerateName:               "",
			Namespace:                  "default",
			SelfLink:                   "/api/v1/namespaces/default/pods/busybox-sleep",
			UID:                        "b1fc6e1b-499b-11e8-946c-000c29479092",
			ResourceVersion:            "10338145",
			Generation:                 0,
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			Labels:          map[string]string{},
			Annotations:     map[string]string{},
			OwnerReferences: nil,
			Initializers:    nil,
			Finalizers:      nil,
			ClusterName:     "",
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "default-token-9q9lr",
					VolumeSource: v1.VolumeSource{
						HostPath:             nil,
						EmptyDir:             nil,
						GCEPersistentDisk:    nil,
						AWSElasticBlockStore: nil,
						GitRepo:              nil,
						Secret: &v1.SecretVolumeSource{
							SecretName: "default-token-9q9lr",
							Items:      nil,
							Optional:   nil,
						},
						NFS:                   nil,
						ISCSI:                 nil,
						Glusterfs:             nil,
						PersistentVolumeClaim: nil,
						RBD:                  nil,
						FlexVolume:           nil,
						Cinder:               nil,
						CephFS:               nil,
						Flocker:              nil,
						DownwardAPI:          nil,
						FC:                   nil,
						AzureFile:            nil,
						ConfigMap:            nil,
						VsphereVolume:        nil,
						Quobyte:              nil,
						AzureDisk:            nil,
						PhotonPersistentDisk: nil,
						Projected:            nil,
						PortworxVolume:       nil,
						ScaleIO:              nil,
						StorageOS:            nil,
					},
				},
			},
			InitContainers: nil,
			Containers: []v1.Container{
				{
					Name:       "busybox-container",
					Image:      "busybox",
					Command:    []string{"/bin/sleep"},
					Args:       []string{"2m"},
					WorkingDir: "",
					Ports:      nil,
					EnvFrom:    nil,
					Env:        nil,
					Resources:  v1.ResourceRequirements{},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "default-token-9q9lr",
							ReadOnly:         true,
							MountPath:        "/var/run/secrets/kubernetes.io/serviceaccount",
							SubPath:          "",
							MountPropagation: nil,
						},
					},
					LivenessProbe:            nil,
					ReadinessProbe:           nil,
					Lifecycle:                nil,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: "File",
					ImagePullPolicy:          "IfNotPresent",
					SecurityContext:          nil,
					Stdin:                    false,
					StdinOnce:                false,
					TTY:                      false,
				},
				{
					Name:       "alpine-container",
					Image:      "alpine",
					Command:    nil,
					Args:       nil,
					WorkingDir: "",
					Ports:      nil,
					EnvFrom:    nil,
					Env:        nil,
					Resources:  v1.ResourceRequirements{},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "default-token-9q9lr",
							ReadOnly:         true,
							MountPath:        "/var/run/secrets/kubernetes.io/serviceaccount",
							SubPath:          "",
							MountPropagation: nil,
						},
					},
					LivenessProbe:            nil,
					ReadinessProbe:           nil,
					Lifecycle:                nil,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: "File",
					ImagePullPolicy:          "IfNotPresent",
					SecurityContext:          nil,
					Stdin:                    false,
					StdinOnce:                false,
					TTY:                      false,
				},
			},
			RestartPolicy:                 "Always",
			TerminationGracePeriodSeconds: new(int64),
			ActiveDeadlineSeconds:         nil,
			DNSPolicy:                     "ClusterFirst",
			NodeSelector:                  map[string]string{"affinity": "vmware"},
			ServiceAccountName:            "default",
			DeprecatedServiceAccount:      "default",
			AutomountServiceAccountToken:  nil,
			NodeName:                      "vic-kubelet",
			HostNetwork:                   false,
			HostPID:                       false,
			HostIPC:                       false,
			SecurityContext:               &v1.PodSecurityContext{},
			ImagePullSecrets:              nil,
			Hostname:                      "",
			Subdomain:                     "",
			Affinity:                      nil,
			SchedulerName:                 "default-scheduler",
			Tolerations: []v1.Toleration{
				{
					Key:               "node.kubernetes.io/not-ready",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoExecute",
					TolerationSeconds: new(int64),
				},
				{
					Key:               "node.kubernetes.io/unreachable",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoExecute",
					TolerationSeconds: new(int64),
				},
			},
			HostAliases:       nil,
			PriorityClassName: "",
			Priority:          nil,
		},
	}

	busyboxIsoConfig = proxy.IsolationContainerConfig{
		ID:         "",
		ImageID:    "",
		LayerID:    "",
		ImageName:  "busybox",
		Name:       "busybox-container",
		Namespace:  "",
		Cmd:        []string{"/bin/sleep", "2m"},
		Path:       "",
		Entrypoint: nil,
		Env:        nil,
		WorkingDir: "",
		User:       "",
		StopSignal: "",
		Attach:     false,
		StdinOnce:  false,
		OpenStdin:  false,
		Tty:        false,
		CPUCount:   2,
		Memory:     2048,
		PortMap:    map[string]proxy.PortBinding{},
	}

	alpineIsoConfig = proxy.IsolationContainerConfig{
		ID:         "",
		ImageID:    "",
		LayerID:    "",
		ImageName:  "alpine",
		Name:       "alpine-container",
		Namespace:  "",
		Cmd:        nil,
		Path:       "",
		Entrypoint: nil,
		Env:        nil,
		WorkingDir: "",
		User:       "",
		StopSignal: "",
		Attach:     false,
		StdinOnce:  false,
		OpenStdin:  false,
		Tty:        false,
		CPUCount:   2,
		Memory:     2048,
		PortMap:    map[string]proxy.PortBinding{},
	}

	vicPod = vicpod.VicPod{
		ID:  podID,
		Pod: &pod,
	}
}
