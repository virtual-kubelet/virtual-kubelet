package azure

import (
	"net/http"

	"github.com/virtual-kubelet/azure-aci/client/api"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
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
		return errdefs.AsNotFound(err)
	default:
		return err
	}
}
