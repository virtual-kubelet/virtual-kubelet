package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// Make sure we can create a simple container with some args
func (s *DockerSuite) TestCliCreateArgs(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	// TODO Windows. This requires further investigation for porting to
	// Windows CI. Currently fails.
	if daemonPlatform == "windows" {
		c.Skip("Fails on Windows CI")
	}
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox", "command", "arg1", "arg2", "arg with space")

	cleanedContainerID := getIDfromOutput(c, out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

	containers := []struct {
		ID      string
		Created time.Time
		Path    string
		Args    []string
		Image   string
	}{}

	err := json.Unmarshal([]byte(out), &containers)
	c.Assert(err, check.IsNil, check.Commentf("Error inspecting the container: %s", err))
	c.Assert(containers, checker.HasLen, 1)

	cont := containers[0]
	c.Assert(string(cont.Path), checker.Equals, "command", check.Commentf("Unexpected container path. Expected command, received: %s", cont.Path))

	b := false
	expected := []string{"arg1", "arg2", "arg with space"}
	for i, arg := range expected {
		if arg != cont.Args[i] {
			b = true
			break
		}
	}
	if len(cont.Args) != len(expected) || b {
		c.Fatalf("Unexpected args. Expected %v, received: %v", expected, cont.Args)
	}

}

// Make sure we can set hostconfig options too
func (s *DockerSuite) TestCliCreateHostConfigBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox", "echo")

	cleanedContainerID := getIDfromOutput(c, out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

	containers := []struct {
		HostConfig *struct {
			PublishAllPorts bool
		}
	}{}

	err := json.Unmarshal([]byte(out), &containers)
	c.Assert(err, check.IsNil, check.Commentf("Error inspecting the container: %s", err))
	c.Assert(containers, checker.HasLen, 1)

	cont := containers[0]
	c.Assert(cont.HostConfig, check.NotNil, check.Commentf("Expected HostConfig, got none"))
	c.Assert(cont.HostConfig.PublishAllPorts, check.NotNil, check.Commentf("Expected PublishAllPorts, got false"))
}

// "test123" should be printed by docker create + start
func (s *DockerSuite) TestCliCreateEchoStdoutBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox", "echo", "test123")

	cleanedContainerID := getIDfromOutput(c, out)

	out, _ = dockerCmd(c, "start", "-ai", cleanedContainerID)
	time.Sleep(5 * time.Second)
	c.Assert(out, checker.Equals, "test123\n", check.Commentf("container should've printed 'test123', got %q", out))
}

