package main

import (
	"fmt"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"time"
)

/// test invalid url //////////////////////////////////////////////////////////////////////////
func (s *DockerSuite) TestCliLoadFromUrlInvalidUrlProtocal(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	invalidURL := "ftp://image-tarball.s3.amazonaws.com/test/public/helloworld.tar"
	output, exitCode, err := dockerCmdWithError("load", "-i", invalidURL)
	c.Assert(output, checker.Equals, "Error response from daemon: Bad request parameters: Get "+invalidURL+": unsupported protocol scheme \"ftp\"\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidUrlHost(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	invalidHost := "invalidhost"
	invalidURL := "http://" + invalidHost + "/test/public/helloworld.tar"
	output, exitCode, err := dockerCmdWithError("load", "-i", invalidURL)
	c.Assert(output, checker.Contains, "Error response from daemon: Bad request parameters: Get "+invalidURL+": dial tcp: lookup invalidhost")
	c.Assert(output, checker.Contains, "no such host\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidUrlPath(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/notexist.tar")
	c.Assert(output, checker.Equals, "Error response from daemon: Bad request parameters: Got HTTP status code >= 400: 403 Forbidden\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

//test invalid ContentType and ContentLength///////////////////////////////////////////////////////////////////////////
func (s *DockerSuite) TestCliLoadFromUrlInvalidContentType(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/readme.txt")
	c.Assert(output, checker.Equals, "Error response from daemon: Download failed: URL MIME type should be one of: binary/octet-stream, application/octet-stream, application/x-tar, application/x-gzip, application/x-bzip, application/x-xz, but now is text/plain\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidContentLengthTooLarge(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	const MaxLength = 4294967295
	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/largefile.tar")
	c.Assert(output, checker.Contains, fmt.Sprintf("should be greater than zero and less than or equal to %v\n", MaxLength))
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

//test invalid content///////////////////////////////////////////////////////////////////////////
func (s *DockerSuite) TestCliLoadFromUrlInvalidContentLengthZero(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	const MaxLength = 4294967295
	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/emptyfile.tar")
	c.Assert(output, checker.Equals, fmt.Sprintf("Error response from daemon: Bad request parameters: The size of the image archive file is 0, should be greater than zero and less than or equal to %v\n", MaxLength))
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidContentUnrelated(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/readme.tar")
	c.Assert(output, checker.Contains, "invalid argument\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidUntarFail(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	output, exitCode, err := dockerCmdWithError("load", "-i", "http://image-tarball.s3.amazonaws.com/test/public/nottar.tar")
	c.Assert(output, checker.Contains, "Untar re-exec error: exit status 1: output: unexpected EOF\n")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliLoadFromUrlInvalidContentIncomplete(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllImages()
	url := "http://image-tarball.s3.amazonaws.com/test/public/helloworld-no-repositories.tgz"
	output, exitCode, err := dockerCmdWithError("load", "-i", url)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	images, _ := dockerCmd(c, "images", "hello-world")
	c.Assert(images, checker.Contains, "hello-world")

	deleteAllImages()

	//// load this image will be OK, but after delete this image, there is a residual image with <none> tag occur.
	//url = "http://image-tarball.s3.amazonaws.com/test/public/helloworld-no-manifest.tgz"
	//output, exitCode, err = dockerCmdWithError("load", "-i", url)
	//c.Assert(output, check.Not(checker.Contains), "has been loaded.")
	//c.Assert(exitCode, checker.Equals, 0)
	//c.Assert(err, checker.IsNil)
	//
	//images, _ = dockerCmd(c, "images", "hello-world")
	//c.Assert(images, checker.Contains, "hello-world")
	//
	//deleteAllImages()

	url = "http://image-tarball.s3.amazonaws.com/test/public/helloworld-no-layer.tgz"
	output, exitCode, err = dockerCmdWithError("load", "-i", url)
	c.Assert(output, checker.Contains, "json: no such file or directory")
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)

	images, _ = dockerCmd(c, "images", "hello-world")
	c.Assert(images, check.Not(checker.Contains), "hello-world")

	deleteAllImages()
}

//test normal///////////////////////////////////////////////////////////////////////////
func (s *DockerSuite) TestCliLoadFromUrlValidBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	publicURL := "http://image-tarball.s3.amazonaws.com/test/public/helloworld.tar"
	output, exitCode, err := dockerCmdWithError("load", "-i", publicURL)
	c.Assert(output, checker.Contains, "hello-world:latest(sha256:")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	images, _ := dockerCmd(c, "images", "hello-world")
	c.Assert(images, checker.Contains, "hello-world")
}

func (s *DockerSuite) TestCliLoadFromUrlValidCompressedArchiveBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	extAry := [...]string{"tar.gz", "tgz", "tar.bz2", "tar.xz"}

	for _, val := range extAry {
		publicURL := "http://image-tarball.s3.amazonaws.com/test/public/helloworld." + val
		output, exitCode, err := dockerCmdWithError("load", "-i", publicURL)
		c.Assert(output, checker.Contains, "hello-world:latest(sha256:")
		c.Assert(output, checker.Contains, "has been loaded.\n")
		c.Assert(exitCode, checker.Equals, 0)
		c.Assert(err, checker.IsNil)

		time.Sleep(1 * time.Second)
	}
}

func (s *DockerSuite) TestCliLoadFromUrlWithQuiet(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	publicURL := "http://image-tarball.s3.amazonaws.com/test/public/helloworld.tar"
	out, _, _ := dockerCmdWithStdoutStderr(c, "load", "-q", "-i", publicURL)
	c.Assert(out, check.Equals, "")

	images, _ := dockerCmd(c, "images", "hello-world")
	c.Assert(images, checker.Contains, "hello-world")
}

func (s *DockerSuite) TestCliLoadFromUrlMultipeImageBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	multiImgURL := "http://image-tarball.s3.amazonaws.com/test/public/busybox_alpine.tar"
	dockerCmd(c, "load", "-i", multiImgURL)

	images, _ := dockerCmd(c, "images", "busybox")
	c.Assert(images, checker.Contains, "busybox")

	images, _ = dockerCmd(c, "images", "alpine")
	c.Assert(images, checker.Contains, "alpine")
}
