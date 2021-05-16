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
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
)

// PodStatsSummaryHandlerFunc defines the handler for getting pod stats summaries
type PodStatsSummaryHandlerFunc func(context.Context) (*statsv1alpha1.Summary, error)

// HandlePodStatsSummary makes an HTTP handler for implementing the kubelet summary stats endpoint
func HandlePodStatsSummary(h PodStatsSummaryHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		stats, err := h(req.Context())
		if err != nil {
			if isCancelled(err) {
				return err
			}
			return errors.Wrap(err, "error getting status from provider")
		}

		b, err := json.Marshal(stats)
		if err != nil {
			return errors.Wrap(err, "error marshalling stats")
		}

		if _, err := w.Write(b); err != nil {
			return errors.Wrap(err, "could not write to client")
		}
		return nil
	})
}

func isCancelled(err error) bool {
	if err == context.Canceled {
		return true
	}

	if e, ok := err.(causal); ok {
		return isCancelled(e.Cause())
	}
	return false
}

type causal interface {
	Cause() error
	error
}
