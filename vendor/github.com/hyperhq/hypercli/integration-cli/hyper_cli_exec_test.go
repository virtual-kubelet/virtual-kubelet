// +build !test_no_exec

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliExecBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "sh", "-c", "echo test > /tmp/file && top")

	out, _ := dockerCmd(c, "exec", "test", "cat", "/tmp/file")
	out = strings.Trim(out, "\r\n")
	c.Assert(out, checker.Equals, "test")

}

//TODO:FIX ExecInteractive WAITING TOO LONG
/*func (s *DockerSuite) TestCliExecInteractive(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "sh", "-c", "echo test > /tmp/file && top")

	execCmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", "-i", "test", "sh")
	stdin, err := execCmd.StdinPipe()
	c.Assert(err, checker.IsNil)
	stdout, err := execCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)

	err = execCmd.Start()
	c.Assert(err, checker.IsNil)
	_, err = stdin.Write([]byte("cat /tmp/file\n"))
	c.Assert(err, checker.IsNil)

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	c.Assert(err, checker.IsNil)
	line = strings.TrimSpace(line)
	c.Assert(line, checker.Equals, "test")
	err = stdin.Close()
	c.Assert(err, checker.IsNil)
	errChan := make(chan error)
	go func() {
		errChan <- execCmd.Wait()
		close(errChan)
	}()
	select {
	case err := <-errChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(10 * time.Second):
		c.Fatal("docker exec failed to exit on stdin close")
	}
}*/

func (s *DockerSuite) TestCliExecAfterContainerRestart(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	out, _ := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)
	c.Assert(waitRun(cleanedContainerID), check.IsNil)
	dockerCmd(c, "restart", cleanedContainerID)
	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	out, _ = dockerCmd(c, "exec", cleanedContainerID, "echo", "hello")
	outStr := strings.TrimSpace(out)
	c.Assert(outStr, checker.Equals, "hello")
}

//TODO:FIX TestExecEnv WAITING TOO LONG
/*func (s *DockerSuite) TestCliExecEnv(c *check.C) {
	// TODO Windows CI: This one is interesting and may just end up being a feature
	// difference between Windows and Linux. On Windows, the environment is passed
	// into the process that is launched, not into the machine environment. Hence
	// a subsequent exec will not have LALA set/
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	runSleepingContainer(c, "-e", "LALA=value1", "-e", "LALA=value2", "-d", "--name", "test")
	c.Assert(waitRun("test"), check.IsNil)

	out, _ := dockerCmd(c, "exec", "test", "env")
	c.Assert(out, checker.Not(checker.Contains), "LALA=value1")
	c.Assert(out, checker.Contains, "LALA=value2")
	c.Assert(out, checker.Contains, "HOME=/root")
}*/

func (s *DockerSuite) TestCliExecExitStatus(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	runSleepingContainer(c, "-d", "--name", "top")

	// Test normal (non-detached) case first
	cmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", "top", "sh", "-c", "exit 23")
	ec, _ := runCommand(cmd)
	c.Assert(ec, checker.Equals, 23)
}

//TODO:FIX TestExecTTYCloseStdin WAITING TOO LONG SAME AS TestExecInteractive
/*func (s *DockerSuite) TestCliExecTTYCloseStdin(c *check.C) {
	// TODO Windows CI: This requires some work to port to Windows.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "-it", "--name", "test", "busybox")

	cmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", "-i", "test", "cat")
	stdinRw, err := cmd.StdinPipe()
	c.Assert(err, checker.IsNil)

	stdinRw.Write([]byte("test"))
	stdinRw.Close()

	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, _ = dockerCmd(c, "top", "test")
	outArr := strings.Split(out, "\n")
	c.Assert(len(outArr), checker.LessOrEqualThan, 3, check.Commentf("exec process left running"))
	c.Assert(out, checker.Not(checker.Contains), "nsenter-exec")
}*/

func (s *DockerSuite) TestCliExecTTYWithoutStdin(c *check.C) {
	// TODO Windows CI: This requires some work to port to Windows.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "-ti", "busybox")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	errChan := make(chan error)
	go func() {
		defer close(errChan)

		cmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", "-ti", id, "true")
		if _, err := cmd.StdinPipe(); err != nil {
			errChan <- err
			return
		}

		expected := "cannot enable tty mode"
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			errChan <- fmt.Errorf("exec should have failed")
			return
		} else if !strings.Contains(out, expected) {
			errChan <- fmt.Errorf("exec failed with error %q: expected %q", out, expected)
			return
		}
	}()

	select {
	case err := <-errChan:
		c.Assert(err, check.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("exec is running but should have failed")
	}
}

