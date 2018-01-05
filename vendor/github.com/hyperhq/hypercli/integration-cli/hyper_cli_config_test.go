package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/hyperhq/hypercli/cliconfig"
	"github.com/hyperhq/hypercli/pkg/homedir"
	"os/exec"
)

func (s *DockerSuite) TestCliConfigAndRewrite(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	cmd := exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", "xx", "--secretkey", "xxxx", "tcp://127.0.0.1:6443")
	out, _, _, err := runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	configDir := filepath.Join(homedir.Get(), ".hyper")
	conf, err := cliconfig.Load(configDir)
	c.Assert(err, checker.IsNil)
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey, checker.Equals, "xx", check.Commentf("Should get xx, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey))
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey, checker.Equals, "xxxx", check.Commentf("Should get xxxx, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey))

	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", "yy", "--secretkey", "yyyy", "tcp://127.0.0.1:6443")
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	conf, err = cliconfig.Load(configDir)
	c.Assert(err, checker.IsNil)
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey, checker.Equals, "yy", check.Commentf("Should get yy, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey))
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey, checker.Equals, "yyyy", check.Commentf("Should get yyyy, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey))

	//patch
	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", os.Getenv("ACCESS_KEY"), "--secretkey", os.Getenv("SECRET_KEY"), os.Getenv("DOCKER_HOST"))
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", os.Getenv("ACCESS_KEY"), "--secretkey", os.Getenv("SECRET_KEY"))
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")
}

func (s *DockerSuite) TestCliConfigMultiHostBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())

	cmd := exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", "xx", "--secretkey", "xxxx", "tcp://127.0.0.1:6443")
	out, _, _, err := runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	configDir := filepath.Join(homedir.Get(), ".hyper")
	conf, err := cliconfig.Load(configDir)
	c.Assert(err, checker.IsNil)
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey, checker.Equals, "xx", check.Commentf("Should get xx, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].AccessKey))
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey, checker.Equals, "xxxx", check.Commentf("Should get xxxx, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6443"].SecretKey))

	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", "yy", "--secretkey", "yyyy", "tcp://127.0.0.1:6444")
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	conf, err = cliconfig.Load(configDir)
	c.Assert(err, checker.IsNil)
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6444"].AccessKey, checker.Equals, "yy", check.Commentf("Should get yy, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6444"].AccessKey))
	c.Assert(conf.CloudConfig["tcp://127.0.0.1:6444"].SecretKey, checker.Equals, "yyyy", check.Commentf("Should get yyyy, but get %s\n", conf.CloudConfig["tcp://127.0.0.1:6444"].SecretKey))

	//patch
	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", os.Getenv("ACCESS_KEY"), "--secretkey", os.Getenv("SECRET_KEY"), os.Getenv("DOCKER_HOST"))
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")

	cmd = exec.Command(dockerBinary, "config", "--default-region", os.Getenv("REGION"), "--accesskey", os.Getenv("ACCESS_KEY"), "--secretkey", os.Getenv("SECRET_KEY"))
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "WARNING: Your login credentials has been saved in "+homedir.Get()+"/.hyper/config.json")
}
