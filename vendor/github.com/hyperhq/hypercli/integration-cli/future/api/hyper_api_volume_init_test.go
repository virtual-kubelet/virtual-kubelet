package main

import (
	"net/http"

	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiVolumeInit(c *check.C) {
	source := "https://raw.githubusercontent.com/hyperhq/hypercli/master/README.md"
	volName := "hyperclitestvol"
	dockerCmd(c, "volume", "create", "--name="+volName)
	options := types.VolumesInitializeRequest{
		Reload: false,
		Volume: make([]types.VolumeInitDesc, 0),
	}
	options.Volume = append(options.Volume, types.VolumeInitDesc{Name: volName, Source: source})
	status, b, err := sockRequest("POST", "/volumes/initialize", options)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK, check.Commentf(string(b)))
	dockerCmd(c, "volume", "rm", volName)
}

func (s *DockerSuite) TestApiVolumeReload(c *check.C) {
	source := "https://raw.githubusercontent.com/hyperhq/hypercli/master/README.md"
	volName := "hyperclitestvol"
	dockerCmd(c, "volume", "create", "--name="+volName)
	dockerCmd(c, "volume", "init", source+":"+volName)
	options := types.VolumesInitializeRequest{
		Reload: true,
		Volume: make([]types.VolumeInitDesc, 0),
	}
	options.Volume = append(options.Volume, types.VolumeInitDesc{Name: volName, Source: source})
	status, b, err := sockRequest("POST", "/volumes/initialize", options)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK, check.Commentf(string(b)))
	dockerCmd(c, "volume", "rm", volName)
}
