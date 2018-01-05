package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/go-connections/nat"
	"github.com/go-check/check"
)

// "test123" should be printed by docker run
func (s *DockerSuite) TestCliRunEchoStdoutBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "busybox", "echo", "test123")
	if out != "test123\n" {
		c.Fatalf("container should've printed 'test123', got '%s'", out)
	}
}

// "test" should be printed
func (s *DockerSuite) TestCliRunEchoNamedContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--name", "testfoonamedcontainer", "busybox", "echo", "test")
	if out != "test\n" {
		c.Errorf("container should've printed 'test'")
	}
}

// docker run should not leak file descriptors. This test relies on Unix
// specific functionality and cannot run on Windows.
func (s *DockerSuite) TestCliRunLeakyFileDescriptors(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "busybox", "ls", "-C", "/proc/self/fd")

	// normally, we should only get 0, 1, and 2, but 3 gets created by "ls" when it does "opendir" on the "fd" directory
	if out != "0  1  2  3\n" {
		c.Errorf("container should've printed '0  1  2  3', not: %s", out)
	}
}

// it should be possible to lookup Google DNS
// this will fail when Internet access is unavailable
func (s *DockerSuite) TestCliRunLookupGoogleDns(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, Network, NotArm)

	image := DefaultImage
	if daemonPlatform == "windows" {
		// nslookup isn't present in Windows busybox. Is built-in.
		image = WindowsBaseImage
	}
	dockerCmd(c, "run", image, "nslookup", "google.com")
}

// the exit code should be 0
func (s *DockerSuite) TestCliRunExitCodeZero(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "busybox", "true")
}

// the exit code should be 1
func (s *DockerSuite) TestCliRunExitCodeOne(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, exitCode, err := dockerCmdWithError("run", "busybox", "false")
	if err != nil && !strings.Contains("exit status 1", fmt.Sprintf("%s", err)) {
		c.Fatal(err)
	}
	if exitCode != 1 {
		c.Errorf("container should've exited with exit code 1. Got %d", exitCode)
	}
}

// it should be possible to pipe in data via stdin to a process running in a container
func (s *DockerSuite) TestCliRunStdinPipe(c *check.C) {
	/* FIXME https://github.com/hyperhq/hypercli/issues/14
	// TODO Windows: This needs some work to make compatible.
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-i", "-a", "stdin", "busybox", "cat")
	runCmd.Stdin = strings.NewReader("blahblah")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)
	dockerCmd(c, "stop", out)

	logsOut, _ := dockerCmd(c, "logs", out)

	containerLogs := strings.TrimSpace(logsOut)
	if containerLogs != "blahblah" {
		c.Errorf("logs didn't print the container's logs %s", containerLogs)
	}

	dockerCmd(c, "rm", out)
	*/
}

// the container's ID should be printed when starting a container in detached mode
func (s *DockerSuite) TestCliRunDetachedContainerIDPrinting(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	time.Sleep(5 * time.Second)

	out = strings.TrimSpace(out)
	dockerCmd(c, "stop", out)
	time.Sleep(5 * time.Second)

	rmOut, _ := dockerCmd(c, "rm", out)
	time.Sleep(5 * time.Second)

	rmOut = strings.TrimSpace(rmOut)
	if rmOut != out {
		c.Errorf("rm didn't print the container ID %s %s", out, rmOut)
	}
}

// the working directory should be set correctly
func (s *DockerSuite) TestCliRunWorkingDirectory(c *check.C) {
	// TODO Windows: There's a Windows bug stopping this from working.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dir := "/root"
	image := "busybox"
	if daemonPlatform == "windows" {
		dir = `/windows`
		image = WindowsBaseImage
	}

	// First with -w
	out, _ := dockerCmd(c, "run", "-w", dir, image, "pwd")
	out = strings.TrimSpace(out)
	if out != dir {
		c.Errorf("-w failed to set working directory")
	}

	// Then with --workdir
	out, _ = dockerCmd(c, "run", "--workdir", dir, image, "pwd")
	out = strings.TrimSpace(out)
	if out != dir {
		c.Errorf("--workdir failed to set working directory")
	}
}

func (s *DockerSuite) TestCliRunLinksContainerWithContainerNameBasic(c *check.C) {
	// TODO Windows: This test cannot run on a Windows daemon as the networking
	// settings are not populated back yet on inspect.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-i", "-t", "-d", "--name", "parent", "busybox")

	ip := inspectField(c, "parent", "NetworkSettings.Networks.bridge.IPAddress")

	out, _ := dockerCmd(c, "run", "--link", "parent:test", "busybox", "/bin/cat", "/etc/hosts")
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container name to link target failed")
	}
}

//test --link use container id to link target
func (s *DockerSuite) TestCliRunLinksContainerWithContainerId(c *check.C) {
	// TODO Windows: This test cannot run on a Windows daemon as the networking
	// settings are not populated back yet on inspect.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	cID, _ := dockerCmd(c, "run", "-i", "-t", "-d", "busybox")

	cID = strings.TrimSpace(cID)
	ip := inspectField(c, cID, "NetworkSettings.Networks.bridge.IPAddress")

	out, _ := dockerCmd(c, "run", "--link", cID+":test", "busybox", "/bin/cat", "/etc/hosts")
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container id to link target failed")
	}
}

