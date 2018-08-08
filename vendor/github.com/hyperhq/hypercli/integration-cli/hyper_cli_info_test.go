package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// ensure docker info succeeds
func (s *DockerSuite) TestCliInfoEnsureSucceedsBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	out, _ := dockerCmd(c, "info")

	// always shown fields
	stringsToCheck := []string{
		"Containers",
		" Running",
		" Paused",
		" Stopped",
		"Images",
		"Server Version",
		"Storage Driver",
		"Execution Driver",
		"Plugins",
		" Volume",
		" Network",
		" Authorization",
		"Kernel Version",
		"CPUs",
		"Total Memory",
		"ID",
		"Debug mode (client)",
		"Debug mode (server)",
	}

	//if utils.ExperimentalBuild() {
	//	stringsToCheck = append(stringsToCheck, "Experimental: true")
	//}

	for _, linePrefix := range stringsToCheck {
		c.Assert(out, checker.Contains, linePrefix, check.Commentf("couldn't find string %v in output", linePrefix))
	}
}

//comment: not support discoveryBackend
//// TestInfoDiscoveryBackend verifies that a daemon run with `--cluster-advertise` and
//// `--cluster-store` properly show the backend's endpoint in info output.
//func (s *DockerSuite) TestCliInfoDiscoveryBackend(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, SameHostDaemon, DaemonIsLinux)
//
//	d := NewDaemon(c)
//	discoveryBackend := "consul://consuladdr:consulport/some/path"
//	discoveryAdvertise := "1.1.1.1:2375"
//	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), fmt.Sprintf("--cluster-advertise=%s", discoveryAdvertise))
//	c.Assert(err, checker.IsNil)
//	defer d.Stop()
//
//	out, err := d.Cmd("info")
//	c.Assert(err, checker.IsNil)
//	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster store: %s\n", discoveryBackend))
//	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster advertise: %s\n", discoveryAdvertise))
//}

//comment: not support discoveryBackend
//// TestInfoDiscoveryInvalidAdvertise verifies that a daemon run with
//// an invalid `--cluster-advertise` configuration
//func (s *DockerSuite) TestCliInfoDiscoveryInvalidAdvertise(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, SameHostDaemon, DaemonIsLinux)
//
//	d := NewDaemon(c)
//	discoveryBackend := "consul://consuladdr:consulport/some/path"
//
//	// --cluster-advertise with an invalid string is an error
//	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), "--cluster-advertise=invalid")
//	c.Assert(err, checker.Not(checker.IsNil))
//
//	// --cluster-advertise without --cluster-store is also an error
//	err = d.Start("--cluster-advertise=1.1.1.1:2375")
//	c.Assert(err, checker.Not(checker.IsNil))
//}

//comment: not support discoveryBackend
//// TestInfoDiscoveryAdvertiseInterfaceName verifies that a daemon run with `--cluster-advertise`
//// configured with interface name properly show the advertise ip-address in info output.
//func (s *DockerSuite) TestCliInfoDiscoveryAdvertiseInterfaceName(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, SameHostDaemon, Network, DaemonIsLinux)
//
//	d := NewDaemon(c)
//	discoveryBackend := "consul://consuladdr:consulport/some/path"
//	discoveryAdvertise := "eth0"
//
//	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), fmt.Sprintf("--cluster-advertise=%s:2375", discoveryAdvertise))
//	c.Assert(err, checker.IsNil)
//	defer d.Stop()
//
//	iface, err := net.InterfaceByName(discoveryAdvertise)
//	c.Assert(err, checker.IsNil)
//	addrs, err := iface.Addrs()
//	c.Assert(err, checker.IsNil)
//	c.Assert(len(addrs), checker.GreaterThan, 0)
//	ip, _, err := net.ParseCIDR(addrs[0].String())
//	c.Assert(err, checker.IsNil)
//
//	out, err := d.Cmd("info")
//	c.Assert(err, checker.IsNil)
//	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster store: %s\n", discoveryBackend))
//	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster advertise: %s:2375\n", ip.String()))
//}

func (s *DockerSuite) TestCliInfoDisplaysRunningContainersBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "busybox", "top")
	out, _ := dockerCmd(c, "info")
	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 0))
}

//comment: not support pause status
//func (s *DockerSuite) TestCliInfoDisplaysPausedContainers(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, DaemonIsLinux)
//
//	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
//	cleanedContainerID := strings.TrimSpace(out)
//
//	dockerCmd(c, "pause", cleanedContainerID)
//
//	out, _ = dockerCmd(c, "info")
//	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
//	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 0))
//	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 1))
//	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 0))
//}

func (s *DockerSuite) TestCliInfoDisplaysStoppedContainers(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	outAry := strings.Split(out, "\n")
	c.Assert(len(outAry), checker.GreaterOrEqualThan, 2)
	cleanedContainerID := outAry[len(outAry)-2]

	dockerCmd(c, "stop", cleanedContainerID)

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 1))
}

// not support daemon
//func (s *DockerSuite) TestCliInfoDebug(c *check.C) {
//	printTestCaseName(); defer printTestDuration(time.Now())
//	testRequires(c, SameHostDaemon, DaemonIsLinux)
//
//	d := NewDaemon(c)
//	err := d.Start("--debug")
//	c.Assert(err, checker.IsNil)
//	defer d.Stop()
//
//	out, err := d.Cmd("--debug", "info")
//	c.Assert(err, checker.IsNil)
//	c.Assert(out, checker.Contains, "Debug mode (client): true\n")
//	c.Assert(out, checker.Contains, "Debug mode (server): true\n")
//	c.Assert(out, checker.Contains, "File Descriptors")
//	c.Assert(out, checker.Contains, "Goroutines")
//	c.Assert(out, checker.Contains, "System Time")
//	c.Assert(out, checker.Contains, "EventsListeners")
//	c.Assert(out, checker.Contains, "Docker Root Dir")
//}
