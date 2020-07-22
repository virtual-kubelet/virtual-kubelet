package suite

import (
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type basicTestSuite struct {
	setupCount      int
	testFooCount    int
	testBarCount    int
	bazCount        int
	testFooBarCount int
	testFooBazCount int
	testBarBazCount int
	teardownCount   int
	testsRan        []string
}

func (bts *basicTestSuite) Setup(t *testing.T) {
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

// TestFooBar should not be executed by the test suite
// because the number of function input is not 2 (*basicTestSuite and *testing.T)
func (bts *basicTestSuite) TestFooBar() {
	bts.testFooBarCount++
	bts.testsRan = append(bts.testsRan, "TestFooBar")
}

// TestFooBaz should not be executed by the test suite
// because the number of function output is not 0
func (bts *basicTestSuite) TestFooBaz(t *testing.T) error {
	bts.testFooBazCount++
	bts.testsRan = append(bts.testsRan, t.Name())
	return nil
}

// TestBarBaz should not be executed by the test suite
// because the type of the function input is not *testing.T
func (bts *basicTestSuite) TestBarBaz(t string) {
	bts.testBarBazCount++
	bts.testsRan = append(bts.testsRan, "TestBarBaz")
}

func TestBasicTestSuite(t *testing.T) {
	bts := new(basicTestSuite)
	Run(t, bts)

	assert.Equal(t, bts.setupCount, 1)
	assert.Equal(t, bts.testFooCount, 1)
	assert.Equal(t, bts.testBarCount, 1)
	assert.Equal(t, bts.teardownCount, 1)
	assert.Assert(t, is.Len(bts.testsRan, 2))
	assertTestsRan(t, bts.testsRan)
	assertNonTests(t, bts)
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
	assert.Equal(t, sts.teardownCount, 1)
	assert.Equal(t, sts.skippedTestCount, 1)
	assert.Assert(t, is.Len(sts.testsRan, 1))
	assertTestsRan(t, sts.testsRan)
	assertNonTests(t, &sts.basicTestSuite)
}

func assertTestsRan(t *testing.T, testsRan []string) {
	for _, testRan := range testsRan {
		parts := strings.Split(testRan, "/")
		// Make sure that the name of the test has exactly one parent name and one subtest name
		assert.Assert(t, is.Len(parts, 2))
		// Check the parent test's name
		assert.Equal(t, parts[0], t.Name())
	}
}

// assertNonTests ensures that any malformed test functions are not run by the test suite
func assertNonTests(t *testing.T, bts *basicTestSuite) {
	assert.Equal(t, bts.bazCount, 0)
	assert.Equal(t, bts.testFooBarCount, 0)
	assert.Equal(t, bts.testFooBazCount, 0)
	assert.Equal(t, bts.testBarBazCount, 0)
}