// this tests verifies the ID format for the container
func (s *DockerSuite) TestCliRunVerifyContainerID(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, exit, err := dockerCmdWithError("run", "-d", "busybox", "true")
	if err != nil {
		c.Fatal(err)
	}
	if exit != 0 {
		c.Fatalf("expected exit code 0 received %d", exit)
	}

	match, err := regexp.MatchString("^[0-9a-f]{64}$", strings.TrimSuffix(out, "\n"))
	if err != nil {
		c.Fatal(err)
	}
	if !match {
		c.Fatalf("Invalid container ID: %s", out)
	}
}

func (s *DockerSuite) TestCliRunExitCode(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	var (
		exit int
		err  error
	)
	pullImageIfNotExist("busybox")
	_, exit, err = dockerCmdWithError("run", "busybox", "/bin/sh", "-c", "exit 72")

	if err == nil {
		c.Fatal("should not have a non nil error")
	}
	if exit != 72 {
		c.Fatalf("expected exit code 72 received %d", exit)
	}
}

func (s *DockerSuite) TestCliRunUserDefaults(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	expected := "uid=0(root) gid=0(root)"
	if daemonPlatform == "windows" {
		expected = "uid=1000(SYSTEM) gid=1000(SYSTEM)"
	}
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "busybox", "id")
	if !strings.Contains(out, expected) {
		c.Fatalf("expected '%s' got %s", expected, out)
	}
}

func (s *DockerSuite) TestCliRunTwoConcurrentContainersBasic(c *check.C) {
	// TODO Windows. There are two bugs in TP4 which means this test cannot
	// be reliably enabled. The first is a race condition where sometimes
	// HCS CreateComputeSystem() will fail "Invalid class string". #4985252 and
	// #4493430.
	//
	// The second, which is seen more readily by increasing the number of concurrent
	// containers to 5 or more, is that CSRSS hangs. This may fixed in the TP4 ZDP.
	// #4898773.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	sleepTime := "10"
	if daemonPlatform == "windows" {
		sleepTime = "5" // Make more reliable on Windows
	}
	group := sync.WaitGroup{}
	group.Add(2)

	errChan := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer group.Done()
			_, _, err := dockerCmdWithError("run", "busybox", "sleep", sleepTime)
			errChan <- err
		}()
	}

	group.Wait()
	close(errChan)

	for err := range errChan {
		c.Assert(err, check.IsNil)
	}
}

func (s *DockerSuite) TestCliRunEnvironment(c *check.C) {
	/* FIXME
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	pullImageIfNotExist("busybox")
	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-h", "testing", "-e=FALSE=true", "-e=TRUE=", "-e=TRICKY=", "-e=HOME=", "busybox", "env")
	cmd.Env = append(os.Environ(),
		"TRUE=false",
		"TRICKY=tri\ncky\n",
	)

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnv := strings.Split(strings.TrimSpace(out), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOSTNAME=testing",
		"FALSE=true",
		"TRUE=false",
		"TRICKY=tri",
		"cky",
		"",
		"HOME=/root",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
	*/
}

func (s *DockerSuite) TestCliRunEnvironmentErase(c *check.C) {
	/* FIXME
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	pullImageIfNotExist("busybox")

	// Test to make sure that when we use -e on env vars that are
	// not set in our local env that they're removed (if present) in
	// the container

	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-e", "FOO", "-e", "HOSTNAME", "busybox", "env")
	cmd.Env = appendBaseEnv([]string{})

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnv := strings.Split(strings.TrimSpace(out), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
	*/
}

func (s *DockerSuite) TestCliRunEnvironmentOverride(c *check.C) {
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	// Test to make sure that when we use -e on env vars that are
	// already in the env that we're overriding them

	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-e", "HOSTNAME", "-e", "HOME=/root2", "busybox", "env")
	cmd.Env = appendBaseEnv([]string{"HOSTNAME=bar"})

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnv := strings.Split(strings.TrimSpace(out), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root2",
		"HOSTNAME=bar",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestCliRunContainerNetworkBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	if daemonPlatform == "windows" {
		// Windows busybox does not have ping. Use built in ping instead.
		dockerCmd(c, "run", WindowsBaseImage, "ping", "-n", "1", "127.0.0.1")
	} else {
		dockerCmd(c, "run", "busybox", "ping", "-c", "1", "127.0.0.1")
	}
}

// #7851 hostname outside container shows FQDN, inside only shortname
// For testing purposes it is not required to set host's hostname directly
// and use "--net=host" (as the original issue submitter did), as the same
// codepath is executed with "docker run -h <hostname>".  Both were manually
// tested, but this testcase takes the simpler path of using "run -h .."
func (s *DockerSuite) TestCliRunFullHostnameSet(c *check.C) {
	// TODO Windows: -h is not yet functional.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-h", "foo.bar.baz", "busybox", "hostname")
	if actual := strings.Trim(out, "\r\n"); actual != "foo.bar.baz" {
		c.Fatalf("expected hostname 'foo.bar.baz', received %s", actual)
	}
}

func (s *DockerSuite) TestCliRunDeviceNumbers(c *check.C) {
	// Not applicable on Windows as /dev/ is a Unix specific concept
	// TODO: NotUserNamespace could be removed here if "root" "root" is replaced w user
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "busybox", "sh", "-c", "ls -l /dev/null")
	deviceLineFields := strings.Fields(out)
	deviceLineFields[6] = ""
	deviceLineFields[7] = ""
	deviceLineFields[8] = ""
	expected := []string{"crw-rw-rw-", "1", "root", "root", "1,", "3", "", "", "", "/dev/null"}

	if !(reflect.DeepEqual(deviceLineFields, expected)) {
		c.Fatalf("expected output\ncrw-rw-rw- 1 root root 1, 3 May 24 13:29 /dev/null\n received\n %s\n", out)
	}
}

