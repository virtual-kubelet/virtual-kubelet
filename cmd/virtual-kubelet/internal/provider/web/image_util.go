package web

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

// CopyImageFromDockerRepository copy image at specific location
func CopyImageFromDockerRepository(imageSource string, imageDest string) string {
	var response string = path.Join(GetScriptPath(), "/cmd/virtual-kubelet/internal/provider/web/bash_scripts/copy_image.sh")
	if response == "" {
		return "1";
	}
	fmt.Println("Started Downloading..")
    cmd, err := exec.Command("/bin/sh", response, imageSource , imageDest).Output()
    if err != nil {
		return "1"
    }
	output := string(cmd)
	fmt.Println("Downloading Completed..")
    return output
}

// GetScriptPath will return copy_image.sh path
func GetScriptPath() string {
	dirname, err := os.Getwd()
    if err != nil {
        panic(err)
    }
    return dirname
}