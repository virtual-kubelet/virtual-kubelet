package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

//TODO: fix #90
/*func (s *DockerSuite) TestApiLogsNoStdoutNorStderr(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	name := "logs-test"
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "/bin/sh")

	status, body, err := sockRequest("GET", fmt.Sprintf("/containers/%s/logs", name), nil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	c.Assert(err, checker.IsNil)

	expected := "Bad parameters: you must choose at least one stream"
	if !bytes.Contains(body, []byte(expected)) {
		c.Fatalf("Expected %s, got %s", expected, string(body[:]))
	}
}*/

// Regression test for #12704
func (s *DockerSuite) TestApiLogsFollowEmptyOutput(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	name := "logs-test"
	t0 := time.Now()
	dockerCmd(c, "run", "-d", "-t", "--name", name, "busybox", "sleep", "10")

	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name), bytes.NewBuffer(nil), "")
	t1 := time.Now()
	c.Assert(err, checker.IsNil)
	body.Close()
	elapsed := t1.Sub(t0).Seconds()
	if elapsed > 40.0 {
		c.Fatalf("HTTP response was not immediate (elapsed %.1fs)", elapsed)
	}
}

func (s *DockerSuite) TestApiLogsContainerNotFound(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	name := "nonExistentContainer"
	resp, _, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/logs?follow=1&stdout=1&stderr=1&tail=all", name), bytes.NewBuffer(nil), "")
	c.Assert(err, checker.IsNil)
	c.Assert(resp.StatusCode, checker.Equals, http.StatusNotFound)
}
