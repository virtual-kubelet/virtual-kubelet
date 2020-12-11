// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"net/http"
)

// Middleware defines a middleware that is to
// take one  HandlerFunc and wrap it within another HandlerFunc
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Middlewares is a function to inject multiple middlewares (in orders)
func Middlewares(hf http.HandlerFunc, ms ...Middleware) http.HandlerFunc {
	if len(ms) < 1 {
		return hf
	}

	wrapper := hf

	for i := len(ms) - 1; i >= 0; i-- {
		wrapper = ms[i](wrapper)
	}
	return wrapper
}
