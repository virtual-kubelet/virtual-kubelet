package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// Image volume mounted at directory "/data1"
func (s *DockerSuite) TestVerifyNoautoVolumeBaseImage(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "hyperhq/noauto_volume_test")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "df", "/data1")
	c.Assert(err, checker.Equals, 0)
	c.Assert(strings.Contains(string(out), "data1"), checker.True, check.Commentf("got df results: %s", string(out)))
	dockerCmd(c, "rm", "-fv", "voltest")
}

// No volume mounted at directory "/data1"
func (s *DockerSuite) TestNoautoVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	_, err := dockerCmd(c, "run", "-d", "--noauto-volume", "--name=voltest", "hyperhq/noauto_volume_test")
	c.Assert(err, checker.Equals, 0)
	_, exitCode, _ := dockerCmdWithError("exec", "voltest", "ls", "/data1")
	c.Assert(exitCode, checker.GreaterThan, 0)
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestImplicitOverwriteNoautoVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	_, err := dockerCmd(c, "run", "-d", "--noauto-volume", "--name=voltest", "-v", "/data1", "hyperhq/noauto_volume_test")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "df", "/data1")
	c.Assert(err, checker.Equals, 0)
	c.Assert(strings.Contains(string(out), "data1"), checker.True, check.Commentf("got df results: %s", string(out)))
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestNamedOverwriteNoautoVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--noauto-volume", "--name=voltest", "-v", volName+":/data1", "hyperhq/noauto_volume_test")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "df", "/data1")
	c.Assert(err, checker.Equals, 0)
	c.Assert(strings.Contains(string(out), "data1"), checker.True, check.Commentf("got df results: %s", string(out)))
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestNoautoAndNormalVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--noauto-volume", "--name=voltest", "-v", volName+":/vol/data", "hyperhq/noauto_volume_test")
	c.Assert(err, checker.Equals, 0)
	_, exitCode, _ := dockerCmdWithError("exec", "voltest", "ls", "/data1")
	c.Assert(exitCode, checker.GreaterThan, 0)
	out, err := dockerCmd(c, "exec", "voltest", "df", "/vol/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(strings.Contains(string(out), "data"), checker.True, check.Commentf("got df /vol/data results: %s", string(out)))
	dockerCmd(c, "rm", "-fv", "voltest")
}
