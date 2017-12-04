package main

import (
	//"fmt"
	//"io/ioutil"
	//"os"
	//"os/exec"
	//"path/filepath"
	"sort"
	//"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliPsListContainersBasic(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := runSleepingContainer(c, "-d")
	firstID := strings.TrimSpace(out)

	out, _ = runSleepingContainer(c, "-d")
	secondID := strings.TrimSpace(out)

	// not long running
	out, _ = dockerCmd(c, "run", "-d", "busybox", "true")
	thirdID := strings.TrimSpace(out)

	out, _ = runSleepingContainer(c, "-d")
	fourthID := strings.TrimSpace(out)

	// make sure the second is running
	c.Assert(waitRun(secondID), checker.IsNil)

	// make sure third one is not running
	dockerCmd(c, "stop", thirdID)

	// make sure the forth is running
	c.Assert(waitRun(fourthID), checker.IsNil)

	// all
	out, _ = dockerCmd(c, "ps", "-a")
	c.Assert(assertContainerList(out, []string{fourthID, thirdID, secondID, firstID}), checker.Equals, true, check.Commentf("ALL: Container list is not in the correct order: \n%s", out))

	// running
	out, _ = dockerCmd(c, "ps")
	c.Assert(assertContainerList(out, []string{fourthID, secondID, firstID}), checker.Equals, true, check.Commentf("RUNNING: Container list is not in the correct order: \n%s", out))

	// from here all flag '-a' is ignored

	// limit
	out, _ = dockerCmd(c, "ps", "-n=2", "-a")
	expected := []string{fourthID, thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("LIMIT & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-n=2")
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("LIMIT: Container list is not in the correct order: \n%s", out))

	// filter since
	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-a")
	expected = []string{fourthID, thirdID, secondID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID)
	expected = []string{fourthID, secondID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE: Container list is not in the correct order: \n%s", out))

	// filter before
	out, _ = dockerCmd(c, "ps", "-f", "before="+thirdID, "-a")
	expected = []string{secondID, firstID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("BEFORE & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "before="+fourthID)
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("BEFORE: Container list is not in the correct order: \n%s", out))

	// filter since & before
	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-f", "before="+fourthID, "-a")
	expected = []string{thirdID, secondID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, BEFORE & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-f", "before="+fourthID)
	expected = []string{secondID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, BEFORE: Container list is not in the correct order: \n%s", out))

	// filter since & limit
	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-n=2", "-a")
	expected = []string{fourthID, thirdID}

	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, LIMIT & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-n=2")
	expected = []string{fourthID, thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, LIMIT: Container list is not in the correct order: \n%s", out))

	// filter before & limit
	out, _ = dockerCmd(c, "ps", "-f", "before="+fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("BEFORE, LIMIT & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "before="+fourthID, "-n=1")
	expected = []string{thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("BEFORE, LIMIT: Container list is not in the correct order: \n%s", out))

	// filter since & filter before & limit
	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-f", "before="+fourthID, "-n=1", "-a")
	expected = []string{thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, BEFORE, LIMIT & ALL: Container list is not in the correct order: \n%s", out))

	out, _ = dockerCmd(c, "ps", "-f", "since="+firstID, "-f", "before="+fourthID, "-n=1")
	expected = []string{thirdID}
	c.Assert(assertContainerList(out, expected), checker.Equals, true, check.Commentf("SINCE, BEFORE, LIMIT: Container list is not in the correct order: \n%s", out))

}

func assertContainerList(out string, expected []string) bool {
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	if len(lines)-1 != len(expected) {
		return false
	}
	containerIDIndex := strings.Index(lines[0], "CONTAINER ID")
	for i := 0; i < len(expected); i++ {
		foundID := lines[i+1][containerIDIndex : containerIDIndex+12]
		if foundID != expected[i][:12] {
			return false
		}
	}
	return true
}

//TODO: fix container size
/*func (s *DockerSuite) TestCliPsListContainersSize(c *check.C) {
	// Problematic on Windows as it doesn't report the size correctly @swernli
	printTestCaseName(); defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "busybox", "echo", "hello")

	baseOut, _ := dockerCmd(c, "ps", "-s", "-n=1")
	baseLines := strings.Split(strings.Trim(baseOut, "\n "), "\n")
	baseSizeIndex := strings.Index(baseLines[0], "SIZE")
	baseFoundsize := baseLines[1][baseSizeIndex:]
	baseBytes, err := strconv.Atoi(strings.Split(baseFoundsize, " ")[0])
	c.Assert(err, checker.IsNil)

	name := "test-size"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", "echo 1 > test")
	id, err := getIDByName(name)
	c.Assert(err, checker.IsNil)

	runCmd := exec.Command(dockerBinary, "--region="+os.Getenv("DOCKER_HOST"), "ps", "-s", "-n=1")

	wait := make(chan struct{})
	go func() {
		out, _, err = runCommandWithOutput(runCmd)
		close(wait)
	}()
	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		c.Fatalf("Calling \"docker ps -s\" timed out!")
	}
	c.Assert(err, checker.IsNil)
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	c.Assert(lines, checker.HasLen, 2, check.Commentf("Expected 2 lines for 'ps -s -n=1' output, got %d", len(lines)))
	sizeIndex := strings.Index(lines[0], "SIZE")
	idIndex := strings.Index(lines[0], "CONTAINER ID")
	foundID := lines[1][idIndex : idIndex+12]
	c.Assert(foundID, checker.Equals, id[:12], check.Commentf("Expected id %s, got %s", id[:12], foundID))
	expectedSize := fmt.Sprintf("%d B", (2 + baseBytes))
	foundSize := lines[1][sizeIndex:]
	c.Assert(foundSize, checker.Contains, expectedSize, check.Commentf("Expected size %q, got %q", expectedSize, foundSize))

}*/

func (s *DockerSuite) TestCliPsListContainersFilterStatusBasic(c *check.C) {
	// start exited container
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox")
	firstID := strings.TrimSpace(out)

	// make sure the exited container is not running
	dockerCmd(c, "stop", firstID)

	// start running container
	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	secondID := strings.TrimSpace(out)

	// filter containers by exited
	out, _ = dockerCmd(c, "ps", "--no-trunc", "-q", "--filter=status=exited")
	containerOut := strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, firstID)

	out, _ = dockerCmd(c, "ps", "-a", "--no-trunc", "-q", "--filter=status=running")
	containerOut = strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, secondID)

	out, _, _ = dockerCmdWithTimeout(time.Second*60, "ps", "-a", "-q", "--filter=status=rubbish")
	c.Assert(out, checker.Contains, "Unrecognised filter value for status", check.Commentf("Expected error response due to invalid status filter output: %q", out))
}

func (s *DockerSuite) TestCliPsListContainersFilterID(c *check.C) {
	// start container
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	runSleepingContainer(c)

	// filter containers by id
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--filter=id="+firstID)
	containerOut := strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, firstID[:12], check.Commentf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out))

}

func (s *DockerSuite) TestCliPsListContainersFilterNameBasic(c *check.C) {
	// start container
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "run", "-d", "--name=a-name-to-match", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	runSleepingContainer(c, "--name=b-name-to-match")

	// filter containers by name
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--filter=name=a-name-to-match")
	containerOut := strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, firstID[:12], check.Commentf("Expected id %s, got %s for exited filter, output: %q", firstID[:12], containerOut, out))

}

func checkPsAncestorFilterOutput(c *check.C, out string, filterName string, expectedIDs []string) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	actualIDs := []string{}
	if out != "" {
		actualIDs = strings.Split(out[:len(out)-1], "\n")
	}
	sort.Strings(actualIDs)
	sort.Strings(expectedIDs)

	c.Assert(actualIDs, checker.HasLen, len(expectedIDs), check.Commentf("Expected filtered container(s) for %s ancestor filter to be %v:%v, got %v:%v", filterName, len(expectedIDs), expectedIDs, len(actualIDs), actualIDs))
	if len(expectedIDs) > 0 {
		same := true
		for i := range expectedIDs {
			if actualIDs[i] != expectedIDs[i] {
				c.Logf("%s, %s", actualIDs[i], expectedIDs[i])
				same = false
				break
			}
		}
		c.Assert(same, checker.Equals, true, check.Commentf("Expected filtered container(s) for %s ancestor filter to be %v, got %v", filterName, expectedIDs, actualIDs))
	}
}

func (s *DockerSuite) TestCliPsListContainersFilterLabel(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	// start container
	out, _ := dockerCmd(c, "run", "-d", "-l", "match=me", "-l", "second=tag", "busybox")
	firstID := strings.TrimSpace(out)

	// start another container
	out, _ = dockerCmd(c, "run", "-d", "-l", "match=me too", "busybox")
	secondID := strings.TrimSpace(out)

	// start third container
	out, _ = dockerCmd(c, "run", "-d", "-l", "nomatch=me", "busybox")
	thirdID := strings.TrimSpace(out)

	// filter containers by exact match
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me")
	containerOut := strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, firstID, check.Commentf("Expected id %s, got %s for exited filter, output: %q", firstID, containerOut, out))

	// filter containers by two labels
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me", "--filter=label=second=tag")
	containerOut = strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, firstID, check.Commentf("Expected id %s, got %s for exited filter, output: %q", firstID, containerOut, out))

	// filter containers by two labels, but expect not found because of AND behavior
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match=me", "--filter=label=second=tag-no")
	containerOut = strings.TrimSpace(out)
	c.Assert(containerOut, checker.Equals, "", check.Commentf("Expected nothing, got %s for exited filter, output: %q", containerOut, out))

	// filter containers by exact key
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=label=match")
	containerOut = strings.TrimSpace(out)
	c.Assert(containerOut, checker.Contains, firstID)
	c.Assert(containerOut, checker.Contains, secondID)
	c.Assert(containerOut, checker.Not(checker.Contains), thirdID)
}

