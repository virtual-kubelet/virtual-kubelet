package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/hyperhq/hyper-api/types"
)

func (s *DockerSuite) TestContainerApiGetAll(c *check.C) {
	startCount, err := getContainerCount()
	c.Assert(err, checker.IsNil, check.Commentf("Cannot query container count"))

	name := "getall"
	dockerCmd(c, "run", "--name", name, "busybox", "true")

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var inspectJSON []struct {
		Names []string
	}
	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil, check.Commentf("unable to unmarshal response body"))

	c.Assert(inspectJSON, checker.HasLen, startCount+1)

	actual := inspectJSON[0].Names[0]
	c.Assert(actual, checker.Equals, "/"+name)
}

// regression test for empty json field being omitted #13691
func (s *DockerSuite) TestContainerApiGetJSONNoFieldsOmitted(c *check.C) {
	dockerCmd(c, "run", "busybox", "true")

	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	// empty Labels field triggered this bug, make sense to check for everything
	// cause even Ports for instance can trigger this bug
	// better safe than sorry..
	fields := []string{
		"Id",
		"Names",
		"Image",
		"Command",
		"Created",
		"Ports",
		"Labels",
		"Status",
		"NetworkSettings",
	}

	// decoding into types.Container do not work since it eventually unmarshal
	// and empty field to an empty go map, so we just check for a string
	for _, f := range fields {
		if !strings.Contains(string(body), f) {
			c.Fatalf("Field %s is missing and it shouldn't", f)
		}
	}
}

type containerPs struct {
	Names []string
	Ports []map[string]interface{}
}

// regression test for non-empty fields from #13901
func (s *DockerSuite) TestContainerApiPsOmitFields(c *check.C) {
	// Problematic for Windows porting due to networking not yet being passed back
	testRequires(c, DaemonIsLinux)
	name := "pstest"
	port := 80

	_, code := dockerCmd(c, "pull", singlePortImage)
	c.Assert(code, check.Equals, 0)
	runSleepingContainerInImage(c, singlePortImage, "--name", name)

	debugEndpoint = "/containers/json?all=1"
	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var resp []containerPs
	err = json.Unmarshal(body, &resp)
	c.Assert(err, checker.IsNil)

	var foundContainer *containerPs
	for _, container := range resp {
		for _, testName := range container.Names {
			if "/"+name == testName {
				foundContainer = &container
				break
			}
		}
	}

	c.Assert(foundContainer.Ports, checker.HasLen, 1)
	c.Assert(foundContainer.Ports[0]["PrivatePort"], checker.Equals, float64(port))
	_, ok := foundContainer.Ports[0]["PublicPort"]
	c.Assert(ok, checker.Equals, true)
	_, ok = foundContainer.Ports[0]["IP"]
	c.Assert(ok, checker.Equals, true)
}

func (s *DockerSuite) TestContainerApiStartVolumeBinds(c *check.C) {
	// TODO Windows CI: Investigate further why this fails on Windows to Windows CI.
	testRequires(c, DaemonIsLinux)
	path := "/foo"
	if daemonPlatform == "windows" {
		path = `c:\foo`
	}
	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{path: {}},
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	bindPath := randomTmpDirPath("test", daemonPlatform)
	config = map[string]interface{}{
		"Binds": []string{bindPath + ":" + path},
	}
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	pth, err := inspectMountSourceField(name, path)
	c.Assert(err, checker.IsNil)
	c.Assert(pth, checker.Equals, bindPath, check.Commentf("expected volume host path to be %s, got %s", bindPath, pth))
}

