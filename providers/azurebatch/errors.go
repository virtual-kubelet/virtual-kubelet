package azurebatch

import (
	"net/http"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/cpuguy83/strongerrors"
)

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case isStatus(err, http.StatusNotFound):
		return strongerrors.NotFound(err)
	default:
		return err
	}
}

type causal interface {
	Cause() error
}

func isStatus(err error, status int) bool {
	if err == nil {
		return false
	}

	switch e := err.(type) {
	case *azure.RequestError:
		if e.StatusCode != 0 {
			return e.StatusCode == status
		}
		return isStatus(e.Original, status)
	case autorest.DetailedError:
		if e.StatusCode != 0 {
			return e.StatusCode == status
		}
		return isStatus(e.Original, status)
	case causal:
		return isStatus(e.Cause(), status)
	}

	return false
}