func (s *DockerSuite) TestCliCreateVolumesCreated(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon)
	prefix := "/"
	if daemonPlatform == "windows" {
		prefix = `c:\`
	}

	name := "test_create_volume"
	dockerCmd(c, "create", "--name", name, "-v", prefix+"foo", "busybox")

	dir, err := inspectMountSourceField(name, prefix+"foo")
	c.Assert(err, check.IsNil, check.Commentf("Error getting volume host path: %q", err))

	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		c.Fatalf("Volume was not created")
	}
	if err != nil {
		c.Fatalf("Error statting volume host path: %q", err)
	}

}

func (s *DockerSuite) TestCliCreateLabels(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	name := "test-create-labels"
	expected := map[string]string{"k1": "v1", "k2": "v2", "sh.hyper.fip": "", "sh_hyper_instancetype": "s4"}
	dockerCmd(c, "create", "--name", name, "-l", "k1=v1", "--label", "k2=v2", "busybox")

	actual := make(map[string]string)
	inspectFieldAndMarshall(c, name, "Config.Labels", &actual)

	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("Expected %s got %s", expected, actual)
	}
}

func (s *DockerSuite) TestCliCreateRM(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	// Test to make sure we can 'rm' a new container that is in
	// "Created" state, and has ever been run. Test "rm -f" too.

	// create a container
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox")
	cID := getIDfromOutput(c, out)

	dockerCmd(c, "rm", cID)

	// Now do it again so we can "rm -f" this time
	out, _ = dockerCmd(c, "create", "busybox")

	cID = strings.TrimSpace(out)
	dockerCmd(c, "rm", "-f", cID)
}

func (s *DockerSuite) TestCliCreateModeIpcContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	// Uses Linux specific functionality (--ipc)
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, NotUserNamespace)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox")
	id := strings.TrimSpace(out)

	dockerCmd(c, "create", fmt.Sprintf("--ipc=container:%s", id), "busybox")
}

/*
func (s *DockerTrustSuite) TestCliCreateTrustedCreate(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	repoName := s.setupTrustedImage(c, "trusted-create")

	// Try create
	createCmd := exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf("Missing expected output on trusted push:\n%s", out))

	dockerCmd(c, "rmi", repoName)

	// Try untrusted create to ensure we pushed the tag to the registry
	createCmd = exec.Command(dockerBinary, "create", "--disable-content-trust=true", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf("Missing expected output on trusted create with --disable-content-trust:\n%s", out))

}

func (s *DockerTrustSuite) TestCliCreateUntrustedCreate(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	repoName := fmt.Sprintf("%v/dockercliuntrusted/createtest", privateRegistryURL)
	withTagName := fmt.Sprintf("%s:latest", repoName)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", withTagName)
	dockerCmd(c, "push", withTagName)
	dockerCmd(c, "rmi", withTagName)

	// Try trusted create on untrusted tag
	createCmd := exec.Command(dockerBinary, "create", withTagName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(string(out), checker.Contains, fmt.Sprintf("does not have trust data for %s", repoName), check.Commentf("Missing expected output on trusted create:\n%s", out))

}

func (s *DockerTrustSuite) TestCliCreateTrustedIsolatedCreate(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	repoName := s.setupTrustedImage(c, "trusted-isolated-create")

	// Try create
	createCmd := exec.Command(dockerBinary, "--config", "/tmp/docker-isolated-create", "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf("Missing expected output on trusted push:\n%s", out))

	dockerCmd(c, "rmi", repoName)
}

func (s *DockerTrustSuite) TestCliCreateWhenCertExpired(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	c.Skip("Currently changes system time, causing instability")
	repoName := s.setupTrustedImage(c, "trusted-create-expired")

	// Certificates have 10 years of expiration
	elevenYearsFromNow := time.Now().Add(time.Hour * 24 * 365 * 11)

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try create
		createCmd := exec.Command(dockerBinary, "create", repoName)
		s.trustedCmd(createCmd)
		out, _, err := runCommandWithOutput(createCmd)
		c.Assert(err, check.Not(check.IsNil))
		c.Assert(string(out), checker.Contains, "could not validate the path to a trusted root", check.Commentf("Missing expected output on trusted create in the distant future:\n%s", out))
	})

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try create
		createCmd := exec.Command(dockerBinary, "create", "--disable-content-trust", repoName)
		s.trustedCmd(createCmd)
		out, _, err := runCommandWithOutput(createCmd)
		c.Assert(err, check.Not(check.IsNil))
		c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf("Missing expected output on trusted create in the distant future:\n%s", out))

	})
}

func (s *DockerTrustSuite) TestCliCreateTrustedCreateFromBadTrustServer(c *check.C) {
	printTestCaseName(); defer printTestDuration(time.Now())
	repoName := fmt.Sprintf("%v/dockerclievilcreate/trusted:latest", privateRegistryURL)
	evilLocalConfigDir, err := ioutil.TempDir("", "evil-local-config-dir")
	c.Assert(err, check.IsNil)

	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push:\n%s", out))

	dockerCmd(c, "rmi", repoName)

	// Try create
	createCmd := exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf("Missing expected output on trusted push:\n%s", out))

	dockerCmd(c, "rmi", repoName)

	// Kill the notary server, start a new "evil" one.
	s.not.Close()
	s.not, err = newTestNotary(c)
	c.Assert(err, check.IsNil)

	// In order to make an evil server, lets re-init a client (with a different trust dir) and push new data.
	// tag an image and upload it to the private registry
	dockerCmd(c, "--config", evilLocalConfigDir, "tag", "busybox", repoName)

	// Push up to the new server
	pushCmd = exec.Command(dockerBinary, "--config", evilLocalConfigDir, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push:\n%s", out))

	// Now, try creating with the original client from this new trust server. This should fail.
	createCmd = exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(string(out), checker.Contains, "valid signatures did not meet threshold", check.Commentf("Missing expected output on trusted push:\n%s", out))

}
*/

func (s *DockerSuite) TestCliCreateWithWorkdir(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	// TODO Windows. This requires further investigation for porting to
	// Windows CI. Currently fails.
	if daemonPlatform == "windows" {
		c.Skip("Fails on Windows CI")
	}
	name := "foo"

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	dir := prefix + slash + "home" + slash + "foo" + slash + "bar"

	dockerCmd(c, "create", "--name", name, "-w", dir, "busybox")
}