func (s *DockerSuite) TestCliExecParseError(c *check.C) {
	// TODO Windows CI: Requires some extra work. Consider copying the
	// runSleepingContainer helper to have an exec version.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "top", "busybox", "top")

	// Test normal (non-detached) case first
	cmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", "top")
	_, stderr, _, err := runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(stderr, checker.Contains, "See '"+dockerBinary+" exec --help'")
}

func (s *DockerSuite) TestCliExecStopNotHanging(c *check.C) {
	// TODO Windows CI: Requires some extra work. Consider copying the
	// runSleepingContainer helper to have an exec version.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	err := exec.Command(dockerBinary, "exec", "--region="+os.Getenv("DOCKER_HOST"), "test", "top").Start()
	c.Assert(err, checker.IsNil)

	type dstop struct {
		out []byte
		err error
	}

	ch := make(chan dstop)
	go func() {
		out, err := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "stop", "test").CombinedOutput()
		ch <- dstop{out, err}
		close(ch)
	}()
	select {
	case <-time.After(30 * time.Second):
		c.Fatal("Container stop timed out")
	case s := <-ch:
		c.Assert(s.err, check.IsNil)
	}
}

func (s *DockerSuite) TestCliExecCgroup(c *check.C) {
	// Not applicable on Windows - using Linux specific functionality
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")

	out, _ := dockerCmd(c, "exec", "test", "cat", "/proc/1/cgroup")
	containerCgroups := sort.StringSlice(strings.Split(out, "\n"))

	var wg sync.WaitGroup
	var mu sync.Mutex
	execCgroups := []sort.StringSlice{}
	errChan := make(chan error)
	// exec a few times concurrently to get consistent failure
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			out, _, err := dockerCmdWithError("exec", "test", "cat", "/proc/self/cgroup")
			if err != nil {
				errChan <- err
				return
			}
			cg := sort.StringSlice(strings.Split(out, "\n"))

			mu.Lock()
			execCgroups = append(execCgroups, cg)
			mu.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		c.Assert(err, checker.IsNil)
	}

	for _, cg := range execCgroups {
		if !reflect.DeepEqual(cg, containerCgroups) {
			fmt.Println("exec cgroups:")
			for _, name := range cg {
				fmt.Printf(" %s\n", name)
			}

			fmt.Println("container cgroups:")
			for _, name := range containerCgroups {
				fmt.Printf(" %s\n", name)
			}
			c.Fatal("cgroups mismatched")
		}
	}
}

func (s *DockerSuite) TestCliExecLinksPingLinkedContainersOnRename(c *check.C) {
	// Problematic on Windows as Windows does not support links
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	var out string
	out, _ = dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	idA := strings.TrimSpace(out)
	c.Assert(idA, checker.Not(checker.Equals), "", check.Commentf("%s, id should not be nil", out))
	out, _ = dockerCmd(c, "run", "-d", "--link", "container1:alias1", "--name", "container2", "busybox", "top")
	idB := strings.TrimSpace(out)
	c.Assert(idB, checker.Not(checker.Equals), "", check.Commentf("%s, id should not be nil", out))

	dockerCmd(c, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
	dockerCmd(c, "rename", "container1", "container-new")
	dockerCmd(c, "exec", "container2", "ping", "-c", "1", "alias1", "-W", "1")
}

func (s *DockerSuite) TestCliExecDir(c *check.C) {
	// TODO Windows CI. This requires some work to port as it uses execDriverPath
	// which is currently (and incorrectly) hard coded as a string assuming
	// the daemon is running Linux :(
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, SameHostDaemon, DaemonIsLinux)

	out, _ := runSleepingContainer(c, "-d")
	id := strings.TrimSpace(out)

	execDir := filepath.Join(execDriverPath, id)
	stateFile := filepath.Join(execDir, "state.json")

	{
		fi, err := os.Stat(execDir)
		c.Assert(err, checker.IsNil)
		if !fi.IsDir() {
			c.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		c.Assert(err, checker.IsNil)
	}

	dockerCmd(c, "stop", id)
	{
		_, err := os.Stat(execDir)
		c.Assert(err, checker.NotNil)
		c.Assert(err, checker.NotNil, check.Commentf("Exec directory %q exists for removed container!", execDir))
		if !os.IsNotExist(err) {
			c.Fatalf("Error should be about non-existing, got %s", err)
		}
	}
	dockerCmd(c, "start", id)
	{
		fi, err := os.Stat(execDir)
		c.Assert(err, checker.IsNil)
		if !fi.IsDir() {
			c.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		c.Assert(err, checker.IsNil)
	}
	dockerCmd(c, "rm", "-f", id)
	{
		_, err := os.Stat(execDir)
		c.Assert(err, checker.NotNil, check.Commentf("Exec directory %q exists for removed container!", execDir))
		if !os.IsNotExist(err) {
			c.Fatalf("Error should be about non-existing, got %s", err)
		}
	}
}

func (s *DockerSuite) TestCliExecRunMutableNetworkFiles(c *check.C) {
	// Not applicable on Windows to Windows CI.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, SameHostDaemon, DaemonIsLinux)
	pullImageIfNotExist("busybox")
	for _, fn := range []string{"resolv.conf", "hosts"} {
		deleteAllContainers()

		content, err := runCommandAndReadContainerFile(fn, exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "run", "-d", "--name", "c1", "busybox", "sh", "-c", fmt.Sprintf("echo success >/etc/%s && top", fn)))
		c.Assert(err, checker.IsNil)

		c.Assert(strings.TrimSpace(string(content)), checker.Equals, "success", check.Commentf("Content was not what was modified in the container", string(content)))

		out, _ := dockerCmd(c, "run", "-d", "--name", "c2", "busybox", "top")
		contID := strings.TrimSpace(out)
		netFilePath := containerStorageFile(contID, fn)

		f, err := os.OpenFile(netFilePath, os.O_WRONLY|os.O_SYNC|os.O_APPEND, 0644)
		c.Assert(err, checker.IsNil)

		if _, err := f.Seek(0, 0); err != nil {
			f.Close()
			c.Fatal(err)
		}

		if err := f.Truncate(0); err != nil {
			f.Close()
			c.Fatal(err)
		}

		if _, err := f.Write([]byte("success2\n")); err != nil {
			f.Close()
			c.Fatal(err)
		}
		f.Close()

		res, _ := dockerCmd(c, "exec", contID, "cat", "/etc/"+fn)
		c.Assert(res, checker.Equals, "success2\n")
	}
}

func (s *DockerSuite) TestCliExecStartFails(c *check.C) {
	// TODO Windows CI. This test should be portable. Figure out why it fails
	// currently.
	printTestCaseName()
	defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	name := "exec-15750"
	runSleepingContainer(c, "-d", "--name", name)
	c.Assert(waitRun(name), checker.IsNil)

	out, _, err := dockerCmdWithError("exec", name, "no-such-cmd")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "exec failed: No such file or directory")
}

