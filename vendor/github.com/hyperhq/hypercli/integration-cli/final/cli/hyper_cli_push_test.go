package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-check/check"
	"github.com/hyperhq/hypercli/pkg/integration/checker"
)

func (s *DockerSuite) TestCliPush(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	var (
		out      string
		err      error
		newImage = fmt.Sprintf("%v/busybox:latest", os.Getenv("DOCKERHUB_USERNAME"))
	)
	pullImageIfNotExist("busybox")
	out, _ = dockerCmd(c, "run", "-d", "busybox", "bash", "-c", "touch", "test")
	cleanedContainerID := strings.TrimSpace(out)

	_, _, err = dockerCmdWithError("commit", cleanedContainerID, newImage)
	c.Assert(err, checker.IsNil)

	//login dockerhub
	out, _ = dockerCmd(c, "login", "-e", os.Getenv("DOCKERHUB_EMAIL"), "-u", os.Getenv("DOCKERHUB_USERNAME"), "-p", os.Getenv("DOCKERHUB_PASSWD"))
	c.Assert(out, checker.Contains, "Login Succeeded")

	//push to dockerhub
	out, _, err = dockerCmdWithError("push", newImage)
	c.Assert(out, checker.Contains, "digest:")
	c.Assert(err, checker.IsNil)
}
