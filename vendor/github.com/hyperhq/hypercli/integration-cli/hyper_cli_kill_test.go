package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliKillContainerBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)
	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	dockerCmd(c, "kill", cleanedContainerID)

	out, _ = dockerCmd(c, "ps", "-q")
	c.Assert(out, checker.Not(checker.Contains), cleanedContainerID, check.Commentf("killed container is still running"))
}

func (s *DockerSuite) TestCliKillofStoppedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "stop", cleanedContainerID)

	_, _, err := dockerCmdWithError("kill", "-s", "30", cleanedContainerID)
	c.Assert(err, check.Not(check.IsNil), check.Commentf("Container %s is not running", cleanedContainerID))
}

/*
func (s *DockerSuite) TestCliKillStoppedContainerAPIPre120(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "docker-kill-test-api", "-d", "busybox", "top")
	dockerCmd(c, "stop", "docker-kill-test-api")

	status, _, err := sockRequest("POST", fmt.Sprintf("/v1.19/containers/%s/kill", "docker-kill-test-api"), nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNoContent)
}
*/
