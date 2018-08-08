package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliCommitAfterContainerIsDone(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")

	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "wait", cleanedContainerID)

	out, _ = dockerCmd(c, "commit", cleanedContainerID)

	cleanedImageID := strings.TrimSpace(out)

	dockerCmd(c, "inspect", cleanedImageID)
}

func (s *DockerSuite) TestCliCommitRunningContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	out, _, err := dockerCmdWithError("commit", cleanedContainerID)
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Equals, "Error response from daemon: Bad request parameters: only stopped container could be committed\n")
}

func (s *DockerSuite) TestCliCommitNewFileBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name", "commiter", "busybox", "/bin/sh", "-c", "echo koye > /foo")

	imageID, _ := dockerCmd(c, "commit", "commiter")
	imageID = strings.TrimSpace(imageID)

	out, _ := dockerCmd(c, "run", imageID, "cat", "/foo")
	actual := strings.TrimSpace(out)
	c.Assert(actual, checker.Equals, "koye")
}

func (s *DockerSuite) TestCliCommitChange(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name", "test", "busybox", "true")

	imageID, _ := dockerCmd(c, "commit",
		"--change", "EXPOSE 8080",
		"--change", "ENV DEBUG true",
		"--change", "ENV test 1",
		"--change", "ENV PATH /foo",
		"--change", "LABEL foo bar",
		"--change", "CMD [\"/bin/sh\"]",
		"--change", "WORKDIR /opt",
		"--change", "ENTRYPOINT [\"/bin/sh\"]",
		"--change", "USER testuser",
		"--change", "VOLUME /var/lib/docker",
		"test", "test-commit",
	)
	imageID = strings.TrimSpace(imageID)

	expected := map[string]string{
		"Config.ExposedPorts": "map[8080/tcp:{}]",
		"Config.Env":          "[DEBUG=true test=1 PATH=/foo]",
		"Config.Labels":       "map[foo:bar]",
		"Config.Cmd":          "[/bin/sh]",
		"Config.WorkingDir":   "/opt",
		"Config.Entrypoint":   "[/bin/sh]",
		"Config.User":         "testuser",
		"Config.Volumes":      "map[/var/lib/docker:{}]",
	}

	for conf, value := range expected {
		res := inspectField(c, imageID, conf)
		if res != value {
			c.Errorf("%s('%s'), expected %s", conf, res, value)
		}
	}
}
