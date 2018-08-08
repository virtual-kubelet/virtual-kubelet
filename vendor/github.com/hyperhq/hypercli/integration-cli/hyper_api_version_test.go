package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/hyperhq/hyper-api/types"
)

func (s *DockerSuite) TestApiGetVersion(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	status, body, err := sockRequest("GET", "/version", nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	var v types.Version

	c.Assert(json.Unmarshal(body, &v), checker.IsNil)

	c.Assert(v.Version, checker.Equals, dockerversion.Version, check.Commentf("Version mismatch"))
}

func (s *DockerSuite) TestApiSimpleCreate(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh"},
	}
	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	type createResp struct {
		ID      string
		Warning string
	}
	//var container createResp
	fmt.Println(string(b))
	c.Assert(status, checker.Equals, http.StatusCreated)
}
