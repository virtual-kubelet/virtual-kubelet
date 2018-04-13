package pod2docker

import (
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
)

func TestPod2DockerVolume_Integration(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "busybox",
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/home",
				},
			},
			Command: []string{"touch /home/created.txt"},
		},
		{
			Name:            "worker",
			Image:           "busybox",
			ImagePullPolicy: apiv1.PullAlways,
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/home",
				},
			},
			Command: []string{"cat /home/created.txt"},
		},
	}

	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    randomName(6),
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

	t.Log(podCommand)

	cmd := exec.Command("/bin/bash", "-c", podCommand)
	tempdir, err := ioutil.TempDir("", "pod2docker")
	if err != nil {
		t.Error(err)
	}
	cmd.Dir = tempdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(err)
	}

	t.Log(string(out))
}

func TestPod2DockerNetwork_Integration(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "nginx",
		},
		{
			Name:    "worker",
			Image:   "busybox",
			Command: []string{"wget localhost"},
		},
	}

	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    randomName(6),
		Volumes:    []apiv1.Volume{},
	})

	if err != nil {
		t.Error(err)
	}

	t.Log(podCommand)

	cmd := exec.Command("/bin/bash", "-c", podCommand)

	wd, _ := os.Getwd()
	cmd.Dir = wd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(err)
	}

	t.Log(string(out))
}

func TestPod2DockerIPCAndHostDir_Integration(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "ubuntu",
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/testdata",
				},
			},
			Command: []string{"/testdata/readpipe.sh"},
		},
		{
			Name:            "worker",
			Image:           "ubuntu",
			ImagePullPolicy: apiv1.PullAlways,
			VolumeMounts: []apiv1.VolumeMount{
				{
					Name:      "sharedvolume",
					MountPath: "/testdata",
				},
			},
			Command: []string{"/testdata/writepipe.sh"},
		},
	}

	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    randomName(6),
		Volumes: []apiv1.Volume{
			{
				Name: "sharedvolume",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "$HOSTDIR/testdata",
					},
				},
			},
		},
	})

	if err != nil {
		t.Error(err)
	}

	t.Log(podCommand)

	cmd := exec.Command("/bin/bash", "-c", podCommand)

	env := os.Getenv("HOSTDIR")
	if env == "" {
		wd, _ := os.Getwd()
		cmd.Dir = wd
	} else {
		cmd.Dir = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(err)
	}

	t.Log(string(out))
}

var lettersLower = []rune("abcdefghijklmnopqrstuvwxyz")

// RandomName random letter sequence
func randomName(n int) string {
	return randFromSelection(n, lettersLower)
}

func randFromSelection(length int, choices []rune) string {
	b := make([]rune, length)
	rand.Seed(time.Now().UnixNano())
	for i := range b {
		b[i] = choices[rand.Intn(len(choices))]
	}
	return string(b)
}
