// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"testing"

	flags "github.com/jessevdk/go-flags"
)

var systemTest *bool
var flagArgs []string

// testOptions is the list of options used by the standard testing package in a format
// that the go-flags package can understand. All the flags are listed as of type
// string, since we only need to parse them and pass them on to the standard
// go flag package. The drawback of this is the boolean flags need to be
// specified with a value, e.g. --test.v=true, instead of just --test.v.
// This is also needed since the go-flags package does not accept a value
// for boolean arguments (i.e. --test.v=true is illegal), while the
// standard go flag package does, making calls by go test (which uses values
// for boolean arguments) illegal when passed through go-flags.
type testOptions struct {
	// Report as tests are run; default is silent for success.
	Short            string `long:"test.short" default:"false" description:"run smaller test suite to save time"`
	OutputDir        string `long:"test.outputdir" default:"" description:"directory in which to write profiles"`
	Chatty           string `long:"test.v" default:"false" description:"verbose: print additional output"`
	Count            string `long:"test.count" default:"1" description:"run tests and benchmarks n times"`
	CoverProfile     string `long:"test.coverprofile" default:"" description:"write a coverage profile to the named file after execution"`
	Match            string `long:"test.run" default:"" description:"regular expression to select tests and examples to run"`
	MemProfile       string `long:"test.memprofile" default:"" description:"write a memory profile to the named file after execution"`
	MemProfileRate   string `long:"test.memprofilerate" default:"0" description:"if >=0, sets runtime.MemProfileRate"`
	CPUProfile       string `long:"test.cpuprofile" default:"" description:"write a cpu profile to the named file during execution"`
	BlockProfile     string `long:"test.blockprofile" default:"" description:"write a goroutine blocking profile to the named file after execution"`
	BlockProfileRate string `long:"test.blockprofilerate" default:"1" description:"if >= 0, calls runtime.SetBlockProfileRate()"`
	TraceFile        string `long:"test.trace" default:"" description:"write an execution trace to the named file after execution"`
	Timeout          string `long:"test.timeout" default:"0" description:"if positive, sets an aggregate time limit for all tests"`
	CPUListStr       string `long:"test.cpu" default:"" description:"comma-separated list of number of CPUs to use for each test"`
	Parallel         string `long:"test.parallel" description:"maximum test parallelism"`

	MatchBenchmarks string `long:"test.bench" default:"" description:"regular expression per path component to select benchmarks to run"`
	BenchTime       string `long:"test.benchtime" description:"approximate run time for each benchmark"`
	BenchmarkMemory string `long:"test.benchmem" default:"false" description:"print memory allocations for benchmarks"`

	SystemTest string `long:"systemTest" default:"false" description:"run system test"`
}

var testOpts testOptions
var testGroup *flags.Group

func init() {
	systemTest = flag.Bool("systemTest", false, "Run system test")
}

// addDash adds an extra dash to long options
// specified with a single dash, e.g. addDash("-test.v")
// returns "--test.v". Other strings are
// returned unchanged.
func addDash(s string) string {
	if len(s) > 2 && s[0] == '-' && s[1] != '-' {
		return "-" + s
	}

	return s
}

func TestMain(m *testing.M) {
	// make sure all single dash arguments
	// are converted to using double dashes
	// to make them parseable by the go-flags
	// package
	for i := range os.Args {
		if i == 0 {
			continue
		}

		if os.Args[i] == "--" {
			break
		}

		os.Args[i] = addDash(os.Args[i])
	}

	// create a new parser to parse just the test options
	testParser := flags.NewParser(nil, flags.IgnoreUnknown|flags.Default)
	testGroup, _ = testParser.AddGroup("go test Options", "go test Options", &testOpts)
	_, err := testParser.Parse()
	if err != nil {
		if err := err.(*flags.Error); err != nil && err.Type == flags.ErrHelp {
			// do not exit if --systemTest == "true", so that we
			// can output the help for the port-layer-server test
			// binary
			o := testGroup.FindOptionByLongName("systemTest")
			if !o.IsSet() || o.Value().(string) != "true" {
				os.Exit(0)
			}
		} else {
			os.Exit(1)
		}
	}

	// build up arguments to pass to the standard
	// go flag package
	for _, o := range testGroup.Options() {
		if !o.IsSet() {
			continue
		}

		flagArgs = append(flagArgs, fmt.Sprintf("-%s=%s", o.LongName, o.Value()))
	}

	flag.CommandLine.Parse(flagArgs)

	// execute tests
	os.Exit(m.Run())
}

func TestSystem(t *testing.T) {
	if *systemTest {
		// add the test options to the default parser
		parser.AddGroup("go test Options", "go test Options", &testOpts)

		main()
	}
}
