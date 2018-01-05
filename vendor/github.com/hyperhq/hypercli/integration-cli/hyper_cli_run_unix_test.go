// +build !windows

package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// #6509
func (s *DockerSuite) TestCliRunRedirectStdout(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	checkRedirect := func(command string) {
		_, tty, err := pty.Open()
		c.Assert(err, checker.IsNil, check.Commentf("Could not open pty"))
		cmd := exec.Command("sh", "-c", command)
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		c.Assert(cmd.Start(), checker.IsNil)
		ch := make(chan error)
		go func() {
			ch <- cmd.Wait()
			close(ch)
		}()

		select {
		case <-time.After(30 * time.Second):
			c.Fatal("command timeout")
		case err := <-ch:
			c.Assert(err, checker.IsNil, check.Commentf("wait err"))
		}
	}

	checkRedirect(dockerBinary + " --region " + os.Getenv("DOCKER_HOST") + " run -it busybox cat /etc/passwd | grep -q root")
	checkRedirect(dockerBinary + " --region " + os.Getenv("DOCKER_HOST") + " run busybox cat /etc/passwd | grep -q root")
}

func (s *DockerSuite) TestCliRunAttachDetachBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	name := "attach-detach"
	dockerCmd(c, "run", "--name", name, "-itd", "busybox", "cat")

	cmd := exec.Command(dockerBinary, "--region", os.Getenv("DOCKER_HOST"), "attach", name)
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	cpty, tty, err := pty.Open()
	c.Assert(err, checker.IsNil)
	defer cpty.Close()
	cmd.Stdin = tty
	c.Assert(cmd.Start(), checker.IsNil)
	c.Assert(waitRun(name), check.IsNil)

	_, err = cpty.Write([]byte("hello\n"))
	c.Assert(err, checker.IsNil)

	out, err := bufio.NewReader(stdout).ReadString('\n')
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello")

	// escape sequence
	_, err = cpty.Write([]byte{16})
	c.Assert(err, checker.IsNil)
	time.Sleep(100 * time.Millisecond)
	_, err = cpty.Write([]byte{17})
	c.Assert(err, checker.IsNil)

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	select {
	case <-ch:
	case <-time.After(30 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running := inspectField(c, name, "State.Running")
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))
}

/*
// Hyper does not support shm yet
func (s *DockerSuite) TestCliRunWithDefaultShmSize(c *check.C) {
	testRequires(c, DaemonIsLinux)

	name := "shm-default"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "mount")
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
	shmSize := inspectField(c, name, "HostConfig.ShmSize")
	c.Assert(shmSize, check.Equals, "67108864")
}
*/
