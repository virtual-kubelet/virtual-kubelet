package main

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliRmContainerWithRemovedVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon)

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	tempDir, err := ioutil.TempDir("", "test-rm-container-with-removed-volume-")
	if err != nil {
		c.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)

	dockerCmd(c, "run", "--name", "losemyvolumes", "-v", tempDir+":"+prefix+slash+"test", "busybox", "true")

	err = os.RemoveAll(tempDir)
	c.Assert(err, check.IsNil)

	dockerCmd(c, "rm", "-v", "losemyvolumes")
}

func (s *DockerSuite) TestCliRmContainerWithVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllContainers()
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name", "foo", "-v", prefix+slash+"srv", "busybox", "true")

	dockerCmd(c, "rm", "-v", "foo")
}

func (s *DockerSuite) TestCliRmContainerRunningBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllContainers()
	createRunningContainer(c, "foo")

	_, _, err := dockerCmdWithError("rm", "foo")
	c.Assert(err, checker.NotNil, check.Commentf("Expected error, can't rm a running container"))
}

func (s *DockerSuite) TestCliRmContainerForceRemoveRunningBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllContainers()
	createRunningContainer(c, "foo")

	// Stop then remove with -s
	dockerCmd(c, "rm", "-f", "foo")
}

func (s *DockerSuite) TestCliRmInvalidContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("rm", "unknown")
	c.Assert(err, checker.NotNil, check.Commentf("Expected error on rm unknown container, got none"))
	c.Assert(out, checker.Contains, "No such container")
}

func createRunningContainer(c *check.C, name string) {
	runSleepingContainer(c, "-dt", "--name", name)
	time.Sleep(1 * time.Second)
}
