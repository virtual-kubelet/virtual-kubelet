package pod2docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	apiv1 "k8s.io/api/core/v1"
)

var defaultNetworkCount = 0

func TestMain(m *testing.M) {
	defaultNetworkCount = getNetworkCount()
	retCode := m.Run()
	os.Exit(retCode)
}

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
	checkCleanup(t, defaultNetworkCount)

}

func TestPod2DockerLogs_Integration(t *testing.T) {

	initContainers := []apiv1.Container{
		{
			Name:    "init1",
			Image:   "ubuntu",
			Command: []string{"echo 'init1'"},
		},
		{
			Name:    "init2",
			Image:   "ubuntu",
			Command: []string{"echo 'init2'"},
		},
	}
	containers := []apiv1.Container{
		{
			Name:    "container1",
			Image:   "ubuntu",
			Command: []string{"echo 'container1'"},
		},
		{
			Name:    "container2",
			Image:   "ubuntu",
			Command: []string{"echo 'container2'"},
		},
	}

	podCommand, err := GetBashCommand(PodComponents{
		Containers:     containers,
		InitContainers: initContainers,
		PodName:        randomName(6),
	})

	if err != nil {
		t.Error(err)
	}

	cmd := exec.Command("/bin/bash", "-c", podCommand)
	tempdir, err := ioutil.TempDir("", "pod2docker")
	if err != nil {
		t.Error(err)
	}
	cmd.Dir = tempdir
	out, err := cmd.CombinedOutput()
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			exitCode := status.ExitStatus()
			if exitCode != 0 {
				t.Errorf("Expected 0 exitcode got: %v", exitCode)
			}
		}
	}

	checkFileContents(filepath.Join(tempdir, "./init1.log"), "init1", t)
	checkFileContents(filepath.Join(tempdir, "./init2.log"), "init2", t)
	checkFileContents(filepath.Join(tempdir, "./container1.log"), "container1", t)
	checkFileContents(filepath.Join(tempdir, "./container2.log"), "container2", t)
	t.Log(string(out))
	checkCleanup(t, defaultNetworkCount)
}

func TestPod2DockerInitContainer_Integration(t *testing.T) {
	testcases := []struct {
		name             string
		initContainers   []apiv1.Container
		containers       []apiv1.Container
		expectedExitCode int
	}{
		{
			name: "failing single init container",
			initContainers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 10'"},
				},
			},
			containers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 13'"},
				},
			},
			expectedExitCode: 10,
		},
		{
			name: "failing second init container",
			initContainers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"echo 'first init container'"},
				},
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 11'"},
				},
			},
			containers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 13'"},
				},
			},
			expectedExitCode: 11,
		},
		{
			name: "successful single init container",
			initContainers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"echo 'hello'"},
				},
			},
			containers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 13'"},
				},
			},
			expectedExitCode: 13,
		},
		{
			name: "sucessful double init container",
			initContainers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"echo 'hello'"},
				},
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"echo 'world'"},
				},
			},
			containers: []apiv1.Container{
				{
					Name:  "sidecar",
					Image: "ubuntu",
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "sharedvolume",
							MountPath: "/home",
						},
					},
					Command: []string{"bash -c 'exit 13'"},
				},
			},
			expectedExitCode: 13,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			podCommand, err := GetBashCommand(PodComponents{
				Containers:     test.containers,
				InitContainers: test.initContainers,
				PodName:        randomName(6),
			})

			if err != nil {
				t.Error(err)
			}

			cmd := exec.Command("/bin/bash", "-c", podCommand)
			tempdir, err := ioutil.TempDir("", "pod2docker")
			if err != nil {
				t.Error(err)
			}
			cmd.Dir = tempdir
			out, err := cmd.CombinedOutput()
			if exiterr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					exitCode := status.ExitStatus()
					if exitCode != test.expectedExitCode {
						t.Errorf("Expected exitcode: %v got: %v", test.expectedExitCode, exitCode)
					}
				}
			}

			t.Log(string(out))
			checkCleanup(t, defaultNetworkCount)
		})
	}
}

