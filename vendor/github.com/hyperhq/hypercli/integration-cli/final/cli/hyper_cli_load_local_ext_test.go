package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliLoadFromLocalDocker(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	testImage := "hello-world:latest"

	//local docker pull image
	pullCmd := exec.Command("docker", "--region", os.Getenv("LOCAL_DOCKER_HOST"), "pull", testImage)
	output, exitCode, err := runCommandWithOutput(pullCmd)
	c.Assert(err, checker.IsNil)
	c.Assert(exitCode, checker.Equals, 0)

	//load image from local docker to hyper
	output, exitCode, err = dockerCmdWithError("load", "-l", testImage)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(err, checker.IsNil)
	c.Assert(exitCode, checker.Equals, 0)

	//check image
	images, _ := dockerCmd(c, "images", "hello-world")
	c.Assert(images, checker.Contains, "hello-world")
}

func (s *DockerSuite) TestCliLoadFromLocalTarSize600MB(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	publicURL := "http://image-tarball.s3.amazonaws.com/test/public/jenkins.tar"
	imagePath := fmt.Sprintf("%s/jenkins.tar", os.Getenv("IMAGE_DIR"))

	//download image tar
	wgetCmd := exec.Command("wget", "-cO", imagePath, publicURL)
	output, exitCode, err := runCommandWithOutput(wgetCmd)
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)
	c.Assert(pathExist(imagePath), checker.Equals, true)

	//ensure jenkins:latest not exist
	dockerCmdWithError("rmi", "jenkins:latest")
	images, _ := dockerCmd(c, "images", "jenkins:latest")
	c.Assert(images, checker.Not(checker.Contains), "jenkins")

	//load image tar
	output, exitCode, err = dockerCmdWithError("load", "-i", imagePath)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(err, checker.IsNil)
	c.Assert(exitCode, checker.Equals, 0)

	//check image
	images, _ = dockerCmd(c, "images", "jenkins:latest")
	c.Assert(images, checker.Contains, "jenkins")
}