func (s *DockerSuite) TestCliRunThatCharacterDevicesActLikeCharacterDevices(c *check.C) {
	// Not applicable on Windows as /dev/ is a Unix specific concept
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "busybox", "sh", "-c", "dd if=/dev/zero of=/zero bs=1k count=5 2> /dev/null ; du -h /zero")
	if actual := strings.Trim(out, "\r\n"); actual[0] == '0' {
		c.Fatalf("expected a new file called /zero to be create that is greater than 0 bytes long, but du says: %s", actual)
	}
}

func (s *DockerSuite) TestCliRunRootWorkdir(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--workdir", "/", "busybox", "pwd")
	expected := "/\n"
	if daemonPlatform == "windows" {
		expected = "C:" + expected
	}
	if out != expected {
		c.Fatalf("pwd returned %q (expected %s)", s, expected)
	}
}

// Verify that a container gets default DNS when only localhost resolvers exist
func (s *DockerSuite) TestCliRunDnsDefaultOptions(c *check.C) {
	// Not applicable on Windows as this is testing Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	// preserve original resolv.conf for restoring after test
	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		c.Fatalf("/etc/resolv.conf does not exist")
	}
	// defer restored original conf
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", origResolvConf, 0644); err != nil {
			c.Fatal(err)
		}
	}()

	// test 3 cases: standard IPv4 localhost, commented out localhost, and IPv6 localhost
	// 2 are removed from the file at container start, and the 3rd (commented out) one is ignored by
	// GetNameservers(), leading to a replacement of nameservers with the default set
	tmpResolvConf := []byte("nameserver 127.0.0.1\n#nameserver 127.0.2.1\nnameserver ::1")
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	actual, _ := dockerCmd(c, "run", "busybox", "cat", "/etc/resolv.conf")
	// check that the actual defaults are appended to the commented out
	// localhost resolver (which should be preserved)
	// NOTE: if we ever change the defaults from google dns, this will break
	expected := "#nameserver 127.0.2.1\n\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if actual != expected {
		c.Fatalf("expected resolv.conf be: %q, but was: %q", expected, actual)
	}
}

// Regression test for #6983
func (s *DockerSuite) TestCliRunAttachStdErrOnlyTTYMode(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stderr", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestCliRunAttachStdOutOnlyTTYMode(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stdout", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestCliRunAttachStdOutAndErrTTYMode(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stdout", "-a", "stderr", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Test for #10388 - this will run the same test as TestRunAttachStdOutAndErrTTYMode
// but using --attach instead of -a to make sure we read the flag correctly
func (s *DockerSuite) TestCliRunAttachWithDetach(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	cmd := exec.Command(dockerBinary, "run", "-d", "--attach", "stdout", "busybox", "true")
	_, stderr, _, err := runCommandWithStdoutStderr(cmd)
	if err == nil {
		c.Fatal("Container should have exited with error code different than 0")
	} else if !strings.Contains(stderr, "Conflicting options: -a and -d") {
		c.Fatal("Should have been returned an error with conflicting options -a and -d")
	}
}

func (s *DockerSuite) TestCliRunState(c *check.C) {
	// TODO Windows: This needs some rework as Windows busybox does not support top
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

	id := strings.TrimSpace(out)
	state := inspectField(c, id, "State.Running")
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	/* FIXME
	pid1 := inspectField(c, id, "State.Pid")
	if pid1 == "0" {
		c.Fatal("Container state Pid 0")
	}
	*/

	dockerCmd(c, "stop", id)
	state = inspectField(c, id, "State.Running")
	if state != "false" {
		c.Fatal("Container state is 'running'")
	}
	/* FIXME
	pid2 := inspectField(c, id, "State.Pid")
	if pid2 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}
	*/

	dockerCmd(c, "start", id)
	state = inspectField(c, id, "State.Running")
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	/* FIXME
	pid3 := inspectField(c, id, "State.Pid")
	if pid3 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}
	*/
}

// TestRunWorkdirExistsAndIsFile checks that if 'docker run -w' with existing file can be detected
func (s *DockerSuite) TestCliRunWorkdirExistsAndIsFile(c *check.C) {
	/* FIXME
	printTestCaseName(); defer printTestDuration(time.Now())
	existingFile := "/bin/cat"
	expected := "Cannot mkdir: /bin/cat is not a directory"
	if daemonPlatform == "windows" {
		existingFile = `\windows\system32\ntdll.dll`
		expected = "The directory name is invalid"
	}

	out, exitCode, err := dockerCmdWithError("run", "-w", existingFile, "busybox")
	if !(err != nil && exitCode == 125 && strings.Contains(out, expected)) {
		c.Fatalf("Docker must complains about making dir with exitCode 125 but we got out: %s, exitCode: %d", out, exitCode)
	}
	*/
}

func (s *DockerSuite) TestCliRunExitOnStdinClose(c *check.C) {
	/* FIXME
	printTestCaseName(); defer printTestDuration(time.Now())
	name := "testrunexitonstdinclose"

	meow := "/bin/cat"
	delay := 60
	if daemonPlatform == "windows" {
		meow = "cat"
		delay = 60
	}
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--name", name, "-i", "busybox", meow)

	stdin, err := runCmd.StdinPipe()
	if err != nil {
		c.Fatal(err)
	}
	stdout, err := runCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := runCmd.Start(); err != nil {
		c.Fatal(err)
	}
	if _, err := stdin.Write([]byte("hello\n")); err != nil {
		c.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		c.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if line != "hello" {
		c.Fatalf("Output should be 'hello', got '%q'", line)
	}
	if err := stdin.Close(); err != nil {
		c.Fatal(err)
	}
	finish := make(chan error)
	go func() {
		finish <- runCmd.Wait()
		close(finish)
	}()
	select {
	case err := <-finish:
		c.Assert(err, check.IsNil)
	case <-time.After(time.Duration(delay) * time.Second):
		c.Fatal("docker run failed to exit on stdin close")
	}
	state := inspectField(c, name, "State.Running")

	if state != "false" {
		c.Fatal("Container must be stopped after stdin closing")
	}
	*/
}

// Test for #2267
func (s *DockerSuite) TestCliRunWriteHostsFileAndNotCommit(c *check.C) {
	// Cannot run on Windows as Windows does not support diff.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "writehosts"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hosts && cat /etc/hosts")
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/hosts should contain 'test2267'")
	}

	/* TODO
	out, _ = dockerCmd(c, "diff", name)
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
	*/
}

func eqToBaseDiff(out string, c *check.C) bool {
	pullImageIfNotExist("busybox")
	out1, _ := dockerCmd(c, "run", "-d", "busybox", "echo", "hello")
	cID := strings.TrimSpace(out1)

	baseDiff, _ := dockerCmd(c, "diff", cID)
	baseArr := strings.Split(baseDiff, "\n")
	sort.Strings(baseArr)
	outArr := strings.Split(out, "\n")
	sort.Strings(outArr)
	return sliceEq(baseArr, outArr)
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Test for #2267
func (s *DockerSuite) TestCliRunWriteHostnameFileAndNotCommit(c *check.C) {
	// Cannot run on Windows as Windows does not support diff.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "writehostname"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hostname && cat /etc/hostname")
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/hostname should contain 'test2267'")
	}

	/* TODO
	out, _ = dockerCmd(c, "diff", name)
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
	*/
}

// Test for #2267
func (s *DockerSuite) TestCliRunWriteResolvFileAndNotCommit(c *check.C) {
	// Cannot run on Windows as Windows does not support diff.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "writeresolv"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/resolv.conf && cat /etc/resolv.conf")
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/resolv.conf should contain 'test2267'")
	}

	/* TODO
	out, _ = dockerCmd(c, "diff", name)
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
	*/
}

