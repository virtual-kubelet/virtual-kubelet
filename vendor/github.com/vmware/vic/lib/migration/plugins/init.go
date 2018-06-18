// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package plugins

// import all plugin packages here to register plugins
import (
	// imported for the side effect
	_ "github.com/vmware/vic/lib/migration/plugins/plugin1"
	_ "github.com/vmware/vic/lib/migration/plugins/plugin2"
	_ "github.com/vmware/vic/lib/migration/plugins/plugin5"
	_ "github.com/vmware/vic/lib/migration/plugins/plugin7"
	_ "github.com/vmware/vic/lib/migration/plugins/plugin8"
	_ "github.com/vmware/vic/lib/migration/plugins/plugin9"
)
