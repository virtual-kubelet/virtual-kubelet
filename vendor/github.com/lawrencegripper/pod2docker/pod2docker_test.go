package pod2docker

import (
	"strings"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	v1resource "k8s.io/apimachinery/pkg/api/resource"
)

func TestPod2DockerVolumeGenerated(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "doesntmatter",
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/shared",
				},
			},
		},
		{
			Name:  "worker",
			Image: "doesntmatter",
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/shared",
				},
			},
		},
	}

	// Todo: Pull this out into a standalone package once stabilized
	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    "examplePodName",
		Volumes: []apiv1.Volume{
			{
				Name: "sharedvolume",
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	// Todo: Very basic smoke test that shared volume path is present in batch command
	if !strings.Contains(podCommand, "/shared") {
		t.Log(podCommand)
		t.Error("Missing shared volume")
	}

	// Todo: Very basic smoke test that shared volume path is present in batch command
	if !strings.Contains(podCommand, "docker volume create examplePodName_sharedvolume") {
		t.Log(podCommand)
		t.Error("Missing shared volume")
	}
}

func TestPod2DockerGeneratesValidOutputEncoding(t *testing.T) {
	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "barry",
			Args:  []string{"encoding"},
		},
		{
			Name:  "worker",
			Image: "marge",
		},
	}

	// Todo: Pull this out into a standalone package once stabilized
	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    "examplePodName",
		Volumes:    nil,
	})

	if err != nil {
		t.Error(err)
	}

	if strings.Contains(podCommand, "&lt;") {
		t.Error("output contains incorrect encoding")
	}
}

func TestPod2DockerRuntimeSetToNvidia(t *testing.T) {
	//This is a very basic test.
	//Without mandating all dev/build machines have a gpu this is the best I can do
	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "barry",
			Args:  []string{"encoding"},
			Resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					"nvidia.com/gpu": *v1resource.NewQuantity(1, v1resource.DecimalSI),
				},
			},
		},
		{
			Name:  "worker",
			Image: "marge",
		},
	}

	// Todo: Pull this out into a standalone package once stabilized
	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    "examplePodName",
		Volumes:    nil,
	})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(podCommand, "--runtime nvidia") {
		t.Error("output doesn't contain nvidia runtime")
	}
}

func TestPod2DockerPullAlways(t *testing.T) {
	containers := []apiv1.Container{
		{
			Name:            "sidecar",
			Image:           "barry",
			Args:            []string{"encoding"},
			ImagePullPolicy: apiv1.PullAlways,
		},
		{
			Name:  "worker",
			Image: "marge",
		},
	}

	// Todo: Pull this out into a standalone package once stabilized
	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    "examplePodName",
		Volumes:    nil,
	})

	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(podCommand, "docker pull barry") {
		t.Error("docker pull command is missing")
	}
}