func (s *DockerSuite) TestCliRunEntrypoint(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "entrypoint"

	// Note Windows does not have an echo.exe built in.
	var out, expected string
	if daemonPlatform == "windows" {
		out, _ = dockerCmd(c, "run", "--name", name, "--entrypoint", "cmd /s /c echo", "busybox", "foobar")
		expected = "foobar\r\n"
	} else {
		out, _ = dockerCmd(c, "run", "--name", name, "--entrypoint", "/bin/echo", "busybox", "-n", "foobar")
		expected = "foobar"
	}

	if out != expected {
		c.Fatalf("Output should be %q, actual out: %q", expected, out)
	}
}

//FIXME not sure this shoud be kept
// Ensure that CIDFile gets deleted if it's empty
// Perform this test by making `docker run` fail
func (s *DockerSuite) TestCliRunCidFileCleanupIfEmpty(c *check.C) {
	/* FIXME
	printTestCaseName(); defer printTestDuration(time.Now())
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpCidFile := path.Join(tmpDir, "cid")

	image := "busybox"
	if daemonPlatform == "windows" {
		// Windows can't support an emptyfs image. Just use the regular Windows image
		image = WindowsBaseImage
	}
	pullImageIfNotExist(image)
	out, _, err := dockerCmdWithError("run", "--cidfile", tmpCidFile, image)
	if err == nil {
		c.Fatalf("Run without command must fail. out=%s", out)
	} else if !strings.Contains(out, "No command specified") {
		c.Fatalf("Run without command failed with wrong output. out=%s\nerr=%v", out, err)
	}

	if _, err := os.Stat(tmpCidFile); err == nil {
		c.Fatalf("empty CIDFile %q should've been deleted", tmpCidFile)
	}
	*/
}

// #2098 - Docker cidFiles only contain short version of the containerId
//sudo docker run --cidfile /tmp/docker_tesc.cid ubuntu echo "test"
// TestRunCidFile tests that run --cidfile returns the longid
func (s *DockerSuite) TestCliRunCidFileCheckIDLength(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")
	defer os.RemoveAll(tmpDir)
	pullImageIfNotExist("busybox")

	out, _ := dockerCmd(c, "run", "-d", "--cidfile", tmpCidFile, "busybox", "true")

	id := strings.TrimSpace(out)
	buffer, err := ioutil.ReadFile(tmpCidFile)
	if err != nil {
		c.Fatal(err)
	}
	cid := string(buffer)
	if len(cid) != 64 {
		c.Fatalf("--cidfile should be a long id, not %q", id)
	}
	if cid != id {
		c.Fatalf("cid must be equal to %s, got %s", id, cid)
	}
}