func (s *DockerSuite) TestCliPsListContainersFilterExited(c *check.C) {
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	runSleepingContainer(c, "--name=sleep")

	dockerCmd(c, "run", "--name", "zero1", "busybox", "true")
	firstZero, err := getIDByName("zero1")
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "run", "--name", "zero2", "busybox", "true")
	secondZero, err := getIDByName("zero2")
	c.Assert(err, checker.IsNil)

	out, _, err := dockerCmdWithError("run", "--name", "nonzero1", "busybox", "false")
	//TODO: generate err when exited code is not 0
	//c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))

	firstNonZero, err := getIDByName("nonzero1")
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("run", "--name", "nonzero2", "busybox", "false")
	//TODO: generate err when exited code is not 0
	//c.Assert(err, checker.NotNil, check.Commentf("Should fail.", out, err))
	secondNonZero, err := getIDByName("nonzero2")
	c.Assert(err, checker.IsNil)

	// filter containers by exited=0
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=exited=0")
	ids := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(ids, checker.HasLen, 2, check.Commentf("Should be 2 zero exited containers got %d: %s", len(ids), out))
	c.Assert(ids[0], checker.Equals, secondZero, check.Commentf("First in list should be %q, got %q", secondZero, ids[0]))
	c.Assert(ids[1], checker.Equals, firstZero, check.Commentf("Second in list should be %q, got %q", firstZero, ids[1]))

	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=exited=1")
	ids = strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(ids, checker.HasLen, 2, check.Commentf("Should be 2 zero exited containers got %d", len(ids)))
	c.Assert(ids[0], checker.Equals, secondNonZero, check.Commentf("First in list should be %q, got %q", secondNonZero, ids[0]))
	c.Assert(ids[1], checker.Equals, firstNonZero, check.Commentf("Second in list should be %q, got %q", firstNonZero, ids[1]))

}

