// Copyright 2016 VMware, Inc. All Rights Reserved.
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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"parser"
)

var exceptionFile = flag.String("exceptionFile", "", "json file containing documented exceptions to using tagged releases")

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: Formats the json output of groupbyrepo and considers json exceptions. Takes input via stdin. Eg: gettags | groupbyrepo | report")
		flag.PrintDefaults()
	}
	flag.Parse()
}

func printOutput(out io.Writer, repos parser.SortedManifestByRepo, exceptions map[string]string) {
	for _, repo := range repos {
		fmt.Fprintf(out, "\n%s\n", repo.Repository)
		for _, dep := range repo.Dependencies {
			fmt.Fprintf(out, "\t%s", dep.Importpath)
			if exceptions[dep.Revision] == "" {
				if dep.HasTags {
					if dep.RevisionTag != "" {
						fmt.Fprintf(out, " is tagged as "+dep.RevisionTag)
					} else {
						fmt.Fprintf(out, " "+dep.Revision+" is an untagged version. Most recent tag is %s for sha %s", dep.SuggestedTag, dep.SuggestedRev)
					}
				} else {
					if strings.Index(dep.Repository, "github.com") == -1 {
						fmt.Fprintf(out, " "+dep.Revision+" sha is from non-Github repo")
					} else {
						fmt.Fprintf(out, " "+dep.Revision+" sha is from a repo with no tags")
					}
				}
			} else {
				fmt.Fprintf(out, " "+dep.Revision+" MUST BE THIS REVISION because %s", exceptions[dep.Revision])
			}
			fmt.Fprintf(out, "\n")
		}
	}
}

func loadAndParseExceptions() (map[string]string, error) {
	var exceptions map[string]string
	var err error

	if *exceptionFile != "" {
		exceptions = make(map[string]string)
		data, err := ioutil.ReadFile(*exceptionFile)
		if err == nil {
			e := []parser.UntaggedException{}
			err := json.Unmarshal(data, &e)
			if err == nil {
				for _, v := range e {
					exceptions[v.Revision] = v.Reason
				}
			}
		}
	}
	return exceptions, err
}

// Takes the output from groupbyrepo, plus exception data from a file, and produces a human-parsable report
func main() {

	// Load the json from stdin (designed to support piping between binaries)
	input, err := parser.ReadFromStdin()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	// Parse the data into useful types
	repos, err := parser.ParseManifestByRepo(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input json: %v\n", err)
		os.Exit(1)
	}

	// Load the exception file into useful types
	exceptions, err := loadAndParseExceptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading exceptions file: %v\n", err)
		os.Exit(1)
	}

	// Write the report to stdout
	sort.Sort(parser.SortedManifestByRepo(repos))
	printOutput(os.Stdout, repos, exceptions)
}
