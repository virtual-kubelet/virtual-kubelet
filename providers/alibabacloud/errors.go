package alibabacloud

import (
	"net/http"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
)

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	se, ok := err.(*errors.ServerError)
	if !ok {
		return err
	}

	switch se.HttpStatus() {
	case http.StatusNotFound:
		return errdefs.AsNotFound(err)
	default:
		return err
	}
}
