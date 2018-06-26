package pod2docker

import (
	"bytes"
	"strings"
	"text/template"

	"k8s.io/api/core/v1"
)

// ImageRegistryCredential - Used to input a credential used by docker login
type ImageRegistryCredential struct {
	Server   string `json:"server,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// PodComponents provides details to run a pod
type PodComponents struct {
	PullCredentials []ImageRegistryCredential
	InitContainers  []v1.Container
	Containers      []v1.Container
	Volumes         []v1.Volume
	PodName         string
}

// GetBashCommand generates the bash script to execute the pod
func GetBashCommand(p PodComponents) (string, error) {
	template := template.New("run.sh.tmpl").Option("missingkey=error").Funcs(template.FuncMap{
		"getLaunchCommand":     getLaunchCommand,
		"isHostPathVolume":     isHostPathVolume,
		"isEmptyDirVolume":     isEmptyDirVolume,
		"isPullAlways":         isPullAlways,
		"getValidVolumeMounts": getValidVolumeMounts,
		"isNvidiaRuntime":      isNvidiaRuntime,
	})

	template, err := template.Parse(azureBatchPodTemplate)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = template.Execute(&output, p)
	return output.String(), err
}

func getLaunchCommand(container v1.Container) (cmd string) {
	if len(container.Command) > 0 {
		cmd += strings.Join(container.Command, " ")
	}
	if len(cmd) > 0 {
		cmd += " "
	}
	if len(container.Args) > 0 {
		cmd += strings.Join(container.Args, " ")
	}
	return
}

func isNvidiaRuntime(c v1.Container) bool {
	if _, exists := c.Resources.Limits["nvidia.com/gpu"]; exists {
		return true
	}
	return false
}

func isHostPathVolume(v v1.Volume) bool {
	if v.HostPath == nil {
		return false
	}
	return true
}

func isEmptyDirVolume(v v1.Volume) bool {
	if v.EmptyDir == nil {
		return false
	}
	return true
}

func isPullAlways(c v1.Container) bool {
	if c.ImagePullPolicy == v1.PullAlways {
		return true
	}
	return false
}

func getValidVolumeMounts(container v1.Container, volumes []v1.Volume) []v1.VolumeMount {
	volDic := make(map[string]v1.Volume)
	for _, vol := range volumes {
		volDic[vol.Name] = vol
	}
	var mounts []v1.VolumeMount
	for _, mount := range container.VolumeMounts {
		vol, ok := volDic[mount.Name]
		if !ok {
			continue
		}
		if vol.EmptyDir == nil && vol.HostPath == nil {
			continue
		}
		mounts = append(mounts, mount)
	}
	return mounts
}
