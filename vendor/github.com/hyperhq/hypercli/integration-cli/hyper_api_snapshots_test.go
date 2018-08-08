package main

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/hyperhq/hyper-api/types"
)

func (s *DockerSuite) TestApiSnapshotsCreate(c *check.C) {
	dockerCmd(c, "volume", "create", "--name", "test")

	status, b, err := sockRequest("POST", "/snapshots/create?name=snap-test&volume=test", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated, check.Commentf(string(b)))

	var snap types.Snapshot
	err = json.Unmarshal(b, &snap)
	c.Assert(err, checker.IsNil)
}

func (s *DockerSuite) TestApiSnapshotsList(c *check.C) {
	dockerCmd(c, "volume", "create", "--name", "test")

	sockRequest("POST", "/snapshots/create?name=snap-test&volume=test", nil)

	status, b, err := sockRequest("GET", "/snapshots", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var snapshots types.SnapshotsListResponse
	c.Assert(json.Unmarshal(b, &snapshots), checker.IsNil)

	c.Assert(len(snapshots.Snapshots), checker.Equals, 1, check.Commentf("\n%v", snapshots.Snapshots))
}

func (s *DockerSuite) TestApiSnapshotsRemove(c *check.C) {
	dockerCmd(c, "volume", "create", "--name", "test")

	sockRequest("POST", "/snapshots/create?name=snap-test&volume=test", nil)

	status, b, err := sockRequest("GET", "/snapshots", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var snapshots types.SnapshotsListResponse
	c.Assert(json.Unmarshal(b, &snapshots), checker.IsNil)
	c.Assert(len(snapshots.Snapshots), checker.Equals, 1, check.Commentf("\n%v", snapshots.Snapshots))

	snap := snapshots.Snapshots[0]
	status, data, err := sockRequest("DELETE", "/snapshots/"+snap.Name, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent, check.Commentf(string(data)))
}

func (s *DockerSuite) TestApiSnapshotsInspect(c *check.C) {
	dockerCmd(c, "volume", "create", "--name", "test")

	sockRequest("POST", "/snapshots/create?name=snap-test&volume=test", nil)

	status, b, err := sockRequest("GET", "/snapshots", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var snapshots types.SnapshotsListResponse
	c.Assert(json.Unmarshal(b, &snapshots), checker.IsNil)
	c.Assert(len(snapshots.Snapshots), checker.Equals, 1, check.Commentf("\n%v", snapshots.Snapshots))

	var snap types.Snapshot
	status, b, err = sockRequest("GET", "/snapshots/snap-test", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf(string(b)))
	c.Assert(json.Unmarshal(b, &snap), checker.IsNil)
	c.Assert(snap.Name, checker.Equals, "snap-test")
}
