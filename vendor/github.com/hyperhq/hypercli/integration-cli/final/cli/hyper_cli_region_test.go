package main

import (
	"time"
	"os"
	"os/exec"

	"github.com/go-check/check"
	"github.com/docker/docker/pkg/integration/checker"
)

func (s *DockerSuite) TestCliRegionBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	var (
		defaultRegion = os.Getenv("REGION")
		anotherRegion = ""
		err error
	)
	switch defaultRegion {
	case "":
		defaultRegion = "us-west-1"
		anotherRegion = "eu-central-1"
	case "us-west-1":
		anotherRegion = "eu-central-1"
	case "eu-central-1":
		anotherRegion = "us-west-1"
	}

	////////////////////////////////////////////
	//pull image with default region
	cmd := exec.Command(dockerBinary, "pull", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Contains, "Status: Downloaded newer image for busybox:latest")

	cmd = exec.Command(dockerBinary, "run", "busybox", "echo", "test123")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Not(checker.Contains), "Unable to find image 'busybox:latest' in the current region")
	c.Assert(out, checker.Contains, "test123")

	////////////////////////////////////////////
	//image not exist in another region
	cmd = exec.Command(dockerBinary, "--region", anotherRegion, "images", "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Not(checker.Contains), "busybox")

	////////////////////////////////////////////
	//pull image with specified region
	cmd = exec.Command(dockerBinary, "--region", anotherRegion, "pull", "ubuntu")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Contains, "Status: Downloaded newer image for ubuntu:latest")

	cmd = exec.Command(dockerBinary, "--region", anotherRegion, "run", "ubuntu", "echo", "test123")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Not(checker.Contains), "Unable to find image 'ubuntu:latest' in the current region")
	c.Assert(out, checker.Contains, "test123")
}

func (s *DockerSuite) TestCliRegionWithFullEntrypoint(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "pull", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	c.Assert(out, checker.Contains, "Status: Downloaded newer image for busybox:latest")
}
