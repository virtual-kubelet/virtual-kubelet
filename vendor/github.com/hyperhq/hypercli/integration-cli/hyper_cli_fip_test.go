package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliFipAssociateUsedIPBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	out, _ := dockerCmd(c, "fip", "allocate", "-y", "1")
	firstIP := strings.TrimSpace(out)
	fipList := []string{firstIP}
	defer releaseFip(c, fipList)

	pullImageIfNotExist("busybox")
	out, _ = runSleepingContainer(c, "-d")
	firstContainerID := strings.TrimSpace(out)

	out, _ = runSleepingContainer(c, "-d")
	secondContainerID := strings.TrimSpace(out)

	dockerCmd(c, "fip", "attach", firstIP, firstContainerID)
	out, _, err := dockerCmdWithError("fip", "attach", firstIP, secondContainerID)
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, firstIP+"\n")
}

func (s *DockerSuite) TestCliFipAttachConfedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	out, _ := dockerCmd(c, "fip", "allocate", "-y", "1")
	firstIP := strings.TrimSpace(out)
	fipList := []string{firstIP}

	out, _ = dockerCmd(c, "fip", "allocate", "-y", "1")
	secondIP := strings.TrimSpace(out)
	fipList = append(fipList, secondIP)
	defer releaseFip(c, fipList)

	pullImageIfNotExist("busybox")
	out, _ = runSleepingContainer(c, "-d")
	firstContainerID := strings.TrimSpace(out)

	dockerCmd(c, "fip", "attach", firstIP, firstContainerID)
	out, _, err := dockerCmdWithError("fip", "attach", secondIP, firstContainerID)
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, firstIP+"\n")
}

func (s *DockerSuite) TestCliFipDettachUnconfedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	out, _ := runSleepingContainer(c, "-d")
	firstContainerID := strings.TrimSpace(out)

	out, _, err := dockerCmdWithError("fip", "detach", firstContainerID)
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
}

func (s *DockerSuite) TestCliFipReleaseUsedIP(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	out, _ := dockerCmd(c, "fip", "allocate", "-y", "1")
	firstIP := strings.TrimSpace(out)
	fipList := []string{firstIP}
	defer releaseFip(c, fipList)

	pullImageIfNotExist("busybox")
	out, _ = runSleepingContainer(c, "-d")
	firstContainerID := strings.TrimSpace(out)

	dockerCmd(c, "fip", "attach", firstIP, firstContainerID)
	out, _, err := dockerCmdWithError("fip", "release", firstIP)
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, firstIP+"\n")
}

func (s *DockerSuite) TestCliFipReleaseInvalidIP(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	out, _, err := dockerCmdWithError("fip", "release", "InvalidIP")
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))

	out, _, err = dockerCmdWithError("fip", "release", "0.0.0.0")
	c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
}

func (s *DockerSuite) TestCliFipName(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	out, _ := dockerCmd(c, "fip", "allocate", "-y", "1")
	firstIP := strings.TrimSpace(out)
	fipList := []string{firstIP}

	out, _ = dockerCmd(c, "fip", "allocate", "-y", "1")
	secondIP := strings.TrimSpace(out)
	fipList = append(fipList, secondIP)

	defer releaseFip(c, fipList)

	pullImageIfNotExist("busybox")
	out, _ = runSleepingContainer(c, "-d")
	firstContainerID := strings.TrimSpace(out)

	// multiple FIPs without name, attach one with container
	dockerCmd(c, "fip", "attach", secondIP, firstContainerID)
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, secondIP+"\n")

	// multiple FIPs (one FIP has name), attach one with container
	dockerCmd(c, "fip", "name", secondIP, "ip2")
	dockerCmd(c, "fip", "attach", secondIP, firstContainerID)
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, secondIP+"\n")

	// multiple FIPs (two FIPs have name), attach one with container
	dockerCmd(c, "fip", "name", firstIP, "ip1")
	dockerCmd(c, "fip", "attach", secondIP, firstContainerID)
	out, _ = dockerCmd(c, "fip", "detach", firstContainerID)
	c.Assert(out, checker.Equals, secondIP+"\n")
}
