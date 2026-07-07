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

package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"google.golang.org/protobuf/proto"
)

const (
	prometheusContentType = "text/plain; version=0.0.4"
)

func TestHandlePodMetricsResource(t *testing.T) {
	testCases := []struct {
		name               string
		handler            api.PodMetricsResourceHandlerFunc
		expectedStatusCode int
		expectedError      error
	}{
		{
			name: "Valid PodMetricsResourceHandlerFunc",
			handler: func(_ context.Context) ([]*dto.MetricFamily, error) {
				// Create the expected metrics.
				cpuUsageMetric := &dto.MetricFamily{
					Name:   proto.String("container_cpu_usage_seconds_total"),
					Help:   proto.String("[ALPHA] Cumulative cpu time consumed by the container in core-seconds"),
					Type:   dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{},
				}
				memoryUsageMetric := &dto.MetricFamily{
					Name:   proto.String("container_memory_working_set_bytes"),
					Help:   proto.String("[ALPHA] Current working set of the container in bytes"),
					Type:   dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{},
				}

				// Add the sample metrics to the metric families.
				cpuUsageMetric.Metric = append(cpuUsageMetric.Metric, createSampleMetric(
					map[string]string{
						"container": "simple-hello-world-container",
						"namespace": "k8se-apps",
						"pod":       "test-pod--zruwatj-86454fdc54-2wwpw",
					},
					0.1104636, 1680536423102,
				))
				cpuUsageMetric.Metric = append(cpuUsageMetric.Metric, createSampleMetric(
					map[string]string{
						"container": "simple-hello-world-container",
						"namespace": "k8se-apps",
						"pod":       "test-pod--zruwatj-86454fdc54-4mzd4",
					},
					0.11322, 1680536423103,
				))
				memoryUsageMetric.Metric = append(memoryUsageMetric.Metric, createSampleMetric(
					map[string]string{
						"container": "simple-hello-world-container",
						"namespace": "k8se-apps",
						"pod":       "test-pod--zruwatj-86454fdc54-2wwpw",
					},
					2.3277568e+07, 1680536423102,
				))
				memoryUsageMetric.Metric = append(memoryUsageMetric.Metric, createSampleMetric(
					map[string]string{
						"container": "simple-hello-world-container",
						"namespace": "k8se-apps",
						"pod":       "test-pod--zruwatj-86454fdc54-4mzd4",
					},
					2.2450176e+07, 1680536423104,
				))

				return []*dto.MetricFamily{cpuUsageMetric, memoryUsageMetric}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Nil PodMetricsResourceHandlerFunc",
			handler:            nil,
			expectedStatusCode: http.StatusNotImplemented,
		},
		{
			name: "Error in PodMetricsResourceHandlerFunc",
			handler: func(_ context.Context) ([]*dto.MetricFamily, error) {
				return nil, errors.New("test error")
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedError:      errors.New("error getting status from provider: test error"),
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := api.HandlePodMetricsResource(tc.handler)
			require.NotNil(t, h)

			rr := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/metrics/resource", nil)
			require.NoError(t, err)

			h.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code)

			if tc.expectedError != nil {
				bodyBytes, err := io.ReadAll(rr.Body)
				require.NoError(t, err)
				assert.Contains(t, string(bodyBytes), tc.expectedError.Error())
			} else if tc.expectedStatusCode == http.StatusOK {
				contentType := rr.Header().Get("Content-Type")
				assert.Equal(t, prometheusContentType, contentType)
			}
		})
	}
}

func createSampleMetric(labels map[string]string, value float64, timestamp int64) *dto.Metric {
	labelPairs := []*dto.LabelPair{}
	for k, v := range labels {
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  proto.String(k),
			Value: proto.String(v),
		})
	}

	return &dto.Metric{Label: labelPairs, Gauge: &dto.Gauge{Value: &value}, TimestampMs: &timestamp}
}
