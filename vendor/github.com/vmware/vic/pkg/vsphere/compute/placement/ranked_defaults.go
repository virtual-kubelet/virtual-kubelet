// Copyright 2018 VMware, Inc. All Rights Reserved.
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

package placement

const (
	// These default values serve as a simple weighting approach towards unconsumed vs. inactive memory.
	// They are basically an initial arbitrary guess with the intention of preferring a host with more
	// unconsumed memory vs. one with more active memory.

	// memDefaultInactiveWeight defines the default value used to weight inactive memory when ranking hosts by metric.
	memDefaultInactiveWeight = 0.3
	// memDefaultUnconsumedWeight defines the default value used to weight unconsumed memory when ranking hosts by metric.
	memDefaultUnconsumedWeight = 0.7
)
