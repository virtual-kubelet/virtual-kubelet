package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/hyperhq/hyper-api/types/container"
)

func checkValidGraphDriver(c *check.C, name string) {
	if name != "rbd" && name != "devicemapper" && name != "overlay" && name != "vfs" && name != "zfs" && name != "btrfs" && name != "aufs" {
		c.Fatalf("%v is not a valid graph driver name", name)
	}
}

func (s *DockerSuite) TestCliInspectImageBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageTest := "busybox:1.26.2"
	ensureImageExist(c, imageTest)
	// It is important that this ID remain stable. If a code change causes
	// it to be different, this is equivalent to a cache bust when pulling
	// a legacy-format manifest. If the check at the end of this function
	// fails, fix the difference in the image serialization instead of
	// updating this hash.
	// Warning: before test , make sure imageTest and imageTestId are match
	imageTestID := "sha256:c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27"
	id := inspectField(c, imageTest, "Id")

	c.Assert(id, checker.Equals, imageTestID)
}

func (s *DockerSuite) TestCliInspectInt64(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "inspect-test", "busybox", "true")
	inspectOut := inspectField(c, "inspect-test", "HostConfig.Memory")
	c.Assert(inspectOut, checker.Equals, "0")
}

func (s *DockerSuite) TestCliInspectDefault(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch the container JSON.
	//If the container JSON is not available, it will go for the image JSON.

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")
	containerID := getIDfromOutput(c, out)

	inspectOut := inspectField(c, "busybox", "Id")
	c.Assert(strings.TrimSpace(inspectOut), checker.Equals, containerID)
}

func (s *DockerSuite) TestCliInspectStatusBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	//	defer unpauseAllContainers()
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	out = getIDfromOutput(c, out)

	inspectOut := inspectField(c, out, "State.Status")
	c.Assert(inspectOut, checker.Equals, "running")

	dockerCmd(c, "stop", out)
	inspectOut = inspectField(c, out, "State.Status")
	c.Assert(inspectOut, checker.Equals, "exited")

}

func (s *DockerSuite) TestCliInspectTypeFlagContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON State.Running field. If the field is true, it's a container.

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.State.Running}}'"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	c.Assert(out, checker.Equals, "true\n") // not a container JSON
}

func (s *DockerSuite) TestCliInspectTypeFlagWithNoContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	//Run this test on an image named busybox. docker inspect will try to fetch container
	//JSON. Since there is no container named busybox and --type=container, docker inspect will
	//not try to get the image JSON. It will throw an error.

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "busybox", "true")

	_, _, err := dockerCmdWithError("inspect", "--type=container", "busybox")
	// docker inspect should fail, as there is no container named busybox
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestCliInspectTypeFlagWithImage(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch image
	//JSON as --type=image. if there is no image with name busybox, docker inspect
	//will throw an error.

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, _ := dockerCmd(c, "inspect", "--type=image", "busybox")
	c.Assert(out, checker.Not(checker.Contains), "State") // not an image JSON
}

func (s *DockerSuite) TestCliInspectTypeFlagWithInvalidValue(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fail
	//as --type=foobar is not a valid value for the flag.

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, exitCode, err := dockerCmdWithError("inspect", "--type=foobar", "busybox")
	c.Assert(err, checker.NotNil, check.Commentf("%s", exitCode))
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("%s", err))
	c.Assert(out, checker.Contains, "not a valid value for --type")
}

func (s *DockerSuite) TestCliInspectImageFilterInt(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageTest := "busybox"
	ensureImageExist(c, imageTest)
	out := inspectField(c, imageTest, "Size")

	size, err := strconv.Atoi(out)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect size of the image: %s, %v", out, err))

	//now see if the size turns out to be the same
	formatStr := fmt.Sprintf("--format='{{eq .Size %d}}'", size)
	out, _ = dockerCmd(c, "inspect", formatStr, imageTest)
	result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	c.Assert(err, checker.IsNil)
	c.Assert(result, checker.Equals, true)
}

func (s *DockerSuite) TestCliInspectContainerFilterIntBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := getIDfromOutput(c, out)

	out = inspectField(c, id, "State.ExitCode")

	exitCode, err := strconv.Atoi(out)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect exitcode of the container: %s, %v", out, err))

	//now get the exit code to verify
	formatStr := fmt.Sprintf("--format='{{eq .State.ExitCode %d}}'", exitCode)
	out, _ = dockerCmd(c, "inspect", formatStr, id)
	result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	c.Assert(err, checker.IsNil)
	c.Assert(result, checker.Equals, true)
}

