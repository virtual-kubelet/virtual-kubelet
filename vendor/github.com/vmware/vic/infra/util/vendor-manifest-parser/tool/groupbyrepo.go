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
	"net/url"
	"os"

	"parser"
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: Groups the json output of gettags by repository. Takes input via stdin. Eg: gettags | groupbyrepo")
	}
	flag.Parse()
}

func groupByRepo(m parser.Manifest) []*parser.ManifestByRepo {
	var repoMap = make(map[string]*parser.ManifestByRepo)

	for _, me := range m.Dependencies {
		u, _ := url.Parse(me.Repository)
		repo := u.Path
		mapEntry := repoMap[repo]
		if mapEntry == nil {
			mapEntry = &parser.ManifestByRepo{Repository: repo}
		}
		mapEntry.Dependencies = append(mapEntry.Dependencies, me)
		repoMap[repo] = mapEntry
	}

	values := make([]*parser.ManifestByRepo, len(repoMap))
	i := 0
	for _, k := range repoMap {
		values[i] = k
		i++
	}
	return values
}

// Takes json output from gettags and groups it by repository. Output is in json format and can be further parsed by report
func main() {

	// Read json data from stdin (designed to support piping between binaries)
	input, err := parser.ReadFromStdin()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	// Parse the data into useful types
	m, err := parser.ParseManifest(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input json: %v", err)
		flag.Usage()
		os.Exit(1)
	}

	// Group the data into a slice of repo->dependencies
	values := groupByRepo(m)

	// Format the output to stdout
	output, err := json.MarshalIndent(values, "", "   ")
	fmt.Println(string(output))
}
