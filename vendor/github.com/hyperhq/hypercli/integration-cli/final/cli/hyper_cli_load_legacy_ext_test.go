package main

import (
	"time"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strings"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"os"
	//"fmt"
)

func (s *DockerSuite) TestCliLoadFromUrlLegacyImageArchiveFileWithQuota(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageName := "ubuntu";
	legacyImageUrl := "http://image-tarball.s3.amazonaws.com/test/public/old/ubuntu_1.8.tar.gz"
	imageUrl := "http://image-tarball.s3.amazonaws.com/test/public/ubuntu.tar.gz"


	/////////////////////////////////////////////////////////////////////
	checkImageQuota(c, 2)
	//load legacy image(saved by docker 1.8)
	output, exitCode, err := dockerCmdWithError("load", "-i", legacyImageUrl)
	c.Assert(output, checker.Contains, "Starting to download and load the image archive, please wait...\n")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, imageName)
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)


	/////////////////////////////////////////////////////////////////////
	checkImageQuota(c, 1)
	//load new format image(saved by docker 1.10)
	output, exitCode, err = dockerCmdWithError("load", "-i", imageUrl)
	c.Assert(output, checker.Contains, "Start to download and load the image archive, please wait...\n")
	c.Assert(output, checker.Contains, "has been loaded.\n")
	c.Assert(exitCode, checker.Equals, 0)
	c.Assert(err, checker.IsNil)

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, imageName)
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)


	/////////////////////////////////////////////////////////////////////
	checkImageQuota(c, 1)
	//delete single layer
	output, _ = dockerCmd(c, "images", "-q", imageName)
	imageId := strings.Split(output, "\n")[0]
	c.Assert(imageId, checker.Not(checker.Equals), "")

	output, _ = dockerCmd(c, "rmi", "--no-prune", imageId)
	c.Assert(output, checker.Contains, "Untagged:")
	c.Assert(output, checker.Contains, "Deleted:")

	checkImageQuota(c, 1)

	output, _ = dockerCmd(c, "images")
	c.Assert(output, checker.Contains, "<none>")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 3)
	imageId = strings.Split(output, "\n")[0]

	output, _ = dockerCmd(c, "images", "-a")
	c.Assert(output, checker.Contains, "<none>")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 6)


	/////////////////////////////////////////////////////////////////////
	checkImageQuota(c, 1)
	//delete all rest layer
	output, _ = dockerCmd(c, "images", "-q")
	imageId = strings.Split(output, "\n")[0]
	c.Assert(imageId, checker.Not(checker.Equals), "")

	output, _ = dockerCmd(c, "rmi", imageId)
	c.Assert(output, checker.Contains, "Deleted:")

	checkImageQuota(c, 2)

	output, _ = dockerCmd(c, "images")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 2)

	output, _ = dockerCmd(c, "images", "-a")
	c.Assert(len(strings.Split(output, "\n")), checker.Equals, 2)
}


//func (s *DockerSuite) TestCliLoadFromUrlLegacyCheckImageQuota(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, DaemonIsLinux)
//	checkImageQuota(c, 2)
//}

func checkImageQuota(c *check.C, expected int) {

	//collection struct: credential
	type Credential struct {
		TenantId string `bson:"tenantId"`
	}

	//collection struct: tenant
	type Total struct {
		Images int `bson:"images"`
	}
	type Balance struct {
		Images int `bson:"images"`
	}
	type Resourceinfo struct {
		Total   Total `bson:"total"`
		Balance Balance `bson:"balance"`
	}
	type Tenant struct {
		Resourceinfo Resourceinfo `bson:"resourceinfo"`
	}


	///////////////////////////////////////////
	//init connection to mongodb
	session, err := mgo.Dial(os.Getenv("MONGODB_URL"))
	if err != nil {
		panic(err)
	}
	defer session.Close()
	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)
	db := session.DB("hypernetes")

	///////////////////////////////////////////
	// query tenantId by accessKey
	collection := db.C("credentials")
	resultCred := Credential{}

	//countNum, _ := collection.Find(condition).Count()
	//fmt.Println("\ncount:\n", countNum)

	collection.Find(bson.M{"accessKey": os.Getenv("ACCESS_KEY")}).Select(bson.M{"tenantId": 1}).One(&resultCred)
	c.Assert(resultCred.TenantId, checker.NotNil)
	tenantId := resultCred.TenantId


	///////////////////////////////////////////
	// query image quota by tenant
	collection = db.C("tenant")
	resultTenant := Tenant{}

	//countNum, _ := collection.Find(condition).Count()
	//fmt.Println("\ncount:\n", countNum)

	collection.Find(bson.M{"tenantid": tenantId}).Select(bson.M{"resourceinfo": 1}).One(&resultTenant)
	//fmt.Printf("total images: %v\n", resultTenant.Resourceinfo.Total.Images)
	//fmt.Printf("balance images: %v\n", resultTenant.Resourceinfo.Balance.Images)
	totalImages := resultTenant.Resourceinfo.Total.Images
	balanceImages := resultTenant.Resourceinfo.Balance.Images

	c.Assert(totalImages, checker.GreaterThan, 0)
	c.Assert(balanceImages, checker.LessOrEqualThan, totalImages)
	c.Assert(balanceImages, checker.Equals, expected)
}
