package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

//this test case will test all the apis about fip
func (s *DockerSuite) TestApiFip(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	endpoint := "/fips/allocate?count=1"

	status, body, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusCreated)
	c.Assert(err, checker.IsNil)

	var IP []string
	err = json.Unmarshal(body, &IP)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	containerID := strings.TrimSpace(out)

	endpoint = "/fips/associate?ip=" + IP[0] + "&container=" + containerID
	status, body, err = sockRequest("POST", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(err, checker.IsNil)

	endpoint = "/fips"
	status, body, err = sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)
	c.Assert(string(body), checker.Contains, IP[0], check.Commentf("should get IP %s", IP[0]))
	c.Assert(string(body), checker.Contains, containerID, check.Commentf("should get containerID %s", containerID))

	endpoint = "/fips/disassociate?container=" + containerID
	status, body, err = sockRequest("POST", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	time.Sleep(5 * time.Second)
	endpoint = "/fips/release?ip=" + IP[0]
	status, body, err = sockRequest("POST", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(err, checker.IsNil)

	//make sure that IP[0] has been released
	endpoint = "/fips"
	status, body, err = sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)
	c.Assert(string(body), checker.Not(checker.Contains), IP[0], check.Commentf("should not get IP %s", IP[0]))
}
