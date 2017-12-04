package main

import (
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliShareVolumeNamedVolumeBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("hyperhq/nfs-server")
	pullImageIfNotExist("busybox")

	volName := "testvolume"
	_, err := dockerCmd(c, "run", "-d", "--name=volserver", "-v", volName+":/data", "hyperhq/nfs-server")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "exec", "volclient", "ls", "/data")
	c.Assert(err, checker.Equals, 0)
}

func (s *DockerSuite) TestCliShareVolumeImplicitVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("hyperhq/nfs-server")
	pullImageIfNotExist("busybox")

	_, err := dockerCmd(c, "run", "-d", "--name=volserver", "-v", "/data", "hyperhq/nfs-server")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "exec", "volclient", "ls", "/data")
	c.Assert(err, checker.Equals, 0)
}

func (s *DockerSuite) TestCliShareVolumePopulatedVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("hyperhq/nfs-server")
	pullImageIfNotExist("busybox")

	_, err := dockerCmd(c, "run", "-d", "--name=volserver", "-v", "https://github.com/hyperhq/hypercli.git:/data", "hyperhq/nfs-server")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "volclient", "ls", "/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "Dockerfile")
}

func (s *DockerSuite) TestCliShareVolumeBadSource(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	_, err := dockerCmd(c, "run", "-d", "--name=volserver", "-v", "/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	_, _, failErr := dockerCmdWithError("run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(failErr, checker.NotNil)
}

func (s *DockerSuite) TestCliShareVolumeNoSource(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	_, _, err := dockerCmdWithError("run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliShareVolumeNoVolume(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("hyperhq/nfs-server")
	pullImageIfNotExist("busybox")

	_, err := dockerCmd(c, "run", "-d", "--name=volserver", "hyperhq/nfs-server")
	c.Assert(err, checker.Equals, 0)
	_, err = dockerCmd(c, "run", "-d", "--name=volclient", "--volumes-from", "volserver", "busybox")
	c.Assert(err, checker.Equals, 0)
	_, _, failErr := dockerCmdWithError("exec", "volclient", "ls", "/data")
	c.Assert(failErr, checker.NotNil)
}
