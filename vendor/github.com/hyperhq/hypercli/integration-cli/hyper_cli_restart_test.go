package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliRestartStoppedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "foobar")
	time.Sleep(5 * time.Second)
	cleanedContainerID := strings.TrimSpace(out)
	c.Assert(waitExited(cleanedContainerID, 30*time.Second), checker.IsNil)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\n")

	dockerCmd(c, "restart", cleanedContainerID)
	time.Sleep(5 * time.Second)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\n")
}

func (s *DockerSuite) TestCliRestartRunningContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "sh", "-c", "echo foobar && sleep 30 && echo 'should not print this'")
	time.Sleep(5 * time.Second)
	cleanedContainerID := strings.TrimSpace(out)

	c.Assert(waitRun(cleanedContainerID), checker.IsNil)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)
	c.Assert(out, checker.Equals, "foobar\n")

	dockerCmd(c, "restart", "-t", "1", cleanedContainerID)
	time.Sleep(5 * time.Second)

	out, _ = dockerCmd(c, "logs", cleanedContainerID)

	c.Assert(waitRun(cleanedContainerID), checker.IsNil)

	c.Assert(out, checker.Equals, "foobar\nfoobar\n")
}

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func (s *DockerSuite) TestCliRestartWithVolumesBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "-v", "/test", "busybox", "top")

	cleanedContainerID := strings.TrimSpace(out)
	out, err := inspectFilter(cleanedContainerID, "len .Mounts")
	c.Assert(err, check.IsNil, check.Commentf("failed to inspect %s: %s", cleanedContainerID, out))
	out = strings.Trim(out, " \n\r")
	c.Assert(out, checker.Equals, "1")

	source, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "restart", cleanedContainerID)

	out, err = inspectFilter(cleanedContainerID, "len .Mounts")
	c.Assert(err, check.IsNil, check.Commentf("failed to inspect %s: %s", cleanedContainerID, out))
	out = strings.Trim(out, " \n\r")
	c.Assert(out, checker.Equals, "1")

	sourceAfterRestart, err := inspectMountSourceField(cleanedContainerID, "/test")
	c.Assert(err, checker.IsNil)
	c.Assert(source, checker.Equals, sourceAfterRestart)
}

func (s *DockerSuite) TestCliRestartPolicyNO(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--restart=no", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "no")
}

func (s *DockerSuite) TestCliRestartPolicyAlwaysBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "always")

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")

	// MaximumRetryCount=0 if the restart policy is always
	c.Assert(MaximumRetryCount, checker.Equals, "0")
}

func (s *DockerSuite) TestCliRestartPolicyOnFailure(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:1", "busybox", "false")

	id := strings.TrimSpace(string(out))
	name := inspectField(c, id, "HostConfig.RestartPolicy.Name")
	c.Assert(name, checker.Equals, "on-failure")

}

// a good container with --restart=on-failure:3
// MaximumRetryCount!=0; RestartCount=0
func (s *DockerSuite) TestCliRestartContainerRestartwithGoodContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:3", "busybox", "true")

	id := strings.TrimSpace(string(out))
	err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", 50*time.Second)
	c.Assert(err, checker.IsNil)

	count := inspectField(c, id, "RestartCount")
	c.Assert(count, checker.Equals, "0")

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(MaximumRetryCount, checker.Equals, "3")

}

func (s *DockerSuite) TestCliRestartContainerRestartSuccess(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--restart=always", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	pidStr := inspectField(c, id, "State.Pid")

	pid, err := strconv.Atoi(pidStr)
	c.Assert(err, check.IsNil)

	p, err := os.FindProcess(pid)
	c.Assert(err, check.IsNil)
	c.Assert(p, check.NotNil)

	err = p.Kill()
	c.Assert(err, check.IsNil)

	err = waitInspect(id, "{{.RestartCount}}", "1", 50*time.Second)
	c.Assert(err, check.IsNil)

	err = waitInspect(id, "{{.State.Status}}", "running", 50*time.Second)
	c.Assert(err, check.IsNil)
}
