package main

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliSnapshotCreate(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "snapshot", "create", "--volume=test", "--name=test-snap")
	name = strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test-snap")

	out, _, err := dockerCmdWithError("snapshot", "create", "--volume=test", "--name=test-snap")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "A snapshot named test-snap already exists. Choose a different snapshot name")
	dockerCmd(c, "snapshot", "rm", "test-snap")
	dockerCmd(c, "volume", "rm", "test")
}

func (s *DockerSuite) TestCliSnapshotInspect(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	c.Assert(
		exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "snapshot", "inspect", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("snapshot inspect should error on non-existent volume"),
	)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "snapshot", "create", "--volume=test")
	name = strings.TrimSpace(out)
	out, _ = dockerCmd(c, "snapshot", "inspect", "--format='{{ .Name }}'", name)
	c.Assert(strings.TrimSpace(out), check.Equals, name)

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name=test-snap")
	out, _ = dockerCmd(c, "snapshot", "inspect", "--format='{{ .Name }}'", "test-snap")
	c.Assert(strings.TrimSpace(out), check.Equals, "test-snap")
	dockerCmd(c, "snapshot", "rm", name)
	dockerCmd(c, "snapshot", "rm", "test-snap")
	dockerCmd(c, "volume", "rm", "test")
}

func (s *DockerSuite) TestCliSnapshotInspectMulti(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name=test-snap1")
	dockerCmd(c, "snapshot", "create", "--volume=test", "--name=test-snap2")
	dockerCmd(c, "snapshot", "create", "--volume=test", "--name=not-shown")

	out, _, err := dockerCmdWithError("snapshot", "inspect", "--format='{{ .Name }}'", "test-snap1", "test-snap2", "doesntexist", "not-shown")
	c.Assert(err, checker.NotNil)
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 3, check.Commentf("\n%s", out))

	c.Assert(out, checker.Contains, "test-snap1")
	c.Assert(out, checker.Contains, "test-snap2")
	c.Assert(out, checker.Contains, "Error: No such snapshot: doesntexist")
	c.Assert(out, checker.Not(checker.Contains), "not-shown")
	dockerCmd(c, "snapshot", "rm", "test-snap1")
	dockerCmd(c, "snapshot", "rm", "test-snap2")
	dockerCmd(c, "snapshot", "rm", "not-shown")
	dockerCmd(c, "volume", "rm", "test")
}

func (s *DockerSuite) TestCliSnapshotLs(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "snapshot", "create", "--volume=test")
	id := strings.TrimSpace(out)

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name=test-snap")

	out, _ = dockerCmd(c, "snapshot", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 3, check.Commentf("\n%s", out))

	// Since there is no guarantee of ordering of volumes, we just make sure the names are in the output
	c.Assert(strings.Contains(out, id), check.Equals, true)
	c.Assert(strings.Contains(out, "test-snap"), check.Equals, true)
	dockerCmd(c, "snapshot", "rm", "test-snap")
	dockerCmd(c, "snapshot", "rm", id)
	dockerCmd(c, "volume", "rm", "test")
}

func (s *DockerSuite) TestCliSnapshotRm(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "snapshot", "create", "--volume=test")
	id := strings.TrimSpace(out)

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name", "test-snap")
	dockerCmd(c, "snapshot", "rm", id)
	dockerCmd(c, "snapshot", "rm", "test-snap")

	out, _ = dockerCmd(c, "snapshot", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))

	c.Assert(
		exec.Command("snapshot", "rm", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("snapshot rm should fail with non-existent snapshot"),
	)
}

func (s *DockerSuite) TestCliSnapshotNoArgs(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "snapshot")
	// no args should produce the cmd usage output
	usage := "Usage:	hyper snapshot [OPTIONS] [COMMAND]"
	c.Assert(out, checker.Contains, usage)

	// invalid arg should error and show the command on stderr
	_, stderr, _, err := runCommandWithStdoutStderr(exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "snapshot", "somearg"))
	c.Assert(err, check.NotNil, check.Commentf(stderr))
	c.Assert(stderr, checker.Contains, usage)

	// invalid flag should error and show the flag error and cmd usage
	_, stderr, _, err = runCommandWithStdoutStderr(exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "snapshot", "--no-such-flag"))
	c.Assert(err, check.NotNil, check.Commentf(stderr))
	c.Assert(stderr, checker.Contains, usage)
	c.Assert(stderr, checker.Contains, "flag provided but not defined: --no-such-flag")
}

func (s *DockerSuite) TestCliSnapshotInspectTmplError(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "snapshot", "create", "--volume=test")
	name = strings.TrimSpace(out)

	out, exitCode, err := dockerCmdWithError("snapshot", "inspect", "--format='{{ .FooBar}}'", name)
	c.Assert(err, checker.NotNil, check.Commentf("Output: %s", out))
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Contains, "Template parsing error")
	dockerCmd(c, "snapshot", "rm", name)
	dockerCmd(c, "volume", "rm", "test")
}

func (s *DockerSuite) TestCliSnapshotCreateVolBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name", "test-snap")

	dockerCmd(c, "volume", "create", "--name=snap-vol", "--snapshot=test-snap")
	out, _ = dockerCmd(c, "volume", "ls")
	c.Assert(strings.Contains(out, "snap-vol"), check.Equals, true)

	// delete, in the order snapshot, volume, volume
	out, _ = dockerCmd(c, "snapshot", "rm", "test-snap")
	name = strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test-snap")

	out, _ = dockerCmd(c, "volume", "rm", "test")
	name = strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "volume", "rm", "snap-vol")
	name = strings.TrimSpace(out)
	c.Assert(name, check.Equals, "snap-vol")
}

func (s *DockerSuite) TestCliSnapshotRmBasedVol(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	dockerCmd(c, "snapshot", "create", "--volume=test", "--name", "test-snap")

	out, _, err := dockerCmdWithError("volume", "rm", "test")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "Volume(test) has (1) snapshots")

	dockerCmd(c, "snapshot", "rm", "test-snap")
	_, _, err = dockerCmdWithError("volume", "rm", "test")
	c.Assert(err, checker.IsNil)
}