/*
//FIXME panic
func (s *DockerSuite) TestGetContainerStats(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	var (
		name = "statscontainer"
	)
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		bc <- b{status, body, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(15 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(20 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		c.Assert(sr.err, checker.IsNil)
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		dec := json.NewDecoder(bytes.NewBuffer(sr.body))
		var s *types.Stats
		// decode only one object from the stream
		c.Assert(dec.Decode(&s), checker.IsNil)
	}
}

func (s *DockerSuite) TestGetContainerStatsRmRunning(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)

	buf := &integration.ChannelBuffer{make(chan []byte, 1)}
	defer buf.Close()
	chErr := make(chan error, 1)
	go func() {
		_, body, err := sockRequestRaw("GET", "/containers/"+id+"/stats?stream=1", nil, "application/json")
		if err != nil {
			chErr <- err
		}
		defer body.Close()
		_, err = io.Copy(buf, body)
		chErr <- err
	}()
	defer func() {
		select {
		case err := <-chErr:
			c.Assert(err, checker.IsNil)
		default:
			return
		}
	}()

	b := make([]byte, 32)
	// make sure we've got some stats
	_, err := buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	// Now remove without `-f` and make sure we are still pulling stats
	_, _, err = dockerCmdWithError("rm", id)
	c.Assert(err, checker.Not(checker.IsNil), check.Commentf("rm should have failed but didn't"))
	_, err = buf.ReadTimeout(b, 2*time.Second)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "kill", id)
}

// regression test for gh13421
// previous test was just checking one stat entry so it didn't fail (stats with
// stream false always return one stat)
func (s *DockerSuite) TestGetContainerStatsStream(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	name := "statscontainer"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		bc <- b{status, body, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		c.Assert(sr.err, checker.IsNil)
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		s := string(sr.body)
		// count occurrences of "read" of types.Stats
		if l := strings.Count(s, "read"); l < 2 {
			c.Fatalf("Expected more than one stat streamed, got %d", l)
		}
	}
}

func (s *DockerSuite) TestGetContainerStatsNoStream(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	name := "statscontainer"
	dockerCmd(c, "run", "-d", "--name", name, "busybox", "top")

	type b struct {
		status int
		body   []byte
		err    error
	}
	bc := make(chan b, 1)
	go func() {
		status, body, err := sockRequest("GET", "/containers/"+name+"/stats?stream=0", nil)
		bc <- b{status, body, err}
	}()

	// allow some time to stream the stats from the container
	time.Sleep(4 * time.Second)
	dockerCmd(c, "rm", "-f", name)

	// collect the results from the stats stream or timeout and fail
	// if the stream was not disconnected.
	select {
	case <-time.After(2 * time.Second):
		c.Fatal("stream was not closed after container was removed")
	case sr := <-bc:
		c.Assert(sr.err, checker.IsNil)
		c.Assert(sr.status, checker.Equals, http.StatusOK)

		s := string(sr.body)
		// count occurrences of "read" of types.Stats
		c.Assert(strings.Count(s, "read"), checker.Equals, 1, check.Commentf("Expected only one stat streamed, got %d", strings.Count(s, "read")))
	}
}
*/

func (s *DockerSuite) TestGetStoppedContainerStats(c *check.C) {
	// Problematic on Windows as Windows does not support stats
	testRequires(c, DaemonIsLinux)
	// TODO: this test does nothing because we are c.Assert'ing in goroutine
	var (
		name = "statscontainer"
	)
	dockerCmd(c, "create", "--name", name, "busybox", "top")

	go func() {
		// We'll never get return for GET stats from sockRequest as of now,
		// just send request and see if panic or error would happen on daemon side.
		status, _, err := sockRequest("GET", "/containers/"+name+"/stats", nil)
		c.Assert(err, checker.IsNil)
		c.Assert(status, checker.Equals, http.StatusOK)
	}()

	// allow some time to send request and let daemon deal with it
	time.Sleep(1 * time.Second)
}

// #9981 - Allow a docker created volume (ie, one in /var/lib/docker/volumes) to be used to overwrite (via passing in Binds on api start) an existing volume
func (s *DockerSuite) TestPostContainerBindNormalVolume(c *check.C) {
	// TODO Windows to Windows CI - Port this
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "create", "-v", "/foo", "--name=one", "busybox")

	fooDir, err := inspectMountSourceField("one", "/foo")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "create", "-v", "/foo", "--name=two", "busybox")

	bindSpec := map[string][]string{"Binds": {fooDir + ":/foo"}}
	status, _, err := sockRequest("POST", "/containers/two/start", bindSpec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	fooDir2, err := inspectMountSourceField("two", "/foo")
	c.Assert(err, checker.IsNil)
	c.Assert(fooDir2, checker.Equals, fooDir, check.Commentf("expected volume path to be %s, got: %s", fooDir, fooDir2))
}

func (s *DockerSuite) TestContainerApiCreate(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", "touch /test && ls /test"},
	}

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	type createResp struct {
		ID string
	}
	var container createResp
	c.Assert(json.Unmarshal(b, &container), checker.IsNil)

	out, _ := dockerCmd(c, "start", "-a", container.ID)
	c.Assert(strings.TrimSpace(out), checker.Equals, "/test")
}

