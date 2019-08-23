package suite

import (
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type basicTestSuite struct {
	setupCount    int
	testFooCount  int
	testBarCount  int
	bazCount      int
	teardownCount int
	testsRan      []string
}

func (bts *basicTestSuite) Setup() {
	bts.setupCount++
}

func (bts *basicTestSuite) Teardown() {
	bts.teardownCount++
}

func (bts *basicTestSuite) TestFoo(t *testing.T) {
	bts.testFooCount++
	bts.testsRan = append(bts.testsRan, t.Name())
}

func (bts *basicTestSuite) TestBar(t *testing.T) {
	bts.testBarCount++
	bts.testsRan = append(bts.testsRan, t.Name())
}

// Baz should not be executed by the test suite
// because it does not have the prefix 'Test'
func (bts *basicTestSuite) Baz(t *testing.T) {
	bts.bazCount++
	bts.testsRan = append(bts.testsRan, t.Name())
}

func TestBasicTestSuite(t *testing.T) {
	bts := new(basicTestSuite)
	Run(t, bts)

	assert.Equal(t, bts.setupCount, 1)
	assert.Equal(t, bts.testFooCount, 1)
	assert.Equal(t, bts.testBarCount, 1)
	assert.Equal(t, bts.bazCount, 0)
	assert.Equal(t, bts.teardownCount, 1)
	assert.Assert(t, is.Len(bts.testsRan, 2))
	checkTestName(t, bts.testsRan)
}

type skipTestSuite struct {
	basicTestSuite
	skippedTestCount int
}

func (sts *skipTestSuite) ShouldSkipTest(testName string) bool {
	if testName == "TestBar" {
		sts.skippedTestCount++
		return true
	}
	return false
}

func TestSkipTest(t *testing.T) {
	sts := new(skipTestSuite)
	Run(t, sts)

	assert.Equal(t, sts.setupCount, 1)
	assert.Equal(t, sts.testFooCount, 1)
	assert.Equal(t, sts.testBarCount, 0)
	assert.Equal(t, sts.bazCount, 0)
	assert.Equal(t, sts.teardownCount, 1)
	assert.Equal(t, sts.skippedTestCount, 1)
	assert.Assert(t, is.Len(sts.testsRan, 1))
	checkTestName(t, sts.testsRan)
}

func checkTestName(t *testing.T, testsRan []string) {
	for _, testRan := range testsRan {
		parts := strings.Split(testRan, "/")
		// Make sure that the subtest has only one parent test
		assert.Assert(t, is.Len(parts, 2))
		// Check the parent test's name
		assert.Equal(t, parts[0], t.Name())
	}
}

type panickingTestSuite struct {
	basicTestSuite
	panicDuringSetup    bool
	panicDuringTeardown bool
}

func (pts *panickingTestSuite) Setup() {
	pts.setupCount++
	if pts.panicDuringSetup {
		panic("Error in Setup()")
	}
}

func (pts *panickingTestSuite) Teardown() {
	pts.teardownCount++
	if pts.panicDuringTeardown {
		panic("Error in Teardown()")
	}
}

func (pts *panickingTestSuite) FailOnPanic(t *testing.T) {
	// noop
	// Not invoking t.Fail() because it will actually fail TestRecoverFromPanic.
	// Also not invoking recover() here because this method won't be run on the
	// same goroutine as the test, therefore, recover() will return nil.
}

// TestPanickingTestSuiteDuringSetup ensures the correct flow
// when a panic happens in Setup()
func TestPanickingTestSuiteDuringSetup(t *testing.T) {
	pts := panickingTestSuite{
		panicDuringSetup: true,
	}

	// This makes sure we can obtain panic signal through recover()
	// because panic and recover are in the same goroutine
	func(t *testing.T) {
		defer func() {
			r := recover()
			assert.Assert(t, r != nil)
			t.Logf("Recovered from the following panic: %s", r)
		}()
		Run(t, &pts)
	}(t)

	// Make sure only the setup functions has been run
	assert.Equal(t, pts.setupCount, 1)
	assert.Equal(t, pts.testFooCount, 0)
	assert.Equal(t, pts.testBarCount, 0)
	assert.Equal(t, pts.bazCount, 0)
	assert.Equal(t, pts.teardownCount, 0)
	assert.Assert(t, is.Len(pts.testsRan, 0))
}

// TestPanickingTestSuiteDuringSetup ensures the correct flow
// when a panic happens in Teardown()
func TestPanickingTestSuiteDuringTeardown(t *testing.T) {
	pts := panickingTestSuite{
		panicDuringTeardown: true,
	}

	// This makes sure we can obtain panic signal through recover()
	// because panic and recover are in the same goroutine
	func() {
		defer func() {
			r := recover()
			assert.Assert(t, r != nil)
			t.Logf("Recovered from the following panic: %s", r)
		}()
		Run(t, &pts)
	}()

	assert.Equal(t, pts.setupCount, 1)
	assert.Equal(t, pts.testFooCount, 1)
	assert.Equal(t, pts.testBarCount, 1)
	assert.Equal(t, pts.bazCount, 0)
	assert.Equal(t, pts.teardownCount, 1)
	assert.Assert(t, is.Len(pts.testsRan, 2))
}
