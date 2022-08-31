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
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type PodListerFunc func(context.Context) ([]*v1.Pod, error) //nolint:golint

func HandleRunningPods(getPods PodListerFunc) http.HandlerFunc { //nolint:golint
	if getPods == nil {
		return NotImplemented
	}

	scheme := runtime.NewScheme()
	/* #nosec */
	v1.SchemeBuilder.AddToScheme(scheme) //nolint:errcheck
	codecs := serializer.NewCodecFactory(scheme)

	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		ctx := req.Context()
		ctx = log.WithLogger(ctx, log.L)
		pods, err := getPods(ctx)
		if err != nil {
			return err
		}

		// Borrowed from github.com/kubernetes/kubernetes/pkg/kubelet/server/server.go
		// encodePods creates an v1.PodList object from pods and returns the encoded
		// PodList.
		podList := new(v1.PodList)
		for _, pod := range pods {
			podList.Items = append(podList.Items, *pod)
		}
		codec := codecs.LegacyCodec(v1.SchemeGroupVersion)
		data, err := runtime.Encode(codec, podList)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		if err != nil {
			return err
		}
		return nil
	})
}