func (s *DockerSuite) TestContainerApiCreateEmptyConfig(c *check.C) {
	config := map[string]interface{}{}

	status, b, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusInternalServerError)

	expected := "Config cannot be empty in order to create a container\n"
	c.Assert(string(b), checker.Equals, expected)
}

func (s *DockerSuite) TestContainerApiCreateWithHostName(c *check.C) {
	// TODO Windows: Port this test once hostname is supported
	testRequires(c, DaemonIsLinux)
	hostName := "test-host"
	config := map[string]interface{}{
		"Image":    "busybox",
		"Hostname": hostName,
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)
	c.Assert(containerJSON.Config.Hostname, checker.Equals, hostName, check.Commentf("Mismatched Hostname"))
}

func (s *DockerSuite) TestContainerApiCreateWithDomainName(c *check.C) {
	// TODO Windows: Port this test once domain name is supported
	testRequires(c, DaemonIsLinux)
	domainName := "test-domain"
	config := map[string]interface{}{
		"Image":      "busybox",
		"Domainname": domainName,
	}

	status, body, err := sockRequest("POST", "/containers/create", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	var container types.ContainerCreateResponse
	c.Assert(json.Unmarshal(body, &container), checker.IsNil)

	status, body, err = sockRequest("GET", "/containers/"+container.ID+"/json", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var containerJSON types.ContainerJSON
	c.Assert(json.Unmarshal(body, &containerJSON), checker.IsNil)
	c.Assert(containerJSON.Config.Domainname, checker.Equals, domainName, check.Commentf("Mismatched Domainname"))
}

func (s *DockerSuite) TestContainerApiVerifyHeader(c *check.C) {
	config := map[string]interface{}{
		"Image": "busybox",
	}

	create := func(ct string) (*http.Response, io.ReadCloser, error) {
		jsonData := bytes.NewBuffer(nil)
		c.Assert(json.NewEncoder(jsonData).Encode(config), checker.IsNil)
		return sockRequestRaw("POST", "/containers/create", jsonData, ct)
	}

	// Try with no content-type
	res, body, err := create("")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)
	body.Close()

	// Try with wrong content-type
	res, body, err = create("application/xml")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
	body.Close()

	// now application/json
	res, body, err = create("application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)
	body.Close()
}

// Issue 7941 - test to make sure a "null" in JSON is just ignored.
// W/o this fix a null in JSON would be parsed into a string var as "null"
func (s *DockerSuite) TestContainerApiPostCreateNull(c *check.C) {
	// TODO Windows to Windows CI. Bit of this with alternate fields checked
	// can probably be ported.
	testRequires(c, DaemonIsLinux)
	config := `{
		"Hostname":"",
		"Domainname":"",
		"Memory":0,
		"MemorySwap":0,
		"CpuShares":0,
		"Cpuset":null,
		"AttachStdin":true,
		"AttachStdout":true,
		"AttachStderr":true,
		"ExposedPorts":{},
		"Tty":true,
		"OpenStdin":true,
		"StdinOnce":true,
		"Env":[],
		"Cmd":"ls",
		"Image":"busybox",
		"Volumes":{},
		"WorkingDir":"",
		"Entrypoint":null,
		"NetworkDisabled":false,
		"OnBuild":null}`

	res, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusCreated)

	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	type createResp struct {
		ID string
	}
	var container createResp
	c.Assert(json.Unmarshal(b, &container), checker.IsNil)
	out := inspectField(c, container.ID, "HostConfig.CpusetCpus")
	c.Assert(out, checker.Equals, "")

	outMemory := inspectField(c, container.ID, "HostConfig.Memory")
	c.Assert(outMemory, checker.Equals, "0")
	outMemorySwap := inspectField(c, container.ID, "HostConfig.MemorySwap")
	c.Assert(outMemorySwap, checker.Equals, "0")
}

func (s *DockerSuite) TestContainerApiRename(c *check.C) {
	// TODO Windows: Enable for TP5. Fails on TP4.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--name", "testcontainerapirename", "-d", "busybox", "sh")

	containerID := strings.TrimSpace(out)
	newName := "testcontainerapirenamenew"
	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/rename?name="+newName, nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)

	name := inspectField(c, containerID, "Name")
	c.Assert(name, checker.Equals, "/"+newName, check.Commentf("Failed to rename container"))
}

func (s *DockerSuite) TestContainerApiKill(c *check.C) {
	name := "test-api-kill"
	runSleepingContainer(c, "-i", "--name", name)

	status, _, err := sockRequest("POST", "/containers/"+name+"/kill", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	state := inspectField(c, name, "State.Running")
	c.Assert(state, checker.Equals, "false", check.Commentf("got wrong State from container %s: %q", name, state))
}

func (s *DockerSuite) TestContainerApiRestart(c *check.C) {
	// TODO Windows to Windows CI. This is flaky due to the timing
	testRequires(c, DaemonIsLinux)
	name := "test-api-restart"
	dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")

	status, _, err := sockRequest("POST", "/containers/"+name+"/restart?t=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 5*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerApiRestartNotimeoutParam(c *check.C) {
	// TODO Windows to Windows CI. This is flaky due to the timing
	testRequires(c, DaemonIsLinux)
	name := "test-api-restart-no-timeout-param"
	out, _ := dockerCmd(c, "run", "-di", "--name", name, "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("POST", "/containers/"+name+"/restart", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Restarting  }} {{ .State.Running  }}", "false true", 50*time.Second), checker.IsNil)
}

func (s *DockerSuite) TestContainerApiStart(c *check.C) {
	name := "testing-start"
	config := map[string]interface{}{
		"Image":     "busybox",
		"Cmd":       append([]string{"/bin/sh", "-c"}, defaultSleepCommand...),
		"OpenStdin": true,
	}

	status, _, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	conf := make(map[string]interface{})
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", conf)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/start", conf)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotModified)
}

