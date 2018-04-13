package pod2docker

import (
	"strings"
	"testing"

	apiv1 "k8s.io/api/core/v1"
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

	t.Log(podCommand)
	if strings.Contains(podCommand, "&lt;") {
		t.Error("output contains incorrect encoding")
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

	t.Log(podCommand)
	if !strings.Contains(podCommand, "docker pull barry") {
		t.Error("docker pull command is missing")
	}
}