//TODO: SAME AS ps format multi names
/*func (s *DockerSuite) TestCliPsLinkedWithNoTrunc(c *check.C) {
	// Problematic on Windows as it doesn't support links as of Jan 2016
	printTestCaseName(); defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	runSleepingContainer(c, "--name=first")
	runSleepingContainer(c, "--name=second", "--link=first:first")

	out, _ := dockerCmd(c, "ps", "--no-trunc")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// strip header
	lines = lines[1:]
	expected := []string{"second", "first,second/first"}
	var names []string
	for _, l := range lines {
		fields := strings.Fields(l)
		names = append(names, fields[len(fields)-1])
	}
	c.Assert(expected, checker.DeepEquals, names, check.Commentf("Expected array: %v, got: %v", expected, names))
}*/

func (s *DockerSuite) TestCliPsWithSize(c *check.C) {
	// Problematic on Windows as it doesn't report the size correctly @swernli
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	dockerCmd(c, "run", "-d", "--name", "sizetest", "busybox", "top")

	out, _ := dockerCmd(c, "ps", "--size")
	c.Assert(out, checker.Contains, "virtual", check.Commentf("docker ps with --size should show virtual size of container"))
}

func (s *DockerSuite) TestCliPsListContainersFilterCreated(c *check.C) {
	// create a container
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	pullImageIfNotExist("busybox")
	out, _ := dockerCmd(c, "create", "busybox")
	cID := strings.TrimSpace(out)
	shortCID := cID[:12]

	// Make sure it DOESN'T show up w/o a '-a' for normal 'ps'
	out, _ = dockerCmd(c, "ps", "-q")
	c.Assert(out, checker.Not(checker.Contains), shortCID, check.Commentf("Should have not seen '%s' in ps output:\n%s", shortCID, out))

	// Make sure it DOES show up as 'Created' for 'ps -a'
	out, _ = dockerCmd(c, "ps", "-a")

	hits := 0
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, shortCID) {
			continue
		}
		hits++
		c.Assert(line, checker.Contains, "Created", check.Commentf("Missing 'Created' on '%s'", line))
	}

	c.Assert(hits, checker.Equals, 1, check.Commentf("Should have seen '%s' in ps -a output once:%d\n%s", shortCID, hits, out))

	// filter containers by 'create' - note, no -a needed
	out, _ = dockerCmd(c, "ps", "-q", "-f", "status=created")
	containerOut := strings.TrimSpace(out)
	c.Assert(cID, checker.HasPrefix, containerOut)
}