func (s *DockerSuite) TestContainerApiStop(c *check.C) {
	name := "test-api-stop"
	runSleepingContainer(c, "-i", "--name", name)

	status, _, err := sockRequest("POST", "/containers/"+name+"/stop?t=30", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(name, "{{ .State.Running  }}", "false", 60*time.Second), checker.IsNil)

	// second call to start should give 304
	status, _, err = sockRequest("POST", "/containers/"+name+"/stop?t=30", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotModified)
}

func (s *DockerSuite) TestContainerApiDeleteForce(c *check.C) {
	out, _ := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveLinks(c *check.C) {
	// Windows does not support links
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--name", "tlink1", "busybox", "top")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)
	time.Sleep(5 * time.Second)

	out, _ = dockerCmd(c, "run", "--link", "tlink1:tlink1", "--name", "tlink2", "-d", "busybox", "top")

	id2 := strings.TrimSpace(out)
	c.Assert(waitRun(id2), checker.IsNil)

	links := inspectFieldJSON(c, id2, "HostConfig.Links")
	c.Assert(links, checker.Equals, "[\"/tlink1:/tlink2/tlink1\"]", check.Commentf("expected to have links between containers"))

	status, b, err := sockRequest("DELETE", "/containers/tlink2/tlink1?link=1", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNoContent, check.Commentf(string(b)))

	linksPostRm := inspectFieldJSON(c, id2, "HostConfig.Links")
	c.Assert(linksPostRm, checker.Equals, "null", check.Commentf("call to api deleteContainer links should have removed the specified links"))
}

func (s *DockerSuite) TestContainerApiDeleteConflict(c *check.C) {
	out, _ := runSleepingContainer(c)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)
}

