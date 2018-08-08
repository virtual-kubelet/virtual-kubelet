package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliLinksPingUnlinkedContainers(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, exitCode, err := dockerCmdWithError("run", "--rm", "busybox", "sh", "-c", "ping -c 1 alias1 -W 5 && ping -c 1 alias2 -W 5")

	// run ping failed with error
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("error: %v", err))
}

// Test for appropriate error when calling --link with an invalid target container
func (s *DockerSuite) TestCliLinksInvalidContainerTarget(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _, err := dockerCmdWithError("run", "--link", "bogus:alias", "busybox", "true")

	// an invalid container target should produce an error
	c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))
	// an invalid container target should produce an error
	c.Assert(out, checker.Contains, "No such container")
}

func (s *DockerSuite) TestCliLinksPingLinkedContainersBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "container1", "--hostname", "fred", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "container2", "--hostname", "wilma", "busybox", "top")

	runArgs := []string{"run", "--rm", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "sh", "-c"}
	pingCmd := "ping -c 1 %s -W 5 && ping -c 1 %s -W 5"

	// test ping by alias, ping by name, and ping by hostname
	// 1. Ping by alias
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "alias1", "alias2"))...)
	// 2. Ping by container name
	/* FIXME https://github.com/hyperhq/hypercli/issues/78
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "container1", "container2"))...)
	*/
	// 3. Ping by hostname
	dockerCmd(c, append(runArgs, fmt.Sprintf(pingCmd, "fred", "wilma"))...)

}

func (s *DockerSuite) TestCliLinksPingLinkedContainersAfterRename(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	idA := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	idB := strings.TrimSpace(out)
	dockerCmd(c, "rename", "container1", "container-new")
	dockerCmd(c, "run", "--rm", "--link", "container-new:alias1", "--link", "container2:alias2", "busybox", "sh", "-c", "ping -c 1 alias1 -W 5 && ping -c 1 alias2 -W 5")
	dockerCmd(c, "kill", idA)
	dockerCmd(c, "kill", idB)

}

func (s *DockerSuite) TestCliLinksInspectLinksStarted(c *check.C) {
	/* FIXME https://github.com/hyperhq/hypercli/issues/76
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	var (
		expected = map[string]struct{}{"/container1:/testinspectlink/alias1": {}, "/container2:/testinspectlink/alias2": {}}
		result   []string
	)
	dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "top")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	err := unmarshalJSON([]byte(links), &result)
	c.Assert(err, checker.IsNil)

	output := convertSliceOfStringsToMap(result)

	c.Assert(output, checker.DeepEquals, expected)
	*/
}

func (s *DockerSuite) TestCliLinksInspectLinksStopped(c *check.C) {
	/* FIXME https://github.com/hyperhq/hypercli/issues/76
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	var (
		expected = map[string]struct{}{"/container1:/testinspectlink/alias1": {}, "/container2:/testinspectlink/alias2": {}}
		result   []string
	)
	dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "container2", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name", "testinspectlink", "--link", "container1:alias1", "--link", "container2:alias2", "busybox", "true")
	links := inspectFieldJSON(c, "testinspectlink", "HostConfig.Links")

	err := unmarshalJSON([]byte(links), &result)
	c.Assert(err, checker.IsNil)

	output := convertSliceOfStringsToMap(result)

	c.Assert(output, checker.DeepEquals, expected)
	*/
}

func (s *DockerSuite) TestCliLinksNotStartedParentNotFail(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "create", "--name=first", "busybox", "top")
	dockerCmd(c, "create", "--name=second", "--link=first:first", "busybox", "top")
	dockerCmd(c, "start", "first")
}

func (s *DockerSuite) TestCliLinksHostsFilesInject(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, ExecSupport)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-itd", "--name", "one", "busybox", "top")
	idOne := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "-itd", "--name", "two", "--link", "one:onetwo", "busybox", "top")
	idTwo := strings.TrimSpace(out)

	c.Assert(waitRun(idTwo), checker.IsNil)

	contentOne, err := readContainerFileWithExec(idOne, "/etc/hosts")
	c.Assert(err, checker.IsNil, check.Commentf("contentOne: %s", string(contentOne)))

	contentTwo, err := readContainerFileWithExec(idTwo, "/etc/hosts")
	c.Assert(err, checker.IsNil, check.Commentf("contentTwo: %s", string(contentTwo)))
	// Host is not present in updated hosts file
	c.Assert(string(contentTwo), checker.Contains, "onetwo")
}

func (s *DockerSuite) TestCliLinksUpdateOnRestart(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, ExecSupport)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "one", "busybox", "top")
	out, _ := dockerCmd(c, "run", "-d", "--name", "two", "--link", "one:onetwo", "--link", "one:one", "busybox", "top")
	id := strings.TrimSpace(string(out))

	realIP := inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")
	content, err := readContainerFileWithExec(id, "/etc/hosts")
	c.Assert(err, checker.IsNil)

	getIP := func(hosts []byte, hostname string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(\S*)\t%s`, regexp.QuoteMeta(hostname)))
		matches := re.FindSubmatch(hosts)
		c.Assert(matches, checker.NotNil, check.Commentf("Hostname %s have no matches in hosts", hostname))
		return string(matches[1])
	}
	ip := getIP(content, "one")
	c.Assert(ip, checker.Equals, realIP)

	ip = getIP(content, "onetwo")
	c.Assert(ip, checker.Equals, realIP)

	dockerCmd(c, "restart", "one")
	realIP = inspectField(c, "one", "NetworkSettings.Networks.bridge.IPAddress")

	content, err = readContainerFileWithExec(id, "/etc/hosts")
	c.Assert(err, checker.IsNil, check.Commentf("content: %s", string(content)))
	ip = getIP(content, "one")
	c.Assert(ip, checker.Equals, realIP)

	ip = getIP(content, "onetwo")
	c.Assert(ip, checker.Equals, realIP)
}

func (s *DockerSuite) TestCliLinksEnvs(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "-e", "e1=", "-e", "e2=v2", "-e", "e3=v3=v3", "--name=first", "busybox", "top")
	out, _ := dockerCmd(c, "run", "--name=second", "--link=first:first", "busybox", "env")
	/* FIXME
	c.Assert(out, checker.Contains, "FIRST_ENV_e1=\n")
	*/
	c.Assert(out, checker.Contains, "FIRST_ENV_e2=v2")
	c.Assert(out, checker.Contains, "FIRST_ENV_e3=v3=v3")
}

func (s *DockerSuite) TestCliLinkShortDefinition(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--name", "shortlinkdef", "busybox", "top")

	cid := strings.TrimSpace(out)
	c.Assert(waitRun(cid), checker.IsNil)

	out, _ = dockerCmd(c, "run", "-d", "--name", "link2", "--link", "shortlinkdef", "busybox", "top")

	cid2 := strings.TrimSpace(out)
	c.Assert(waitRun(cid2), checker.IsNil)

	/* FIXME https://github.com/hyperhq/hypercli/issues/76
	links := inspectFieldJSON(c, cid2, "HostConfig.Links")
	c.Assert(links, checker.Equals, "[\"/shortlinkdef:/link2/shortlinkdef\"]")
	*/
}

func (s *DockerSuite) TestCliLinksMultipleWithSameName(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name=upstream-a", "busybox", "top")
	dockerCmd(c, "run", "-d", "--name=upstream-b", "busybox", "top")
	dockerCmd(c, "run", "--link", "upstream-a:upstream", "--link", "upstream-b:upstream", "busybox", "sh", "-c", "ping -c 1 upstream")
}
