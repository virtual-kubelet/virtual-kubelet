package main

import (
	"fmt"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"os"
	"time"
)

//Prerequisite: update image balance to 2 in tenant collection of hypernetes in mongodb
//db.tenant.update({tenantid:"<tenant_id>"},{$set:{"resourceinfo.balance.images":2}})
func (s *DockerSuite) TestCliLoadUrlBasicFromPublicURLWithQuota(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllImages()

	helloworldURL := "http://image-tarball.s3.amazonaws.com/test/public/helloworld.tar"
	multiImgURL := "http://image-tarball.s3.amazonaws.com/test/public/busybox_alpine.tar"
	ubuntuURL := "http://image-tarball.s3.amazonaws.com/test/public/ubuntu.tar.gz"
	//exceedQuotaMsg := "Exceeded quota, please either delete images, or email support@hyper.sh to request increased quota"
	exceedQuotaMsg := "you do not have enough quota"

	///// [init] /////
	// balance 3, images 0
	out, _ := dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 0")

	///// [step 1] load new hello-world image /////
	// balance 3 -> 2, image: 0 -> 1
	output, exitCode, err := dockerCmdWithError("load", "-i", helloworldURL)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	checkImage(c, true, "hello-world")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 1")

	///// [step 2] load hello-world image again /////
	// balance 2 -> 2, image 1 -> 1
	output, exitCode, err = dockerCmdWithError("load", "-i", helloworldURL)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	checkImage(c, true, "hello-world")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 1")

	///// [step 3] load multiple image(busybox+alpine) /////
	// balance 2 -> 2, image 1 -> 1
	output, exitCode, err = dockerCmdWithError("load", "-i", multiImgURL)
	c.Assert(output, checker.Contains, exceedQuotaMsg)
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(err, checker.NotNil)

	checkImage(c, false, "busybox")
	checkImage(c, false, "alpine")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 1")

	///// [step 4] load new ubuntu image /////
	// balance 2 -> 1, image 1 -> 2
	output, exitCode, err = dockerCmdWithError("load", "-i", ubuntuURL)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	checkImage(c, true, "ubuntu")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 2")

	///// [step 5] remove hello-world image /////
	// balance 1 -> 2, image 2 -> 1
	images, _ := dockerCmd(c, "rmi", "-f", "hello-world")
	c.Assert(images, checker.Contains, "Untagged: hello-world:latest")

	checkImage(c, false, "hello-world")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 1")

	///// [step 6] remove busybox and ubuntu image /////
	// balance 2 -> 3, image 1 -> 0
	images, _ = dockerCmd(c, "rmi", "-f", "ubuntu:latest")
	c.Assert(images, checker.Contains, "Untagged: ubuntu:latest")

	checkImage(c, false, "ubuntu")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 0")

	///// [step 7] load multiple image(busybox+alpine) again /////
	// balance 3 -> 0, image 0 -> 3
	output, exitCode, err = dockerCmdWithError("load", "-i", multiImgURL)
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	checkImage(c, true, "busybox")
	checkImage(c, true, "alpine")

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Images: 3")
}

func (s *DockerSuite) TestCliLoadUrlBasicFromAWSS3PreSignedURL(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	deleteAllImages()

	s3Region := "us-west-1"
	s3Bucket := "image-tarball"
	s3Key := "test/private/cirros.tar"
	preSignedUrl, err_ := generateS3PreSignedURL(s3Region, s3Bucket, s3Key)
	c.Assert(err_, checker.IsNil)
	time.Sleep(1 * time.Second)

	output, err := dockerCmd(c, "load", "-i", preSignedUrl)
	if err != 0 {
		fmt.Printf("preSignedUrl:[%v]\n", preSignedUrl)
		fmt.Printf("output:\n%v\n", output)
	}
	c.Assert(output, checker.Contains, "has been loaded.")
	c.Assert(err, checker.Equals, 0)

	checkImage(c, true, "cirros")
}

func (s *DockerSuite) TestCliLoadUrlBasicFromBasicAuthURL(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	urlWithAuth := os.Getenv("URL_WITH_BASIC_AUTH")
	c.Assert(urlWithAuth, checker.NotNil)

	dockerCmd(c, "load", "-i", urlWithAuth)

	images, _ := dockerCmd(c, "images", "ubuntu")
	c.Assert(images, checker.Contains, "ubuntu")
}
