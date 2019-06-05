package api

import (
	"context"
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type PodListerFunc func(context.Context) ([]*v1.Pod, error)

func HandleRunningPods(getPods PodListerFunc) http.HandlerFunc {
	scheme := runtime.NewScheme()
	v1.SchemeBuilder.AddToScheme(scheme)
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