func (s *DockerSuite) TestCliExecInspectID(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	pullImageIfNotExist("busybox")
	out, _ := runSleepingContainer(c, "-d")
	id := strings.TrimSuffix(out, "\n")

	out = inspectField(c, id, "ExecIDs")
	c.Assert(out, checker.Equals, "[]", check.Commentf("ExecIDs should be empty, got: %s", out))

	// Start an exec, have it block waiting so we can do some checking
	cmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", id, "sh", "-c",
		"while ! test -e /execid1; do sleep 1; done")

	err := cmd.Start()
	c.Assert(err, checker.IsNil, check.Commentf("failed to start the exec cmd"))

	// Give the exec 10 chances/seconds to start then give up and stop the test
	tries := 10
	for i := 0; i < tries; i++ {
		// Since its still running we should see exec as part of the container
		out = strings.TrimSpace(inspectField(c, id, "ExecIDs"))

		if out != "[]" && out != "<no value>" {
			break
		}
		c.Assert(i+1, checker.Not(checker.Equals), tries, check.Commentf("ExecIDs still empty after 10 second"))
		time.Sleep(1 * time.Second)
	}

	// Save execID for later
	execID, err := inspectFilter(id, "index .ExecIDs 0")
	c.Assert(err, checker.IsNil, check.Commentf("failed to get the exec id"))

	// End the exec by creating the missing file
	err = exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "exec", id,
		"sh", "-c", "touch /execid1").Run()

	c.Assert(err, checker.IsNil, check.Commentf("failed to run the 2nd exec cmd"))

	// Wait for 1st exec to complete
	cmd.Wait()

	// Give the exec 10 chances/seconds to stop then give up and stop the test
	for i := 0; i < tries; i++ {
		// Since its still running we should see exec as part of the container
		out = strings.TrimSpace(inspectField(c, id, "ExecIDs"))

		if out == "[]" {
			break
		}
		c.Assert(i+1, checker.Not(checker.Equals), tries, check.Commentf("ExecIDs still not empty after 10 second"))
		time.Sleep(1 * time.Second)
	}

	// But we should still be able to query the execID
	sc, body, err := sockRequest("GET", "/exec/"+execID+"/json", nil)
	c.Assert(sc, checker.Equals, http.StatusOK, check.Commentf("received status != 200 OK: %d\n%s", sc, body))

	//TODO: fix receive 500
	// Now delete the container and then an 'inspect' on the exec should
	// result in a 404 (not 'container not running')
	/*out, ec := dockerCmd(c, "rm", "-f", id)
	c.Assert(ec, checker.Equals, 0, check.Commentf("error removing container: %s", out))
	sc, body, err = sockRequest("GET", "/exec/"+execID+"/json", nil)
	c.Assert(sc, checker.Equals, http.StatusNotFound, check.Commentf("received status != 404: %d\n%s", sc, body))*/
}
