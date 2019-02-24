package azure

import (
	"net/http"

	"github.com/cpuguy83/strongerrors"
	"github.com/iofog/virtual-kubelet/providers/azure/client/api"
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
		return err
	}
}
