package main

import (
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliInspectNamedMountPointBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "test", "-v", "data:/data", "busybox", "cat")

	vol := inspectFieldJSON(c, "test", "Mounts")

	var mp []types.MountPoint
	err := unmarshalJSON([]byte(vol), &mp)
	c.Assert(err, checker.IsNil)

	c.Assert(mp, checker.HasLen, 1, check.Commentf("Expected 1 mount point"))

	m := mp[0]
	c.Assert(m.Name, checker.Equals, "data", check.Commentf("Expected name data"))

	c.Assert(m.Source, checker.Not(checker.Equals), "", check.Commentf("Expected source to not be empty"))

	c.Assert(m.RW, checker.Equals, true)

	c.Assert(m.Destination, checker.Equals, "/data", check.Commentf("Expected destination /data"))
}
