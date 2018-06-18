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

package extraconfig

/*
Package extraconfig provides Encode/Decode methods to convert data between Go structs and VMware Extraconfig values.

The implementation understands the following set of annotations and map the fields to appropriate extraConfig keys - in the case where the key describes a boolean state, omitting the annotation implies the opposite:

hidden - hidden from GuestOS
read-only - value can only be modified via vSphere APIs
read-write - value can be modified
non-persistent - value will be lost on VM reboot
volatile - field is not exported directly, but via a function that freshens the value each time)

The struct fields are required to be annotated with the "vic" tag, otherwise extraconfig package simply skips them. Scope and key tags are also required.

Scope tag can contain multiple values (comma separated)
Key tag can contain extra properties (comma separated) but the first element has to the name of the key.

type Example struct {
    // skipped - does not contain any tag
	Note string

    // skipped - does not contain scope and key
	ID string `vic:"0.1"`

    // valid - extraconfig will encode this using a read-only key (as instructed by scope)
	Name string `vic:"0.1" scope:"read-only" key:"name"`

    // valid - but extraconfig won't nest into the struct (so it's value will be type's zero value)
	Time time.Time `vic:"0.1" scope:"volatile" key:"time,omitnested"`
}

*/
