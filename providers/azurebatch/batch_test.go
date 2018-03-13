package azurebatch

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
)

func TestBatchBashGenerator(t *testing.T) {

	templateVars := BatchPodComponents{
		Containers: []v1.Container{
			v1.Container{
				Name:  "testName",
				Image: "busybox",
				Env: []v1.EnvVar{
					v1.EnvVar{
						Name:  "1",
						Value: "1",
					},
					v1.EnvVar{
						Name:  "2",
						Value: "2",
					},
				},
				Command: []string{
					"sleep",
				},
				Args: []string{
					`15`,
				},
				VolumeMounts: []v1.VolumeMount{
					v1.VolumeMount{
						Name:      "emptyDirVol",
						MountPath: "/emptyVol",
					},
					v1.VolumeMount{
						Name:      "hostVol",
						MountPath: "/hostVol",
					},
				},
			},
		},
		PullCredentials: []aci.ImageRegistryCredential{
			aci.ImageRegistryCredential{
				Username: "lawrencegripper",
				Password: "testing",
				Server:   "server",
			},
		},
		PodName: "Barry",
		TaskID:  "barryid",
		Volumes: []v1.Volume{
			v1.Volume{
				Name: "emptyDirVol",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
			v1.Volume{
				Name: "hostVol",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/home",
					},
				},
			},
		},
	}

	podCommand, err := getPodCommand(templateVars)
	t.Log(podCommand)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

}