// Regression test for #7792
func (s *DockerSuite) TestCliRunMountOrdering(c *check.C) {
	// TODO Windows: Post TP4. Updated, but Windows does not support nested mounts currently.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon, DaemonIsLinux, NotUserNamespace)

	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	tmpDir, err := ioutil.TempDir("", "docker_nested_mount_test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDir2, err := ioutil.TempDir("", "docker_nested_mount_test2")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create a temporary tmpfs mounc.
	fooDir := filepath.Join(tmpDir, "foo")
	if err := os.MkdirAll(filepath.Join(tmpDir, "foo"), 0755); err != nil {
		c.Fatalf("failed to mkdir at %s - %s", fooDir, err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", fooDir), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir2), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run",
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp", tmpDir),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/foo", fooDir),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/tmp2", tmpDir2),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/tmp2/foo", fooDir),
		"busybox:latest", "sh", "-c",
		"ls "+prefix+"/tmp/touch-me && ls "+prefix+"/tmp/foo/touch-me && ls "+prefix+"/tmp/tmp2/touch-me && ls "+prefix+"/tmp/tmp2/foo/touch-me")
}

func (s *DockerSuite) TestCliRunNoOutputFromPullInStdout(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	// just run with unknown image
	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "asdfsg")
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	if err := cmd.Run(); err == nil {
		c.Fatal("Run with unknown image should fail")
	}
	if stdout.Len() != 0 {
		c.Fatalf("Stdout contains output from pull: %s", stdout)
	}
}

// Regression test for #3631
func (s *DockerSuite) TestCliRunSlowStdoutConsumer(c *check.C) {
	/* FIXME
	// TODO Windows: This should be able to run on Windows if can find an
	// alternate to /dev/zero and /dev/stdout.
	testRequires(c, DaemonIsLinux)
	printTestCaseName(); defer printTestDuration(time.Now())
	pullImageIfNotExist("busybox")
	cont := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--rm", "busybox", "/bin/sh", "-c", "dd if=/dev/zero of=/dev/stdout bs=1024 count=2000 | catv")

	stdout, err := cont.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := cont.Start(); err != nil {
		c.Fatal(err)
	}
	n, err := consumeWithSpeed(stdout, 10000, 5*time.Millisecond, nil)
	if err != nil {
		c.Fatal(err)
	}

	expected := 2 * 1024 * 2000
	if n != expected {
		c.Fatalf("Expected %d, got %d", expected, n)
	}
	*/
}

func (s *DockerSuite) TestCliRunAllowPortRangeThroughExposeBasic(c *check.C) {
	// TODO Windows: -P is not currently supported. Also network
	// settings are not propagated back.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	_, exitCode := dockerCmd(c, "pull", rangePortImage)
	if exitCode != 0 {
		c.Fatalf("pull image %s failed", rangePortImage)
	}
	out, _ := dockerCmd(c, "run", "-d", rangePortImage, "top")

	id := strings.TrimSpace(out)
	portstr := inspectFieldJSON(c, id, "NetworkSettings.Ports")
	var ports nat.PortMap
	if err := unmarshalJSON([]byte(portstr), &ports); err != nil {
		c.Fatal(err)
	}
	for port, binding := range ports {
		portnum, _ := strconv.Atoi(strings.Split(string(port), "/")[0])
		if portnum < 80 || portnum > 90 {
			c.Fatalf("Port %d is out of range ", portnum)
		}
		if binding == nil || len(binding) != 1 || len(binding[0].HostPort) == 0 {
			c.Fatalf("Port is not mapped for the port %s", port)
		}
	}
}

func (s *DockerSuite) TestCliRunUnknownCommand(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _, _ := dockerCmdWithStdoutStderr(c, "create", "busybox", "/bin/nada")

	cID := strings.TrimSpace(out)
	_, _, err := dockerCmdWithError("start", cID)
	time.Sleep(5 * time.Second)

	// Windows and Linux are different here by architectural design. Linux will
	// fail to start the container, so an error is expected. Windows will
	// successfully start the container, and once started attempt to execute
	// the command which will fail.
	if daemonPlatform == "windows" {
		// Wait for it to exit.
		waitExited(cID, 30*time.Second)
		c.Assert(err, check.IsNil)
	} else {
		c.Assert(err, check.IsNil)
	}

	rc := inspectField(c, cID, "State.ExitCode")
	if rc == "0" {
		c.Fatalf("ExitCode(%v) cannot be 0", rc)
	}
}

func (s *DockerSuite) TestCliRunModePidHost(c *check.C) {
	// Not applicable on Windows as uses Unix-specific capabilities
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon, DaemonIsLinux, NotUserNamespace)

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	if err != nil {
		c.Fatal(err)
	}

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--pid=host", "busybox", "readlink", "/proc/self/ns/pid")
	out = strings.Trim(out, "\n")
	if hostPid != out {
		c.Fatalf("PID different with --pid=host %s != %s\n", hostPid, out)
	}

	out, _ = dockerCmd(c, "run", "busybox", "readlink", "/proc/self/ns/pid")
	out = strings.Trim(out, "\n")
	if hostPid == out {
		c.Fatalf("PID should be different without --pid=host %s == %s\n", hostPid, out)
	}
}

