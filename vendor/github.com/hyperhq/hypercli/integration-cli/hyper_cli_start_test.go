package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

/*
func (s *DockerSuite) TestCliStartRecordError(c *check.C) {
	// TODO Windows CI: Requires further porting work. Should be possible.
	testRequires(c, DaemonIsLinux)
	// when container runs successfully, we should not have state.Error
	dockerCmd(c, "run", "-d", "-p", "9999:9999", "--name", "test", "busybox", "top")
	stateErr := inspectField(c, "test", "State.Error")
	// Expected to not have state error
	c.Assert(stateErr, checker.Equals, "")

	// Expect this to fail and records error because of ports conflict
	out, _, err := dockerCmdWithError("run", "-d", "--name", "test2", "-p", "9999:9999", "busybox", "top")
	// err shouldn't be nil because docker run will fail
	c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))

	stateErr = inspectField(c, "test2", "State.Error")
	c.Assert(stateErr, checker.Contains, "port is already allocated")

	// Expect the conflict to be resolved when we stop the initial container
	dockerCmd(c, "stop", "test")
	dockerCmd(c, "start", "test2")
	stateErr = inspectField(c, "test2", "State.Error")
	// Expected to not have state error but got one
	c.Assert(stateErr, checker.Equals, "")
}
*/

func (s *DockerSuite) TestCliStartMultipleContainersBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	// Windows does not support --link
	testRequires(c, DaemonIsLinux)

	// run a container named 'parent' and create two container link to `parent`
	dockerCmd(c, "run", "-d", "--name", "parent", "busybox", "top")

	for _, container := range []string{"child-first", "child-second"} {
		dockerCmd(c, "create", "--name", container, "--link", "parent:parent", "busybox", "top")
	}

	// stop 'parent' container
	dockerCmd(c, "stop", "parent")

	out := inspectField(c, "parent", "State.Running")
	// Container should be stopped
	c.Assert(out, checker.Equals, "false")

	// start all the three containers, container `child_first` start first which should be failed
	// container 'parent' start second and then start container 'child_second'
	expOut := "Cannot link to a non running container"
	expErr := "failed to start containers: child-first"
	out, _, err := dockerCmdWithError("start", "child-first", "parent", "child-second")
	// err shouldn't be nil because start will fail
	c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))
	// output does not correspond to what was expected
	if !(strings.Contains(out, expOut) || strings.Contains(err.Error(), expErr)) {
		c.Fatalf("Expected out: %v with err: %v  but got out: %v with err: %v", expOut, expErr, out, err)
	}

	for container, expected := range map[string]string{"parent": "true", "child-first": "false", "child-second": "true"} {
		out := inspectField(c, container, "State.Running")
		// Container running state wrong
		c.Assert(out, checker.Equals, expected)
	}
}

func (s *DockerSuite) TestCliStartAttachMultipleContainers(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	// run  multiple containers to test
	for _, container := range []string{"test1", "test2", "test3"} {
		dockerCmd(c, "run", "-d", "--name", container, "busybox", "top")
	}

	// stop all the containers
	for _, container := range []string{"test1", "test2", "test3"} {
		dockerCmd(c, "stop", container)
	}

	// test start and attach multiple containers at once, expected error
	for _, option := range []string{"-a", "-i", "-ai"} {
		out, _, err := dockerCmdWithError("start", option, "test1", "test2", "test3")
		// err shouldn't be nil because start will fail
		c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))
		// output does not correspond to what was expected
		c.Assert(out, checker.Contains, "You cannot start and attach multiple containers at once.")
	}

	// confirm the state of all the containers be stopped
	for container, expected := range map[string]string{"test1": "false", "test2": "false", "test3": "false"} {
		out := inspectField(c, container, "State.Running")
		// Container running state wrong
		c.Assert(out, checker.Equals, expected)
	}
}
