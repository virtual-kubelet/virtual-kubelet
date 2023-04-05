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
	"bytes"
	"context"
	"net/http"

	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	PrometheusTextFormatContentType = "text/plain; version=0.0.4"
)

// PodMetricsResourceHandlerFunc defines the handler for getting pod metrics
type PodMetricsResourceHandlerFunc func(context.Context) ([]*dto.MetricFamily, error)

// HandlePodMetricsResource makes an HTTP handler for implementing the kubelet /metrics/resource endpoint
func HandlePodMetricsResource(h PodMetricsResourceHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		metrics, err := h(req.Context())
		if err != nil {
			if isCancelled(err) {
				return err
			}
			return errors.Wrap(err, "error getting status from provider")
		}

		// Convert metrics to Prometheus text format.
		var buffer bytes.Buffer
		enc := expfmt.NewEncoder(&buffer, expfmt.FmtText)
		for _, mf := range metrics {
			if err := enc.Encode(mf); err != nil {
				return errors.Wrap(err, "could not convert metrics to prometheus text format")
			}
		}

		// Set the response content type to "text/plain; version=0.0.4".
		w.Header().Set("Content-Type", PrometheusTextFormatContentType)

		// Write the metrics in Prometheus text format to the response writer.
		if _, err := w.Write(buffer.Bytes()); err != nil {
			return errors.Wrap(err, "could not write to client")
		}

		return nil
	})
}