func (s *DockerSuite) TestContainerApiDeleteRemoveVolume(c *check.C) {
	testRequires(c, SameHostDaemon)

	vol := "/testvolume"
	if daemonPlatform == "windows" {
		vol = `c:\testvolume`
	}

	out, _ := runSleepingContainer(c, "-v", vol)

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	source, err := inspectMountSourceField(id, vol)
	_, err = os.Stat(source)
	c.Assert(err, checker.IsNil)

	status, _, err := sockRequest("DELETE", "/containers/"+id+"?v=1&force=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent)
	_, err = os.Stat(source)
	c.Assert(os.IsNotExist(err), checker.True, check.Commentf("expected to get ErrNotExist error, got %v", err))
}

func (s *DockerSuite) TestContainerApiPostContainerStop(c *check.C) {
	out, _ := runSleepingContainer(c)

	containerID := strings.TrimSpace(out)
	c.Assert(waitRun(containerID), checker.IsNil)

	statusCode, _, err := sockRequest("POST", "/containers/"+containerID+"/stop", nil)
	c.Assert(err, checker.IsNil)
	// 204 No Content is expected, not 200
	c.Assert(statusCode, checker.Equals, http.StatusNoContent)
	c.Assert(waitInspect(containerID, "{{ .State.Running  }}", "false", 5*time.Second), checker.IsNil)
}

// #14170
func (s *DockerSuite) TestPostContainerApiCreateWithStringOrSliceEntrypoint(c *check.C) {
	config := struct {
		Image      string
		Entrypoint string
		Cmd        []string
	}{"busybox", "echo", []string{"hello", "world"}}
	status, _, err := sockRequest("POST", "/containers/create?name=echotest", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image      string
		Entrypoint []string
		Cmd        []string
	}{"busybox", []string{"echo"}, []string{"hello", "world"}}
	_, _, err = sockRequest("POST", "/containers/create?name=echotest2", config2)
	c.Assert(err, checker.IsNil)
	out, _ = dockerCmd(c, "start", "-a", "echotest2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")
}

// #14170
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceCmd(c *check.C) {
	config := struct {
		Image      string
		Entrypoint string
		Cmd        string
	}{"busybox", "echo", "hello world"}
	status, _, err := sockRequest("POST", "/containers/create?name=echotest", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)
	out, _ := dockerCmd(c, "start", "-a", "echotest")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")

	config2 := struct {
		Image string
		Cmd   []string
	}{"busybox", []string{"echo", "hello", "world"}}
	_, _, err = sockRequest("POST", "/containers/create?name=echotest2", config2)
	c.Assert(err, checker.IsNil)
	out, _ = dockerCmd(c, "start", "-a", "echotest2")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello world")
}

/*
//Hyper does not support Cap
// regression #14318
func (s *DockerSuite) TestPostContainersCreateWithStringOrSliceCapAddDrop(c *check.C) {
	// Windows doesn't support CapAdd/CapDrop
	testRequires(c, DaemonIsLinux)
	config := struct {
		Image   string
		CapAdd  string
		CapDrop string
	}{"busybox", "NET_ADMIN", "SYS_ADMIN"}
	status, _, err := sockRequest("POST", "/containers/create?name=capaddtest0", config)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)

	config2 := struct {
		Image   string
		CapAdd  []string
		CapDrop []string
	}{"busybox", []string{"NET_ADMIN", "SYS_ADMIN"}, []string{"SETGID"}}
	status, _, err = sockRequest("POST", "/containers/create?name=capaddtest1", config2)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated)
}
*/

func (s *DockerSuite) TestContainerApiGetContainersJSONEmpty(c *check.C) {
	debugEndpoint = "/containers/json?all=1"
	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(string(body), checker.Equals, "[]\n")
}

/*
//Hyper does not need a json request body in start api for backwards compatibility
// #14640
func (s *DockerSuite) TestPostContainersStartWithoutLinksInHostConfig(c *check.C) {
	// TODO Windows: Windows doesn't support supplying a hostconfig on start.
	// An alternate test could be written to validate the negative testing aspect of this
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, append([]string{"create", "--name", name, "busybox"}, defaultSleepCommand...)...)

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerSuite) TestPostContainersStartWithLinksInHostConfig(c *check.C) {
	// TODO Windows: Windows doesn't support supplying a hostconfig on start.
	// An alternate test could be written to validate the negative testing aspect of this
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, "run", "--name", "foo", "-d", "busybox", "top")
	dockerCmd(c, "create", "--name", name, "--link", "foo:bar", "busybox", "top")

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerSuite) TestPostContainersStartWithLinksInHostConfigIdLinked(c *check.C) {
	// Windows does not support links
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	out, _ := dockerCmd(c, "run", "--name", "link0", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	dockerCmd(c, "create", "--name", name, "--link", id, "busybox", "top")

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := sockRequestRaw("POST", "/containers/"+name+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()
}

func (s *DockerSuite) TestContainerApiGetContainersJSONEmpty(c *check.C) {
	debugEndpoint = "/containers/json?all=1"
	status, body, err := sockRequest("GET", "/containers/json?all=1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(string(body), checker.Equals, "[]\n")
}

func (s *DockerSuite) TestStartWithNilDNS(c *check.C) {
	// TODO Windows: Add once DNS is supported
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox")
	containerID := strings.TrimSpace(out)

	config := `{"HostConfig": {"Dns": null}}`

	res, b, err := sockRequestRaw("POST", "/containers/"+containerID+"/start", strings.NewReader(config), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusNoContent)
	b.Close()

	dns := inspectFieldJSON(c, containerID, "HostConfig.Dns")
	c.Assert(dns, checker.Equals, "[]")
}
*/
