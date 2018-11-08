package azure

import (
	"net/http"
	"strconv"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	e, ok := err.(*api.Error)
	if !ok {
		return err
	}

	switch e.StatusCode {
	case http.StatusNotFound:
		return strongerrors.NotFound(err)
	default:
		if retryAfter, ok := getRetryAfter(e.Header); ok {
			return providers.NewRetryableError(err, retryAfter)
		}

		return err
	}
}

func getRetryAfter(header http.Header) (time.Duration, bool) {
	if header == nil {
		return time.Duration(0), false
	}

	retryAfterStr := header.Get("Retry-After")
	if retryAfterStr == "" {
		return time.Duration(0), false
	}

	retryAfter, err := strconv.Atoi(retryAfterStr)
	if err != nil {
		return time.Duration(0), false
	}

	return time.Duration(retryAfter)*time.Second, true
}