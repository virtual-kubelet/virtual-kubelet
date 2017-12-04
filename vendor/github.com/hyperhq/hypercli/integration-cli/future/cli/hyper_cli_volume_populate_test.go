package main

import (
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPopulateImplicitVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/etc", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/etc")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "passwd")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestPopulateNamedVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "volume", "create", "--name", volName)
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/etc", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/etc")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "passwd")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestPopulateMultiMountImplicitVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/lib/modules/", "-v", volName+":/tmp", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/lib/modules")
	c.Assert(err, checker.Equals, 0)
	c.Assert(string(out), checker.HasLen, 0)
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestPopulateMultiMountNamedVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "volume", "create", "--name", volName)
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", volName+":/lib/modules/", "-v", volName+":/tmp", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/lib/modules")
	c.Assert(err, checker.Equals, 0)
	c.Assert(string(out), checker.HasLen, 0)
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestPopulateImageVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "--size=l1", "neo4j")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(string(out), checker.Contains, "databases")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestPopulateNamedImageVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "--size=l1", "-v", volName+":/data", "neo4j")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "ls", "/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(string(out), checker.Contains, "databases")
	dockerCmd(c, "rm", "-fv", "voltest")
}
