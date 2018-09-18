package api

import (
	"io"
	"net/http"

	"github.com/cpuguy83/strongerrors/status"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

type handlerFunc func(http.ResponseWriter, *http.Request) error

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code, _ := status.HTTPCode(err)
		w.WriteHeader(code)
		io.WriteString(w, err.Error())
		logger := log.G(req.Context()).WithError(err).WithField("httpStatusCode", code)

		if code >= 500 {
			logger.Error("Internal server error on request")
		} else {
			log.Trace(logger, "Error on request")
		}
	}
}
