package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// PodStatsSummaryHandlerFunc defines the handler for getting pod stats summaries
type PodStatsSummaryHandlerFunc func(context.Context) (*stats.Summary, error)

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
