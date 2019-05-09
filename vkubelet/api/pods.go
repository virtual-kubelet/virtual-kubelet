package api

import (
	"context"
	"net/http"

	"github.com/cpuguy83/strongerrors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// ContainerLogsBackend is used in place of backend implementations for the provider's pods
type RunningPodsBackend interface {
	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods(context.Context) ([]*v1.Pod, error)
}

func RunningPodsHandlerFunc(p RunningPodsBackend) http.HandlerFunc {
	scheme := runtime.NewScheme()
	v1.SchemeBuilder.AddToScheme(scheme)
	codecs := serializer.NewCodecFactory(scheme)

	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		ctx := req.Context()
		ctx = log.WithLogger(ctx, log.L)
		pods, err := p.GetPods(ctx)
		if err != nil {
			return strongerrors.System(err)
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
			return strongerrors.System(err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		if err != nil {
			return strongerrors.System(err)
		}
		return nil
	})
}