func (s *DockerSuite) TestCliRunTLSverify(c *check.C) {
	/* FIXME
	printTestCaseName(); defer printTestDuration(time.Now())
	if out, code, err := dockerCmdWithError("ps"); err != nil || code != 0 {
		c.Fatalf("Should have worked: %v:\n%v", err, out)
	}

	// Regardless of whether we specify true or false we need to
	// test to make sure tls is turned on if --tlsverify is specified at all
	out, code, err := dockerCmdWithError("--tlsverify=false", "ps")
	if err == nil || code == 0 || !strings.Contains(out, "trying to connect") {
		c.Fatalf("Should have failed: \net:%v\nout:%v\nerr:%v", code, out, err)
	}

	out, code, err = dockerCmdWithError("--tlsverify=true", "ps")
	if err == nil || code == 0 || !strings.Contains(out, "cert") {
		c.Fatalf("Should have failed: \net:%v\nout:%v\nerr:%v", code, out, err)
	}
	*/
}

func (s *DockerSuite) TestCliRunTTYWithPipe(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	errChan := make(chan error)
	go func() {
		defer close(errChan)

		cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-ti", "busybox", "true")
		if _, err := cmd.StdinPipe(); err != nil {
			errChan <- err
			return
		}

		expected := "cannot enable tty mode"
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			errChan <- fmt.Errorf("run should have failed")
			return
		} else if !strings.Contains(out, expected) {
			errChan <- fmt.Errorf("run failed with error %q: expected %q", out, expected)
			return
		}
	}()

	select {
	case err := <-errChan:
		c.Assert(err, check.IsNil)
	case <-time.After(6 * time.Second):
		c.Fatal("container is running but should have failed")
	}
}

func (s *DockerSuite) TestCliRunSetDefaultRestartPolicy(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "sleep", "30")
	out := inspectField(c, "test", "HostConfig.RestartPolicy.Name")
	if out != "no" {
		c.Fatalf("Set default restart policy failed")
	}
}

func (s *DockerSuite) TestCliRunRestartMaxRetries(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	RetryCount := "3"
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:"+RetryCount, "busybox", "sh", "-c", "sleep 35; false")
	timeout := 120 * time.Second

	time.Sleep(timeout)
	id := strings.TrimSpace(string(out))

	count := inspectField(c, id, "RestartCount")
	if count != RetryCount {
		c.Fatalf("Container was restarted %s times, expected %s", count, RetryCount)
	}

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")
	if MaximumRetryCount != RetryCount {
		c.Fatalf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, RetryCount)
	}
}

func (s *DockerSuite) TestCliRunContainerWithWritableRootfs(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "--rm", "busybox", "touch", "/file")
}

// run container with --rm should remove container if exit code != 0
func (s *DockerSuite) TestCliRunContainerWithRmFlagExitCodeNotEqualToZero(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "flowers"
	out, _, err := dockerCmdWithError("run", "--name", name, "--rm", "busybox", "ls", "/notexists")
	if err == nil {
		c.Fatal("Expected docker run to fail", out, err)
	}

	out, err = getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestCliRunContainerWithRmFlagCannotStartContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "sparkles"
	out, _, err := dockerCmdWithError("run", "--name", name, "--rm", "busybox", "commandNotFound")
	if err == nil {
		c.Fatal("Expected docker run to fail", out, err)
	}

	out, err = getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestCliRunWriteToProcAsound(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	_, code, err := dockerCmdWithError("run", "busybox", "sh", "-c", "echo 111 >> /proc/asound/version")
	if err == nil || code == 0 {
		c.Fatal("standard container should not be able to write to /proc/asound")
	}
}

func (s *DockerSuite) TestCliRunReadProcTimer(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, code, err := dockerCmdWithError("run", "busybox", "cat", "/proc/timer_stats")
	if code != 0 {
		return
	}
	if err != nil {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/timer_stats but received %q", out)
	}
}

func (s *DockerSuite) TestCliRunReadProcLatency(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	// some kernels don't have this configured so skip the test if this file is not found
	// on the host running the tests.
	if _, err := os.Stat("/proc/latency_stats"); err != nil {
		c.Skip("kernel doesnt have latency_stats configured")
		return
	}
	out, code, err := dockerCmdWithError("run", "busybox", "cat", "/proc/latency_stats")
	if code != 0 {
		return
	}
	if err != nil {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/latency_stats but received %q", out)
	}
}

func (s *DockerSuite) TestCliRunNetworkFilesBindMount(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	pullImageIfNotExist("busybox")

	expected := "test123"

	filename := createTmpFile(c, expected)
	defer os.Remove(filename)

	nwfiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/hostname"}

	for i := range nwfiles {
		actual, _ := dockerCmd(c, "run", "-v", filename+":"+nwfiles[i], "busybox", "cat", nwfiles[i])
		if actual != expected {
			c.Fatalf("expected %s be: %q, but was: %q", nwfiles[i], expected, actual)
		}
	}
}

func (s *DockerSuite) TestCliRunNetworkFilesBindMountRO(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	filename := createTmpFile(c, "test123")
	defer os.Remove(filename)

	nwfiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/hostname"}

	for i := range nwfiles {
		_, exitCode, err := dockerCmdWithError("run", "-v", filename+":"+nwfiles[i]+":ro", "busybox", "touch", nwfiles[i])
		if err == nil || exitCode == 0 {
			c.Fatalf("run should fail because bind mount of %s is ro: exit code %d", nwfiles[i], exitCode)
		}
	}
}

func (s *DockerTrustSuite) TestCliRunWhenCertExpired(c *check.C) {
	// Windows does not support this functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	c.Skip("Currently changes system time, causing instability")
	repoName := s.setupTrustedImage(c, "trusted-run-expired")

	// Certificates have 10 years of expiration
	elevenYearsFromNow := time.Now().Add(time.Hour * 24 * 365 * 11)

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try run
		runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", repoName)
		s.trustedCmd(runCmd)
		out, _, err := runCommandWithOutput(runCmd)
		if err == nil {
			c.Fatalf("Error running trusted run in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "could not validate the path to a trusted root") {
			c.Fatalf("Missing expected output on trusted run in the distant future:\n%s", out)
		}
	})

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try run
		runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--disable-content-trust", repoName)
		s.trustedCmd(runCmd)
		out, _, err := runCommandWithOutput(runCmd)
		if err != nil {
			c.Fatalf("Error running untrusted run in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "Status: Downloaded") {
			c.Fatalf("Missing expected output on untrusted run in the distant future:\n%s", out)
		}
	})
}

