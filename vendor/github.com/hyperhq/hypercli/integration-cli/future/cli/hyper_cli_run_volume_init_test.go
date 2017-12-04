package main

import (
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRunGitVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	source := "git://git.kernel.org/pub/scm/utils/util-linux/util-linux.git"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "cat", "/data/README")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "util-linux")
	dockerCmd(c, "rm", "-fv", "voltest")

	source = "git://git.kernel.org/pub/scm/utils/util-linux/util-linux.git:stable/v2.13.0"
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "cat", "/data/configure.ac")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "2.13.0")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestRunHttpGitVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	source := "http://git.kernel.org/pub/scm/utils/util-linux/util-linux.git"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "cat", "/data/README")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "util-linux")
	dockerCmd(c, "rm", "-fv", "voltest")

	source = "http://git.kernel.org/pub/scm/utils/util-linux/util-linux.git:stable/v2.13.0"
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "cat", "/data/configure.ac")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "2.13.0")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestRunHttpsGitVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	source := "https://git.kernel.org/pub/scm/utils/util-linux/util-linux.git"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "cat", "/data/README")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "util-linux")
	dockerCmd(c, "rm", "-fv", "voltest")

	source = "http://git.kernel.org/pub/scm/utils/util-linux/util-linux.git:stable/v2.13.0"
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "cat", "/data/README")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "util-linux")
	dockerCmd(c, "rm", "-fv", "voltest")
}

func (s *DockerSuite) TestRunHttpFileVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	source := "https://raw.githubusercontent.com/hyperhq/hypercli/master/README.md"
	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "stat", "/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "regular file")
	dockerCmd(c, "rm", "-fv", "voltest")

	source = "https://raw.githubusercontent.com/hyperhq/hypercli/master/README.md"
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "stat", "/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Contains, "regular file")
	dockerCmd(c, "rm", "-fv", "voltest")

	source = "https://raw.githubusercontent.com/nosuchuser/nosuchrepo/masterbeta/README.md"
	_, _, cmdErr := dockerCmdWithError("run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(cmdErr, checker.NotNil)
}

func (s *DockerSuite) TestRunLocalFileVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	source := "/tmp/hyper_integration_test_local_file_volume_file"
	ioutil.WriteFile(source, []byte("foo"), 0644)

	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/volume/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "cat", "/volume/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Equals, "foo")
	dockerCmd(c, "rm", "-fv", "voltest")

	// Dir destination as a file
	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", source+":/volume/data/", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "cat", "/volume/data")
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Equals, "foo")
	dockerCmd(c, "rm", "-fv", "voltest")
	exec.Command("rm", "-f", source).CombinedOutput()

	// NonexistingVolumeBinding
	dir := "/tmp/nosuchfile"
	_, _, realErr := dockerCmdWithError("run", "-d", "--name=voltest", "-v", dir+":/data", "busybox")
	c.Assert(realErr, checker.NotNil)
}

func (s *DockerSuite) TestRunLocalDirVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	dir := "/tmp/hyper_integration_test_local_dir_volume_dir"
	file := "datafile"
	exec.Command("mkdir", "-p", dir).CombinedOutput()
	ioutil.WriteFile(dir+"/"+file, []byte("foo"), 0644)

	_, err := dockerCmd(c, "run", "-d", "--name=voltest", "-v", dir+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err := dockerCmd(c, "exec", "voltest", "cat", "/data/"+file)
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Equals, "foo")
	dockerCmd(c, "rm", "-fv", "voltest")

	exec.Command("rm", "-r", dir).CombinedOutput()

	// Deep dir binding
	dir = "/tmp/hyper_integration_test_local_dir_volume_dir"
	middle_dir := "/dir1/dir2/dir3/dir4/dir5"
	file = "datafile"
	exec.Command("mkdir", "-p", dir+"/"+middle_dir).CombinedOutput()
	ioutil.WriteFile(dir+"/"+middle_dir+"/"+file, []byte("foo"), 0644)

	_, err = dockerCmd(c, "run", "-d", "--name=voltest", "-v", dir+":/data", "busybox")
	c.Assert(err, checker.Equals, 0)
	out, err = dockerCmd(c, "exec", "voltest", "cat", "/data/"+middle_dir+"/"+file)
	c.Assert(err, checker.Equals, 0)
	c.Assert(out, checker.Equals, "foo")
	dockerCmd(c, "rm", "-fv", "voltest")

	exec.Command("rm", "-r", dir).CombinedOutput()
}

func (s *DockerSuite) TestRunExceptionVolumeBinding(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	// NonexistingVolumeBinding
	source := "/tmp/nosuchfile"
	_, _, err := dockerCmdWithError("run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.NotNil)

	source = "http://nosuchdomain"
	_, _, err = dockerCmdWithError("run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.NotNil)

	source = "git://nosuchdomain.git"
	_, _, err = dockerCmdWithError("run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.NotNil)

	source = "git://git.kernel.org/pub/scm/utils/util-linux/util-linux.git:nosuchbranch"
	_, _, err = dockerCmdWithError("run", "-d", "--name=voltest", "-v", source+":/data", "busybox")
	c.Assert(err, checker.NotNil)
}
