package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strings"
	"time"
)

func (s *DockerSuite) TestCliLoadFromUrlLargeImageArchiveFile(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageName := "consol/centos-xfce-vnc"
	imageUrl := "http://image-tarball.s3.amazonaws.com/test/public/consol_centos-xfce-vnc.tar" //1.53GB

	output, exitCode, err := dockerCmdWithError("load", "-i", imageUrl)
	c.Assert(output, checker.Contains, "Starting to download and load the image archive, please wait...\n")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	images, _ := dockerCmd(c, "images")
	c.Assert(images, checker.Contains, imageName)
	c.Assert(len(strings.Split(images, "\n")), checker.Equals, 3)
}
