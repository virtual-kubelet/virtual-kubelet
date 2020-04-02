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
	"io"
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

type handlerFunc func(http.ResponseWriter, *http.Request) error

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code := httpStatusCode(err)
		w.WriteHeader(code)
		io.WriteString(w, err.Error()) //nolint:errcheck
		logger := log.G(req.Context()).WithError(err).WithField("httpStatusCode", code)

		if code >= 500 {
			logger.Error("Internal server error on request")
		} else {
			logger.Debug("Error on request")
		}
	}
}

func flushOnWrite(w io.Writer) io.Writer {
	if fw, ok := w.(writeFlusher); ok {
		return &flushWriter{fw}
	}
	return w
}

type flushWriter struct {
	w writeFlusher
}

type writeFlusher interface {
	Flush()
	Write([]byte) (int, error)
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 {
		fw.w.Flush()
	}
	return n, err
}

func httpStatusCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errdefs.IsNotFound(err):
		return http.StatusNotFound
	case errdefs.IsInvalidInput(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
