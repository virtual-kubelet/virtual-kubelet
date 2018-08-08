package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliRenameStoppedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--name", "first-name", "-d", "busybox", "sh")

	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "stop", cleanedContainerID)

	name := inspectField(c, cleanedContainerID, "Name")
	newName := "new-name" + stringid.GenerateNonCryptoID()
	dockerCmd(c, "rename", "first-name", newName)

	name = inspectField(c, cleanedContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

}

func (s *DockerSuite) TestCliRenameRunningContainerBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--name", "first-name", "-d", "busybox", "sh")

	newName := "new-name" + stringid.GenerateNonCryptoID()
	cleanedContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first-name", newName)

	name := inspectField(c, cleanedContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))
}

func (s *DockerSuite) TestCliRenameRunningContainerAndReuse(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := runSleepingContainer(c, "--name", "first-name")
	c.Assert(waitRun("first-name"), check.IsNil)

	newName := "new-name"
	ContainerID := strings.TrimSpace(out)
	dockerCmd(c, "rename", "first-name", newName)

	name := inspectField(c, ContainerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))

	out, _ = runSleepingContainer(c, "--name", "first-name")
	c.Assert(waitRun("first-name"), check.IsNil)
	newContainerID := strings.TrimSpace(out)
	name = inspectField(c, newContainerID, "Name")
	c.Assert(name, checker.Equals, "/first-name", check.Commentf("Failed to reuse container name"))
}

func (s *DockerSuite) TestCliRenameCheckNames(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name", "first-name", "-d", "busybox", "sh")

	newName := "new-name" + stringid.GenerateNonCryptoID()[:32]
	dockerCmd(c, "rename", "first-name", newName)

	name := inspectField(c, newName, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container %s", name))

	name, err := inspectFieldWithError("first-name", "Name")
	c.Assert(err, checker.NotNil, check.Commentf(name))
	c.Assert(err.Error(), checker.Contains, "No such image or container: first-name")
}

func (s *DockerSuite) TestCliRenameInvalidName(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	runSleepingContainer(c, "--name", "myname")

	out, _, err := dockerCmdWithError("rename", "myname", "new:invalid")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "new:invalid is invalid, should be", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "myname", "")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container to invalid name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _, err = dockerCmdWithError("rename", "", "newname")
	c.Assert(err, checker.NotNil, check.Commentf("Renaming container with empty name should have failed: %s", out))
	c.Assert(out, checker.Contains, "may be empty", check.Commentf("%v", err))

	out, _ = dockerCmd(c, "ps", "-a")
	c.Assert(out, checker.Contains, "myname", check.Commentf("Output of docker ps should have included 'myname': %s", out))
}
