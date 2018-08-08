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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"parser"
)

var client *http.Client

var uid = flag.String("uid", "", "valid Github userid")
var pwd = flag.String("pwd", "", "valid Github password")

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: Queries Github for tags for each dependency sha in manifest input. Takes input via stdin. Eg: cat manifest | gettags")
		fmt.Println("       Uses basic auth, specified by --uid=<uid> and --pwd=<pwd>. Failure to specify will likely hit Github API limits")
		flag.PrintDefaults()
	}
	flag.Parse()
}

func findTagForSha(data []parser.TagData, sha string) string {
	for _, tag := range data {
		if tag.Commit.Sha == sha {
			return tag.Name
		}
	}
	return ""
}

func queryGit(u *url.URL) ([]byte, error) {
	getPath := "https://api.github.com/repos" + u.Path + "/tags"
	req, _ := http.NewRequest("GET", getPath, nil)
	if *uid != "" && *pwd != "" {
		req.SetBasicAuth(*uid, *pwd)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error attempting HTTP GET to %v: %v", u, err)
	}
	return ioutil.ReadAll(resp.Body)
}

// Currently only works for Github URLs as this is the vast majority. Rest will have HasTags == false
func findTag(entry *parser.ManifestEntry) (*parser.ManifestEntry, error) {
	u, err := url.Parse(entry.Repository)
	if err != nil {
		return nil, fmt.Errorf("Malformed repository URL: %v", err)
	}
	if u.Host == "github.com" {
		tagdata, err := queryGit(u)
		if err != nil {
			return nil, err
		}
		td := []parser.TagData{}
		err = json.Unmarshal(tagdata, &td)
		if err != nil {
			return nil, fmt.Errorf("Unparsable output: %s: %v", string(tagdata), err)
		}
		entry.HasTags = (len(td) == 0)
		if len(td) == 0 {
			entry.HasTags = false
		} else {
			entry.HasTags = true
			foundTag := findTagForSha(td, entry.Revision)
			if foundTag != "" {
				entry.RevisionTag = foundTag
			} else {
				entry.SuggestedTag = td[0].Name
				entry.SuggestedRev = td[0].Commit.Sha
			}
		}
	}
	return entry, err
}

// Take the vendor manifest file and uses the Github APIs to retrieve the tag data for each Revision
// Outputted data can be consumed by other parsers, or can be input to groupbyrepo
func main() {

	// Load the json from stdin (designed to support piping between binaries)
	input, err := parser.ReadFromStdin()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	client = &http.Client{}

	// Parse the data into useful types
	m, err := parser.ParseManifest(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input json: %v\n", err)
		os.Exit(1)
	}

	// Find the tag for each repository. This is done serially.
	var foundTag *parser.ManifestEntry
	if err == nil {
		for i, me := range m.Dependencies {
			foundTag, err = findTag(&me)
			if err != nil {
				break
			}
			m.Dependencies[i] = *foundTag
		}
	}

	// Output the original manifest data with new fields added (see ManifestEntry)
	if err == nil {
		output, _ := json.MarshalIndent(m, "", "   ")
		fmt.Println(string(output))
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