func (s *DockerSuite) TestCliRunPtraceContainerProcsFromHost(c *check.C) {
	// Not applicable on Windows as uses Unix specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	pullImageIfNotExist("busybox")

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)
	pid1 := inspectField(c, id, "State.Pid")

	_, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/net", pid1))
	if err != nil {
		c.Fatal(err)
	}
}

// run create container failed should clean up the container
func (s *DockerSuite) TestCliRunCreateContainerFailedCleanUp(c *check.C) {
	// TODO Windows. This may be possible to enable once link is supported
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	name := "unique_name"
	_, _, err := dockerCmdWithError("run", "--name", name, "--link", "nothing:nothing", "busybox")
	c.Assert(err, check.NotNil, check.Commentf("Expected docker run to fail!"))

	containerID, err := inspectFieldWithError(name, "Id")
	c.Assert(err, checker.NotNil, check.Commentf("Expected not to have this container: %s!", containerID))
	c.Assert(containerID, check.Equals, "", check.Commentf("Expected not to have this container: %s!", containerID))
}

// #11957 - stdin with no tty does not exit if stdin is not closed even though container exited
func (s *DockerSuite) TestCliRunStdinBlockedAfterContainerExit(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-i", "--name=test", "busybox", "true")
	in, err := cmd.StdinPipe()
	c.Assert(err, check.IsNil)
	defer in.Close()
	c.Assert(cmd.Start(), check.IsNil)

	waitChan := make(chan error)
	go func() {
		waitChan <- cmd.Wait()
	}()

	select {
	case err := <-waitChan:
		c.Assert(err, check.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("timeout waiting for command to exit")
	}
}

// TestRunNonExecutableCmd checks that 'docker run busybox foo' exits with error code 127'
func (s *DockerSuite) TestCliRunNonExecutableCmd(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "test-non-executable-cmd"
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--name", name, "busybox", "foo")
	_, exit, _ := runCommandWithOutput(runCmd)
	stateExitCode := findContainerExitCode(c, name)
	if !(exit == 127 && strings.Contains(stateExitCode, "127")) {
		c.Fatalf("Run non-executable command should have errored with exit code 127, but we got exit: %d, State.ExitCode: %s", exit, stateExitCode)
	}
}

// TestRunNonExistingCmd checks that 'docker run busybox /bin/foo' exits with code 127.
func (s *DockerSuite) TestCliRunNonExistingCmd(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "test-non-existing-cmd"
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--name", name, "busybox", "/bin/foo")
	_, exit, _ := runCommandWithOutput(runCmd)
	stateExitCode := findContainerExitCode(c, name)
	if !(exit == 127 && strings.Contains(stateExitCode, "127")) {
		c.Fatalf("Run non-existing command should have errored with exit code 127, but we got exit: %d, State.ExitCode: %s", exit, stateExitCode)
	}
}

// TestCmdCannotBeInvoked checks that 'docker run busybox /etc' exits with 126, or
// 127 on Windows. The difference is that in Windows, the container must be started
// as that's when the check is made (and yes, by it's design...)
func (s *DockerSuite) TestCliRunCmdCannotBeInvoked(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	expected := 126
	if daemonPlatform == "windows" {
		expected = 127
	}
	name := "test-cmd-cannot-be-invoked"
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "--name", name, "busybox", "/etc")
	_, exit, _ := runCommandWithOutput(runCmd)
	stateExitCode := findContainerExitCode(c, name)
	if !(exit == expected && strings.Contains(stateExitCode, strconv.Itoa(expected))) {
		c.Fatalf("Run cmd that cannot be invoked should have errored with code %d, but we got exit: %d, State.ExitCode: %s", expected, exit, stateExitCode)
	}
}

// TestRunNonExistingImage checks that 'docker run foo' exits with error msg 125 and contains  'Unable to find image'
func (s *DockerSuite) TestCliRunNonExistingImage(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "foo")
	out, exit, err := runCommandWithOutput(runCmd)
	if !(err != nil && exit == 125 && strings.Contains(out, "Unable to find image")) {
		c.Fatalf("Run non-existing image should have errored with 'Unable to find image' code 125, but we got out: %s, exit: %d, err: %s", out, exit, err)
	}
}

