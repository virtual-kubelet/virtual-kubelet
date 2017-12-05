package main

import (
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestMultiMountImplicitVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/data1", "-v", volName+":/vol/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestMultiMountNamedVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "volume", "create", "--name", volName)
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/data1", "-v", volName+":/vol/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	dockerCmd(c, "rm", "-fv", "voltest")
}
