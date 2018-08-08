package main

import (
	//"encoding/json"
	"fmt"
	//"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/go-check/check"
)

//TODO: get exited container log
// Regression test for #8832
func (s *DockerSuite) TestCliLogsFollowSlowStdoutConsumer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", `usleep 600000;yes X | head -c 200000`)
	time.Sleep(10 * time.Second)
	id := strings.TrimSpace(out)

	stopSlowRead := make(chan bool)

	go func() {
		exec.Command(dockerBinary, "stop", id).Run()
		time.Sleep(10 * time.Second)
		stopSlowRead <- true
	}()

	logCmd := exec.Command(dockerBinary, "logs", "-f", id)
	stdout, err := logCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	c.Assert(logCmd.Start(), checker.IsNil)

	// First read slowly
	bytes1, err := consumeWithSpeed(stdout, 10, 50*time.Millisecond, stopSlowRead)
	c.Assert(err, checker.IsNil)

	// After the container has finished we can continue reading fast
	bytes2, err := consumeWithSpeed(stdout, 32*1024, 0, nil)
	c.Assert(err, checker.IsNil)

	actual := bytes1 + bytes2
	expected := 200000
	c.Assert(actual, checker.Equals, expected)
}

//TODO: get exited container log
func (s *DockerSuite) TestCliLogsFollowStopped(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "hello")
	time.Sleep(5 * time.Second)

	id := strings.TrimSpace(out)
	dockerCmd(c, "stop", id)
	time.Sleep(5 * time.Second)

	logsCmd := exec.Command(dockerBinary, "logs", "-f", id)
	c.Assert(logsCmd.Start(), checker.IsNil)

	errChan := make(chan error)
	go func() {
		errChan <- logsCmd.Wait()
		close(errChan)
	}()

	select {
	case err := <-errChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(10 * time.Second):
		c.Fatal("Following logs is hanged")
	}
}