func TestPod2DockerExitCode_Integration(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:    "sidecar",
			Image:   "ubuntu",
			Command: []string{"bash -c 'exit 13'"},
		},
		{
			Name:            "worker",
			Image:           "ubuntu",
			ImagePullPolicy: apiv1.PullAlways,
			Command:         []string{"bash -c 'sleep 100 && exit 0'"},
		},
	}

	podCommand, err := GetBashCommand(PodComponents{
		Containers: containers,
		PodName:    randomName(6),
	})

	if err != nil {
		t.Error(err)
	}

	cmd := exec.Command("/bin/bash", "-c", podCommand)
	tempdir, err := ioutil.TempDir("", "pod2docker")
	if err != nil {
		t.Error(err)
	}
	cmd.Dir = tempdir

	out, err := cmd.CombinedOutput()
	if msg, ok := err.(*exec.ExitError); ok {
		exitCode := (msg.Sys().(syscall.WaitStatus).ExitStatus())
		if exitCode != 13 {
			t.Errorf("Expected exit code of: %v got: %v", 13, exitCode)
		}
	} else if err != nil {
		t.Error(err)
	}

	t.Log(string(out))
	checkCleanup(t, defaultNetworkCount)
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
	checkCleanup(t, defaultNetworkCount)
}

func TestPod2DockerInvalidContainerImage_Integration(t *testing.T) {

	containers := []apiv1.Container{
		{
			Name:  "sidecar",
			Image: "doesntexist",
		},
		{
			Name:    "worker",
			Image:   "doesntexist",
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

	cmd := exec.Command("/bin/bash", "-c", podCommand)

	tempdir, err := ioutil.TempDir("", "pod2docker")
	if err != nil {
		t.Error(err)
	}
	cmd.Dir = tempdir
	out, err := cmd.CombinedOutput()

	if err.Error() != "exit status 125" {
		t.Error("Expected exit status 125")
		t.Error(err)
	}

	t.Log(string(out))
	checkCleanup(t, defaultNetworkCount)
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
	checkCleanup(t, defaultNetworkCount)
}

func getNetworkCount() int {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		panic(err)
	}
	return len(networks)
}

func checkCleanup(t *testing.T, defaultNetworkCount int) {
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if len(containers) > 1 {
		t.Error("Container left after pod2docker exit!")
		t.Error("Integration tests expect to be run on clean docker deamon with no other containers running")
	}

	for _, container := range containers {
		if container.Names[0] == "/pod2dockerci" {
			continue
		}
		fmt.Print("Stopping container ", container.ID[:10], "... ")
		if err := cli.ContainerStop(ctx, container.ID, nil); err != nil {
			t.Error(err)
			t.FailNow()
		}
		fmt.Print("Removing container ", container.ID[:10], "... ")
		if err := cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			t.Error(err)
			t.FailNow()
		}
		fmt.Println("Success")
	}

	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if len(networks) > defaultNetworkCount {
		t.Error("Network left after pod2docker exit!")
		t.Error("Integration tests expect to be run on clean docker deamon with the standard default networks")
		for _, network := range networks {
			fmt.Print("Removing network ", network.ID[:10], "... ")
			if err := cli.NetworkRemove(ctx, network.ID); err != nil {
				t.Logf("Failed to remove network ID: %v... expected for builtin networks", network.ID)
			}
			fmt.Println("Success")
		}
	}

	volumes, err := cli.VolumeList(ctx, filters.Args{})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if len(volumes.Volumes) > 0 {
		t.Error("Volume left after pod2docker exit!")
		t.Error("Integration tests expect to be run on clean docker deamon with no volumes present before starting")
	}

	for _, volume := range volumes.Volumes {
		fmt.Print("Removing volume ", volume.Name)
		if err := cli.VolumeRemove(ctx, volume.Name, true); err != nil {
			t.Error(err)
		}
		fmt.Println("Success")
	}
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

func checkFileContents(name, expectedContent string, t *testing.T) {
	b, err := ioutil.ReadFile(name) // just pass the file name
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(string(b), `{"log":"`+expectedContent) {
		t.Errorf("Expected to contain: %v it didn't - got: %v", expectedContent, string(b))
	}
}