//TODO: fix ps format multi names
/*func (s *DockerSuite) TestCliPsFormatMultiNames(c *check.C) {
	// Problematic on Windows as it doesn't support link as of Jan 2016
	printTestCaseName(); defer printTestDuration(time.Now())

	testRequires(c, DaemonIsLinux)
	//create 2 containers and link them
	dockerCmd(c, "run", "--name=child", "-d", "busybox", "top")
	dockerCmd(c, "run", "--name=parent", "--link=child:linkedone", "-d", "busybox", "top")

	//use the new format capabilities to only list the names and --no-trunc to get all names
	out, _ := dockerCmd(c, "ps", "--format", "{{.Names}}", "--no-trunc")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	expected := []string{"parent", "child,parent/linkedone"}
	var names []string
	for _, l := range lines {
		names = append(names, l)
	}
	c.Assert(expected, checker.DeepEquals, names, check.Commentf("Expected array with non-truncated names: %v, got: %v", expected, names))

	//now list without turning off truncation and make sure we only get the non-link names
	out, _ = dockerCmd(c, "ps", "--format", "{{.Names}}")
	lines = strings.Split(strings.TrimSpace(string(out)), "\n")
	expected = []string{"parent", "child"}
	var truncNames []string
	for _, l := range lines {
		truncNames = append(truncNames, l)
	}
	c.Assert(expected, checker.DeepEquals, truncNames, check.Commentf("Expected array with truncated names: %v, got: %v", expected, truncNames))

}*/

func (s *DockerSuite) TestCliPsFormatHeaders(c *check.C) {
	// make sure no-container "docker ps" still prints the header row
	printTestCaseName()
	defer printTestDuration(time.Now())
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "ps", "--format", "table {{.ID}}")
	c.Assert(out, checker.Equals, "CONTAINER ID\n", check.Commentf(`Expected 'CONTAINER ID\n', got %v`, out))

	// verify that "docker ps" with a container still prints the header row also
	runSleepingContainer(c, "--name=test")
	out, _ = dockerCmd(c, "ps", "--format", "table {{.Names}}")
	c.Assert(out, checker.Equals, "NAMES\ntest\n", check.Commentf(`Expected 'NAMES\ntest\n', got %v`, out))
}
