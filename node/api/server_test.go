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
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func TestAttachPodMetricsRoutes(t *testing.T) {
	var summaryCalled, metricsCalled bool
	cfg := api.PodMetricsConfig{
		GetStatsSummary: func(context.Context) (*statsv1alpha1.Summary, error) {
			summaryCalled = true
			return &statsv1alpha1.Summary{}, nil
		},
		GetMetricsResource: func(context.Context) ([]*dto.MetricFamily, error) {
			metricsCalled = true
			return nil, nil
		},
	}

	mux := http.NewServeMux()
	// Used to panic because both handlers were registered on "/".
	api.AttachPodMetricsRoutes(cfg, mux)

	for _, path := range []string{"/stats/summary", api.MetricsResourceRouteSuffix} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, rr.Code, path)
	}
	assert.True(t, summaryCalled, "stats summary handler was not called")
	assert.True(t, metricsCalled, "metrics resource handler was not called")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