func (s *DockerSuite) TestCliInspectImageGraphDriver(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	imageTest := "busybox"
	ensureImageExist(c, imageTest)
	name := inspectField(c, imageTest, "GraphDriver.Name")

	checkValidGraphDriver(c, name)

	if name != "devicemapper" {
		c.Skip("requires devicemapper graphdriver")
	}

	deviceID := inspectField(c, imageTest, "GraphDriver.Data.DeviceId")

	_, err := strconv.Atoi(deviceID)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceId of the image: %s, %v", deviceID, err))

	deviceSize := inspectField(c, imageTest, "GraphDriver.Data.DeviceSize")

	_, err = strconv.ParseUint(deviceSize, 10, 64)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceSize of the image: %s, %v", deviceSize, err))
}

// #14947
func (s *DockerSuite) TestCliInspectTimesAsRFC3339Nano(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	id := getIDfromOutput(c, out)

	startedAt := inspectField(c, id, "State.StartedAt")
	finishedAt := inspectField(c, id, "State.FinishedAt")
	created := inspectField(c, id, "Created")

	_, err := time.Parse(time.RFC3339Nano, startedAt)
	c.Assert(err, checker.IsNil)
	_, err = time.Parse(time.RFC3339Nano, finishedAt)
	c.Assert(err, checker.IsNil)
	_, err = time.Parse(time.RFC3339Nano, created)
	c.Assert(err, checker.IsNil)

	created = inspectField(c, "busybox", "Created")

	_, err = time.Parse(time.RFC3339Nano, created)
	c.Assert(err, checker.IsNil)
}

// #15633
func (s *DockerSuite) TestCliInspectLogConfigNoType(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "create", "--name=test", "busybox")
	var logConfig container.LogConfig

	out := inspectFieldJSON(c, "test", "HostConfig.LogConfig")

	err := json.NewDecoder(strings.NewReader(out)).Decode(&logConfig)
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))

	c.Assert(logConfig.Type, checker.Equals, "json-file")
	c.Assert(logConfig.Config["max-file"], checker.Equals, "10", check.Commentf("%v", logConfig))
}

func (s *DockerSuite) TestCliInspectNoSizeFlagContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON SizeRw and SizeRootFs field. If there is no flag --size/-s, there are no size fields.

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	c.Assert(strings.TrimSpace(out), check.Equals, "<nil>,<nil>", check.Commentf("Exepcted not to display size info: %s", out))
}

func (s *DockerSuite) TestCliInspectSizeFlagContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _ := dockerCmd(c, "inspect", "-s", "--type=container", formatStr, "busybox")
	sz := strings.Split(out, ",")

	c.Assert(strings.TrimSpace(sz[0]), check.Not(check.Equals), "<nil>")
	c.Assert(strings.TrimSpace(sz[1]), check.Not(check.Equals), "<nil>")
}

func (s *DockerSuite) TestCliInspectSizeFlagImage(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _, err := dockerCmdWithError("inspect", "-s", "--type=image", formatStr, "busybox")

	// Template error rather than <no value>
	// This is a more correct behavior because images don't have sizes associated.
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestCliInspectTempateError(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	// Template parsing error for both the container and image.
	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=container1", "-d", "busybox", "top")

	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='Format container: {{.ThisDoesNotExist}}'", "container1")
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(out, checker.Contains, "Template parsing error")

	out, _, err = dockerCmdWithError("inspect", "--type=image", "--format='Format container: {{.ThisDoesNotExist}}'", "busybox")
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestCliInspectJSONFields(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")
	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='{{.HostConfig.Dns}}'", "busybox")

	c.Assert(err, check.IsNil)
	c.Assert(out, checker.Equals, "[]\n")
}

func (s *DockerSuite) TestCliInspectByPrefix(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	imageTest := "busybox"
	ensureImageExist(c, imageTest)
	id := inspectField(c, imageTest, "Id")
	c.Assert(id, checker.HasPrefix, "sha256:")

	id2 := inspectField(c, id[:12], "Id")
	c.Assert(id, checker.Equals, id2)

	id3 := inspectField(c, strings.TrimPrefix(id, "sha256:")[:12], "Id")
	c.Assert(id, checker.Equals, id3)
}

func (s *DockerSuite) TestCliInspectStopWhenNotFound(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")
	dockerCmd(c, "run", "--name=not-shown", "-d", "busybox", "top")
	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='{{.Name}}'", "busybox", "missing", "not-shown")

	c.Assert(err, checker.Not(check.IsNil))
	c.Assert(out, checker.Contains, "busybox")
	c.Assert(out, checker.Not(checker.Contains), "not-shown")
	c.Assert(out, checker.Contains, "Error: No such container: missing")
}
