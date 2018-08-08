package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiInspectContainerResponse(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	ensureImageExist(c, "busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)
	keysBase := []string{"Id", "State", "Created", "Path", "Args", "Config", "Image", "NetworkSettings",
		"ResolvConfPath", "HostnamePath", "HostsPath", "LogPath", "Name", "Driver", "MountLabel", "ProcessLabel", "GraphDriver"}

	type acase struct {
		version string
		keys    []string
	}

	var cases []acase

	cases = []acase{
		{"v1.23", append(keysBase, "Mounts")},
	}

	for _, cs := range cases {
		body := getInspectBodyWithoutVersion(c, cleanedContainerID)

		var inspectJSON map[string]interface{}
		err := json.Unmarshal(body, &inspectJSON)
		c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version %s", cs.version))

		for _, key := range cs.keys {
			_, ok := inspectJSON[key]
			c.Check(ok, checker.True, check.Commentf("%s does not exist in response for version %s", key, cs.version))
		}

		_, ok := inspectJSON["Path"].(bool)
		c.Assert(ok, checker.False, check.Commentf("Path of `true` should not be converted to boolean `true` via JSON marshalling"))
	}
}