// TestDockerFails checks that 'docker run -foo busybox' exits with 125 to signal docker run failed
func (s *DockerSuite) TestCliRunDockerFails(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	runCmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "run", "-foo", "busybox")
	out, exit, err := runCommandWithOutput(runCmd)
	if !(err != nil && exit == 125) {
		c.Fatalf("Docker run with flag not defined should exit with 125, but we got out: %s, exit: %d, err: %s", out, exit, err)
	}
}

// TestRunInvalidReference invokes docker run with a bad reference.
func (s *DockerSuite) TestCliRunInvalidReference(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, exit, _ := dockerCmdWithError("run", "busybox@foo")
	if exit == 0 {
		c.Fatalf("expected non-zero exist code; received %d", exit)
	}

	if !strings.Contains(out, "Error parsing reference") {
		c.Fatalf(`Expected "Error parsing reference" in output; got: %s`, out)
	}
}

func (s *DockerSuite) TestCliRunVolumesMountedAsSlave(c *check.C) {
	// Volume propagation is linux only. Also it creates directories for
	// bind mounting, so needs to be same host.
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, SameHostDaemon, NotUserNamespace)

	// Prepare a source directory to bind mount
	tmpDir, err := ioutil.TempDir("", "volume-source")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.Mkdir(path.Join(tmpDir, "mnt1"), 0755); err != nil {
		c.Fatal(err)
	}

	// Prepare a source directory with file in it. We will bind mount this
	// direcotry and see if file shows up.
	tmpDir2, err := ioutil.TempDir("", "volume-source2")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir2)

	if err := ioutil.WriteFile(path.Join(tmpDir2, "slave-testfile"), []byte("Test"), 0644); err != nil {
		c.Fatal(err)
	}

	// Convert this directory into a shared mount point so that we do
	// not rely on propagation properties of parent mount.
	cmd := exec.Command("mount", "--bind", tmpDir, tmpDir)
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command("mount", "--make-private", "--make-shared", tmpDir)
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run", "-i", "-d", "--name", "parent", "-v", fmt.Sprintf("%s:/volume-dest:slave", tmpDir), "busybox", "top")

	// Bind mount tmpDir2/ onto tmpDir/mnt1. If mount propagates inside
	// container then contents of tmpDir2/slave-testfile should become
	// visible at "/volume-dest/mnt1/slave-testfile"
	cmd = exec.Command("mount", "--bind", tmpDir2, path.Join(tmpDir, "mnt1"))
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "exec", "parent", "cat", "/volume-dest/mnt1/slave-testfile")

	mount.Unmount(path.Join(tmpDir, "mnt1"))

	if out != "Test" {
		c.Fatalf("Bind mount under slave volume did not propagate to container")
	}
}

func (s *DockerSuite) TestCliRunNamedVolumesMountedAsShared(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	pullImageIfNotExist("busybox")
	out, exitcode, _ := dockerCmdWithError("run", "-v", "foo:/test:shared", "busybox", "touch", "/test/somefile")

	if exitcode == 0 {
		c.Fatalf("expected non-zero exit code; received %d", exitcode)
	}

	if expected := "Invalid volume specification"; !strings.Contains(out, expected) {
		c.Fatalf(`Expected %q in output; got: %s`, expected, out)
	}
}

func (s *DockerSuite) TestCliRunNamedVolumeNotRemoved(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	dockerCmd(c, "volume", "create", "--name", "test")

	dockerCmdWithError("run", "--rm", "-v", "test:"+prefix+"/foo", "-v", prefix+"/bar", "busybox", "true")
	dockerCmd(c, "volume", "inspect", "test")
	out, _ := dockerCmd(c, "volume", "ls", "-q")
	c.Assert(strings.TrimSpace(out), checker.Equals, "test")

	dockerCmdWithError("run", "--name=test", "-v", "test:"+prefix+"/foo", "-v", prefix+"/bar", "busybox", "true")
	dockerCmdWithError("rm", "-fv", "test")
	dockerCmd(c, "volume", "inspect", "test")
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	c.Assert(strings.TrimSpace(out), checker.Equals, "test")
}

func (s *DockerSuite) TestRunProtectionContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "--protection=true", "-d", "busybox", "top")
	id := strings.TrimSpace(out)

	out = inspectField(c, id, "Config.Labels")
	c.Assert(out, checker.Contains, "sh_hyper_container_protection")

	_, exitcode, _ := dockerCmdWithError("rm", "-f", id)
	if exitcode == 0 {
		c.Fatalf("expected non-zero exit code; received %d", exitcode)
	}

	dockerCmd(c, "update", "--protection=false", id)

	out = inspectField(c, id, "Config.Labels")
	c.Assert(out, checker.Not(checker.Contains), "sh_hyper_container_protection")

	dockerCmd(c, "rm", "-f", id)

	out, err := getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestCreateProtectionContainer(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "--protection=true", "busybox", "top")
	id := strings.TrimSpace(out)

	out = inspectField(c, id, "Config.Labels")
	c.Assert(out, checker.Contains, "sh_hyper_container_protection")

	_, exitcode, _ := dockerCmdWithError("rm", id)
	if exitcode == 0 {
		c.Fatalf("expected non-zero exit code; received %d", exitcode)
	}

	dockerCmd(c, "update", "--protection=false", id)

	out = inspectField(c, id, "Config.Labels")
	c.Assert(out, checker.Not(checker.Contains), "sh_hyper_container_protection")

	dockerCmd(c, "rm", "-f", id)

	out, err := getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}
