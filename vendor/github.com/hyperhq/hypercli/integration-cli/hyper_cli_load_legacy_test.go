package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strings"
	"time"
)

func (s *DockerSuite) TestCliLoadFromUrlLegacyImageArchiveFile(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageName := "ubuntu"
	legacyImageURL := "http://image-tarball.s3.amazonaws.com/test/public/old/ubuntu_1.8.tar.gz"
	imageURL := "http://image-tarball.s3.amazonaws.com/test/public/ubuntu.tar.gz"

	//load legacy image(saved by docker 1.8)
	output, exitCode, err := dockerCmdWithError("load", "-i", legacyImageURL)
	c.Assert(output, checker.Contains, "Starting to download and load the image archive, please wait...\n")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, imageName)
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)

	/////////////////////////////////////////////////////////////////////
	//load new format image(saved by docker 1.10)
	output, exitCode, err = dockerCmdWithError("load", "-i", imageURL)
	c.Assert(output, checker.Contains, "Starting to download and load the image archive, please wait...\n")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, imageName)
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)

	/////////////////////////////////////////////////////////////////////
	//delete single layer
	output, _ = dockerCmd(c, "images", "-q", imageName)
	imageID := strings.Split(output, "\n")[0]
	c.Assert(imageID, checker.Not(checker.Equals), "")

	output, _ = dockerCmd(c, "rmi", "--no-prune", imageID)
	c.Assert(output, checker.Contains, "Untagged:")
	c.Assert(output, checker.Contains, "Deleted:")

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, "<none>")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)
	imageID = strings.Split(output, "\n")[0]

	output, _ = dockerCmd(c, "images", "-a")
	c.Assert(output, checker.Contains, "<none>")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 6)

	/////////////////////////////////////////////////////////////////////
	//delete all rest layer
	output, _ = dockerCmd(c, "images", "-q")
	imageID = strings.Split(output, "\n")[0]
	c.Assert(imageID, checker.Not(checker.Equals), "")

	output, _ = dockerCmd(c, "rmi", imageID)
	c.Assert(output, checker.Contains, "Deleted:")

	output, _ = dockerCmd(c, "images")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 2)

	output, _ = dockerCmd(c, "images", "-a")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 2)
}
