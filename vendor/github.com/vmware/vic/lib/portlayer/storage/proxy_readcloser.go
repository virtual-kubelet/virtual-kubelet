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

package storage

import (
	"io"
)

// ProxyReadCloser is a read closer that provides for wrapping the Close with
// a custom Close call. The original ReadCloser.Close function will be invoked
// after the custom call. Errors from the custom call with be ignored.
type ProxyReadCloser struct {
	io.ReadCloser
	Closer func() error
}

func (p *ProxyReadCloser) Close() error {
	/* #nosec - no useful way to handle this error */
	p.Closer()
	return p.ReadCloser.Close()
}
