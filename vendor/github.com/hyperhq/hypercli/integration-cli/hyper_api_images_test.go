package main

import (
	"net/http"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strings"
)

func (s *DockerSuite) TestApiImagesSearchJSONContentType(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, Network)

	res, b, err := sockRequestRaw("GET", "/images/search?term=test", nil, "application/json")
	c.Assert(err, check.IsNil)
	b.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
}

func (s *DockerSuite) TestApiImagesLoad(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	postData := map[string]interface{}{
		"fromSrc": "http://image-tarball.s3.amazonaws.com/test/public/helloworld.tar.gz",
		"quiet":   false,
	}
	//debugEndpoint = "/images/load"

	status, resp, err := sockRequest("POST", "/images/load", postData)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	expected := "{\"status\":\"Starting to download and load the image archive, please wait...\"}"
	c.Assert(strings.TrimSpace(string(resp)), checker.Contains, expected)

	expected = "has been loaded.\"}"
	c.Assert(strings.TrimSpace(string(resp)), checker.Contains, expected)
}
