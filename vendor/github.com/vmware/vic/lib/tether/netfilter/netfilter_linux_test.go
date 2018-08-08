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

package netfilter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type input struct {
	r        *Rule
	expected string
}

func TestArgs(t *testing.T) {
	input := []input{
		{
			&Rule{
				Chain:     Input,
				States:    []State{Established},
				Interface: "external",
				Target:    Accept,
			},
			"-A VIC -m state --state ESTABLISHED -i external -j ACCEPT",
		},
		{
			&Rule{
				Chain:     Input,
				Protocol:  TCP,
				FromPort:  "7",
				Interface: "external",
				Target:    Accept,
			},
			"-A VIC -p tcp --dport 7 -i external -j ACCEPT",
		},
		{
			&Rule{
				Chain:     Input,
				Interface: "external",
				Target:    Reject,
			},
			"-A VIC -i external -j REJECT",
		},
		{
			&Rule{
				Table:     Nat,
				Chain:     Prerouting,
				Interface: "external",
				Protocol:  TCP,
				FromPort:  "80",
				Target:    Redirect,
				ToPort:    "8080",
			},
			"-t nat -A PREROUTING -p tcp --dport 80 -i external -j REDIRECT --to-port 8080",
		},
	}

	for _, rule := range input {
		args := rule.r.args()

		if !assert.Equal(t, strings.Join(args, " "), rule.expected) {
			return
		}
	}
}
